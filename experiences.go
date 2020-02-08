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

func experiencesHandler(client *datastore.Client, tk *keeper.Keeper) http.HandlerFunc {
	data, err := ioutil.ReadFile("static/experiences.json")
	if err != nil {
		panic(err)
	}
	var exp types.Experiences
	if err := json.Unmarshal(data, &exp); err != nil {
		panic(err)
	}
	expIDs := make(map[string]bool)
	for _, exp := range exp.Statements {
		expIDs[exp.ID] = true
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

		responseKey := datastore.NameKey("ExperiencesResponse", "highlander", user.Key())
		if req.Method == "POST" {
			if err := req.ParseForm(); err != nil {
				fmt.Fprintf(w, "internal error")
				logger.Printf("failed to parse form: %v", err)
				return
			}
			var er types.ExperiencesResponse
			for experience, response := range req.Form {
				if !expIDs[experience] {
					continue
				}
				logger.Printf("%q -> %q", experience, response)
				if len(response) != 1 {
					logger.Printf("Got a confusing value on experience %q: %q", experience, response)
					continue
				}
				er.Results = append(er.Results, types.ExperienceInfo{experience, response[0]})
			}
			if _, err := client.Put(req.Context(), responseKey, &er); err != nil {
				logger.Printf("failed to add skills for %q: %v", user.Email, err)
			}
			http.Redirect(w, req, fmt.Sprintf("/core?user=%s", user.Key().Name), http.StatusFound)
			return
		}

		var er types.ExperiencesResponse
		if err := client.Get(req.Context(), responseKey, &er); err != nil {
		}

		t, err := tk.Get("experiences.tmpl")
		if err != nil {
			logger.Printf("failed to get skills template: %v", err)
			return
		}

		responseset := make(map[string]string)
		for _, info := range er.Results {
			responseset[info.ExperienceID] = info.Response
		}
		if err := t.Execute(w, map[string]interface{}{
			"UserKey":     user.Key().Name,
			"Experiences": exp,
			"Results":     responseset,
		}); err != nil {
			logger.Printf("failed to execute skills template: %v", err)
		}
	}
}
