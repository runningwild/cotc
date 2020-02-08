package main

import (
	"fmt"
	"github.com/runningwild/cotc/keeper"
	"log"
	"math/rand"
	"net/http"
	"os"
	"text/template"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/runningwild/cotc/types"
	"github.com/runningwild/cotc/vote"
)

func voteHandler(v *vote.Vote, client *datastore.Client, surveyName string, tk *keeper.Keeper) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		user, err := getUser(client, req)
		if err != nil {
			fmt.Fprintf(w, "internal error")
			log.Printf("Error getting user from URL %q: %v", req.URL, err)
			return
		}
		logger := log.New(os.Stdout, fmt.Sprintf("user(%s)@", user.Key().Name), log.Lshortfile|log.Ltime|log.Ldate)

		var sr types.SurveyResponse
		doWrite := false
		responseKey := datastore.NameKey("SurveyResponse", surveyName, user.Key())
		if err := client.Get(req.Context(), responseKey, &sr); err != nil {
			doWrite = true
		}
		logger.Printf("Preexisting responses:%v", sr)

		// prev is the hash of all preferences for this survey response before adding the new pref
		// If prev doesn't match the hash of the current survey response prefs, then we will remove
		// up to one pref off the end of the list to get the hash to matche.  This just means that
		// the user hit back in their browser to undo a question.
		prev := req.URL.Query().Get("prev")
		prefs := sr.Preferences
		if len(prefs) > 0 && prev != easyHashObj(prefs) {
			logger.Printf("%q != %q <- %v", prev, easyHashObj(prefs), prefs)
			prefs = prefs[0 : len(prefs)-1]
		}
		logger.Printf("Down to %v", prefs)
		if prev == easyHashObj(prefs) {
			sr.Preferences = prefs
			penc := req.URL.Query().Get("pref")
			p, err := decodePreference(penc)
			if err != nil {
				logger.Printf("failed to decode preference: %v", err)
			} else {
				sr.Preferences = append(sr.Preferences, *p)
				doWrite = true
			}
		} else {
			logger.Printf("hashes don't match, so we'll just ignore that one")
		}

		statements := v.NextQuestion(sr.Preferences, rand.New(rand.NewSource(time.Now().UnixNano())))
		logger.Printf("Statements: %v", statements)
		if statements == nil {
			scores := v.Score(sr.Preferences)
			for _, score := range scores {
				logger.Printf("Score: %v", score)
			}
			sr.Results = v.Top(sr.Preferences, 1)

			var winners []vote.Candidate
			for _, c := range sr.Results {
				winners = append(winners, v.Candidates[c])
			}
			var t *template.Template
			var err error
			if v.ImmediateResults {
				t, err = tk.Get("vote_results.tmpl")
			} else {
				t, err = tk.Get("vote_complete.tmpl")
			}
			if err != nil {
				logger.Printf("failed to get survey complete template: %v", err)
				return
			}
			if err := t.Execute(w, map[string]interface{}{
				"UserKey": user.Key().Name,
				"Winners": winners,
			}); err != nil {
				fmt.Fprintf(w, "error: %v", err)
			}
			doWrite = true
		}

		if doWrite {
			if _, err := client.Put(req.Context(), responseKey, &sr); err != nil {
				fmt.Fprintf(w, "failed when putting survey response for %q: %v", user.Email, err)
			}
		}

		if statements == nil {
			return
		}

		logger.Printf("Put the following responses: %v", sr)

		t, err := tk.Get("vote.tmpl")
		if err != nil {
			logger.Printf("failed to get survey template: %v", err)
			return
		}

		var options []map[string]string
		var ps []types.Preference
		for _, i := range statements {
			p := types.Preference{A: i}
			for _, j := range statements {
				if j == i {
					continue
				}
				p.B = append(p.B, j)
			}
			ps = append(ps, p)
			options = append(options, map[string]string{
				"Text": v.Statements[i].Text,
				"Link": encodePreference(p),
			})
		}
		if err := t.Execute(w, map[string]interface{}{
			"Question":  len(sr.Preferences) + 1,
			"MaxRounds": v.MaxRounds,
			"UserKey":   user.Key().Name,
			"Path":      req.URL.RawPath,
			"Prev":      easyHashObj(sr.Preferences),
			"Options":   options,
		}); err != nil {
			logger.Printf("failed to execute survey template: %v", err)
		}
	}
}
