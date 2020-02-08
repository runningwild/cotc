package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"

	"github.com/runningwild/cotc/keeper"
	"github.com/runningwild/cotc/types"
)

func skillsHandler(client *datastore.Client, tk *keeper.Keeper) http.HandlerFunc {
	data, err := ioutil.ReadFile("static/skills.json")
	if err != nil {
		panic(err)
	}
	var s skills
	if err := json.Unmarshal(data, &s); err != nil {
		panic(err)
	}
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		user, err := getUser(client, req)
		if err != nil {
			fmt.Fprintf(w, "internal error")
			log.Printf("Error getting user from URL %q: %v", req.URL, err)
			return
		}
		logger := log.New(os.Stdout, fmt.Sprintf("user(%s)@", user.Key().Name), log.Lshortfile|log.Ltime|log.Ldate)

		responseKey := datastore.NameKey("SkillsResponse", "highlander", user.Key())
		if req.Method == "POST" {
			if err := req.ParseForm(); err != nil {
				fmt.Fprintf(w, "internal error")
				logger.Printf("failed to parse form: %v", err)
				return
			}
			var sr types.SkillsResponse
			for skill, experience := range req.Form {
				if len(experience) != 1 {
					logger.Printf("Got a confusing value on skill %q: %q", skill, experience)
					continue
				}
				sr.Results = append(sr.Results, types.SkillInfo{skill, experience[0]})
			}
			if _, err := client.Put(req.Context(), responseKey, &sr); err != nil {
				logger.Printf("failed to add skills for %q: %v", user.Email, err)
			}
			http.Redirect(w, req, fmt.Sprintf("/core?user=%s", user.Key().Name), http.StatusFound)
			return
		}

		var sr types.SkillsResponse
		if err := client.Get(req.Context(), responseKey, &sr); err != nil {
		}

		t, err := tk.Get("skills.tmpl")
		if err != nil {
			logger.Printf("failed to get skills template: %v", err)
			return
		}

		skillset := make(map[string]string)
		for _, info := range sr.Results {
			skillset[info.Skill] = info.Experience
		}
		if err := t.Execute(w, map[string]interface{}{
			"UserKey": user.Key().Name,
			"Skills":  s,
			"Results": skillset,
		}); err != nil {
			logger.Printf("failed to execute skills template: %v", err)
		}
	}
}

type skills struct {
	Prompt  string
	Options []string
	Skills  []string
}
