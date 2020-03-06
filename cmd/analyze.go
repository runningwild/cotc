package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"cloud.google.com/go/datastore"

	"github.com/runningwild/cotc/types"
	"github.com/runningwild/cotc/vote"
)

var (
	userKeyName = flag.String("user", "", "user")
)

func main() {
	flag.Parse()
	ctx := context.Background()
	if err := analyze(ctx, *userKeyName); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func analyze(ctx context.Context, userKey string) error {
	client, err := datastore.NewClient(ctx, "montage-generator")
	if err != nil {
		panic(fmt.Sprintf("WHAT: %v", err))
	}

	var user types.User
	if err := client.Get(ctx, datastore.NameKey("user", userKey, nil), &user); err != nil {
		return fmt.Errorf("couldn't find user: %w", err)
	}

	var gicmp types.GICMPData
	data, err := ioutil.ReadFile("../static/gicmp.json")
	if err != nil {
		return fmt.Errorf("failed to load gicmp data: %w")
	}
	if err := json.Unmarshal(data, &gicmp); err != nil {
		return fmt.Errorf("failed to decode gicmp data: %w", err)
	}

	surveys := make(map[string]*vote.Vote)
	for _, name := range []string{"gifts", "enneagram", "strengths"} {
		data, err := ioutil.ReadFile("../static/" + name + ".json")
		if err != nil {
			return fmt.Errorf("failed to load %s data: %w", name, err)
		}
		var v vote.Vote
		if err := json.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("failed to decode %s data: %w", name, err)
		}
		surveys[name] = &v
	}

	var experiences types.Experiences
	{
		data, err := ioutil.ReadFile("../static/experiences.json")
		if err != nil {
			return fmt.Errorf("failed to load experiences data: %w", err)
		}
		if err := json.Unmarshal(data, &experiences); err != nil {
			return fmt.Errorf("failed to decode experiences data: %w", err)
		}
	}

	userData, err := getAllUserData(ctx, client, &user)
	if err != nil {
		return err
	}

	name := "enneagram"
	for i := range surveys[name].Candidates {
		fmt.Printf("Candidate %d\n", i)
		for _, pref := range userData.surveys[name].Preferences {
			win, lose := false, false
			if includes(surveys[name].Statements[pref.A].Candidates, i) {
				win = true
			}
			for _, b := range pref.B {
				if includes(surveys[name].Statements[b].Candidates, i) {
					lose = true
				}
			}
			if !win && !lose {
				continue
			}
			if win && lose {
				fmt.Printf("  ????\n")
			}
			if win {
				fmt.Printf("  prefered in %v vs %v\n", pref.A, pref.B)
			}
			if lose {
				fmt.Printf("  rejected in %v vs %v\n", pref.A, pref.B)
			}
		}
		fmt.Printf("\n")
	}

	return nil
}

func includes(vs []int, n int) bool {
	for _, v := range vs {
		if v == n {
			return true
		}
	}
	return false
}

func getAllUserData(ctx context.Context, client *datastore.Client, user *types.User) (*allUserData, error) {
	userData := allUserData{
		User: user,
	}

	if err := client.Get(ctx, datastore.NameKey("GICMPResponse", "highlander", user.Key()), &userData.gicmp); err != nil {
		return nil, fmt.Errorf("failed to get gicmp data: %w", err)
	}

	if err := client.Get(ctx, datastore.NameKey("SkillsResponse", "highlander", user.Key()), &userData.skills); err != nil {
		return nil, fmt.Errorf("failed to get skills data: %w", err)
	}

	if err := client.Get(ctx, datastore.NameKey("ExperiencesResponse", "highlander", user.Key()), &userData.experiences); err != nil {
		return nil, fmt.Errorf("failed to get experiences data: %w", err)
	}

	userData.surveys = make(map[string]types.SurveyResponse)
	for _, name := range []string{"gifts", "enneagram", "strengths"} {
		var sr types.SurveyResponse
		if err := client.Get(ctx, datastore.NameKey("SurveyResponse", name, user.Key()), &sr); err != nil {
			return nil, fmt.Errorf("failed to get %s survey response: %w", name, err)
		}
		userData.surveys[name] = sr
	}

	return &userData, nil
}

type allUserData struct {
	*types.User
	gicmp       types.GICMPResponse
	surveys     map[string]types.SurveyResponse
	experiences types.ExperiencesResponse
	skills      types.SkillsResponse
}
