package main

import (
	"fmt"
	"github.com/runningwild/cotc/keeper"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"

	"github.com/runningwild/cotc/types"
)

func coreHandler(client *datastore.Client, surveys []string, tk *keeper.Keeper) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		user, err := getUser(client, req)
		if err != nil {
			fmt.Fprintf(w, "internal error")
			log.Printf("Error getting user from URL %q: %v", req.URL, err)
			return
		}
		logger := log.New(os.Stdout, fmt.Sprintf("user(%s)@", user.Key().Name), log.Lshortfile|log.Ltime|log.Ldate)
		allDone := true
		q := datastore.NewQuery("SurveyResponse").Ancestor(user.Key())
		it := client.Run(req.Context(), q)
		srs := make(map[string]types.SurveyResponse)
		for _, survey := range surveys {
			srs[survey] = types.SurveyResponse{}
		}
		for {
			var sr types.SurveyResponse
			key, err := it.Next(&sr)
			if err != nil {
				if err == iterator.Done {
					break
				}
				fmt.Fprintf(w, "internal error")
				logger.Printf("failed to read all responses for %v: %v", user.Email, err)
				allDone = false
				return
			}
			srs[key.Name] = sr
			if len(sr.Results) == 0 {
				allDone = false
			}
		}

		var gr types.GICMPResponse
		grKey := datastore.NameKey("GICMPResponse", "highlander", user.Key())
		if err := client.Get(req.Context(), grKey, &gr); err != nil {
			if err != datastore.ErrNoSuchEntity {
				logger.Printf("failed to get GICMPResponse, but not because it didn't exist: %v", err)
				return
			}
			if _, err := client.Put(req.Context(), grKey, &gr); err != nil {
				logger.Printf("failed to insert new GICMPResponse: %v", err)
				return
			}
		}
		if len(gr.Results) == 0 {
			allDone = false
		}

		var skills types.SkillsResponse
		skillsKey := datastore.NameKey("SkillsResponse", "highlander", user.Key())
		if err := client.Get(req.Context(), skillsKey, &skills); err != nil && err != datastore.ErrNoSuchEntity {
			logger.Printf("failed to get SkillsResponse, but not because it didn't exist: %v", err)
			return
		}
		if len(skills.Results) == 0 {
			allDone = false
		}

		var experiences types.ExperiencesResponse
		experiencesKey := datastore.NameKey("ExperiencesResponse", "highlander", user.Key())
		if err := client.Get(req.Context(), experiencesKey, &experiences); err != nil && err != datastore.ErrNoSuchEntity {
			logger.Printf("failed to get ExperiencesResponse, but not because it didn't exist: %v", err)
			return
		}
		if len(experiences.Results) == 0 {
			allDone = false
		}

		t, err := tk.Get("core.tmpl")
		if err != nil {
			logger.Printf("failed to get core template: %v", err)
			return
		}
		if err := t.Execute(w, map[string]interface{}{
			"Responses":   srs,
			"GICMP":       gr,
			"User":        user,
			"UserKey":     user.Key().Name,
			"Skills":      skills,
			"Experiences": experiences,
			"AllDone":     allDone,
		}); err != nil {
			logger.Printf("%v", err)
		}
	}
}
