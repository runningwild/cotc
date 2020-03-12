package main

import (
	"cloud.google.com/go/datastore"
	"fmt"
	"github.com/runningwild/cotc/keeper"
	"github.com/runningwild/cotc/types"
	"log"
	"net/http"
	"os"
	"strconv"
)

func gicmpHandler(g *types.GICMPData, client *datastore.Client, tk *keeper.Keeper) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		user, err := getUser(client, req)
		if err != nil {
			fmt.Fprintf(w, "internal error")
			log.Printf("Error getting user from URL %q: %v", req.URL, err)
			return
		}
		logger := log.New(os.Stdout, fmt.Sprintf("user(%s)@", user.Key().Name), log.Lshortfile|log.Ltime|log.Ldate)
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

		// Now check if the URL included a response to a question
		doWrite := false
		{
			s := req.URL.Query().Get("s")
			q := req.URL.Query().Get("q")
			if s != "" && q != "" {
				snum, serr := strconv.Atoi(s)
				qnum, qerr := strconv.Atoi(q)
				if serr == nil && qerr == nil {
					if snum >= 0 && snum <= 2 && qnum >= 0 && qnum < len(g.Statements) {
						// valid
						if qnum <= len(gr.Ratings) {
							if qnum < len(gr.Ratings) {
								// No problem, they just hit back in their browser.
								gr.Ratings = gr.Ratings[0:qnum]
							}
							gr.Ratings = append(gr.Ratings, snum)
							doWrite = true
						} else {
							logger.Printf("somehow got an answer for the future?")
						}
					} else {
						logger.Printf("parsed s and q but got invalid values: %d %d", snum, qnum)
					}
				} else {
					logger.Printf("failed to parse one or more of s and q: %v %v", serr, qerr)
				}
			}
		}

		if len(gr.Ratings) >= len(g.Statements) && len(gr.Results) == 0 {
			for _, group := range g.Groups {
				score := 0
				count := 0
				for _, q := range group.Questions {
					score += gr.Ratings[q]
					count++
				}
				score = int(float64(score)/float64(count)*4.0/3.0 + 0.5)
				gr.Results = append(gr.Results, score)
			}
			doWrite = true
		}
		if doWrite {
			if _, err := client.Put(req.Context(), grKey, &gr); err != nil {
				logger.Printf("failed to update GICMPResponse: %v", err)
			}
		}

		if len(gr.Results) > 0 {
			var names []string
			for _, group := range g.Groups {
				names = append(names, group.Name)
			}
			texts := map[string]string{}
			env := map[string]interface{}{
				"Names":   names,
				"Texts":   texts,
				"UserKey": user.Key().Name,
			}
			for i, group := range g.Groups {
				if gr.Results[i] < 0 || gr.Results[i] >= len(group.Blurbs) {
					logger.Printf("got a GICMP result out of range: %d vs %d", gr.Results[i], len(group.Blurbs))
					if gr.Results[i] < 0 {
						gr.Results[i] = 0
					}
					if gr.Results[i] >= len(group.Blurbs) {
						gr.Results[i] = len(group.Blurbs)
					}
				}
				texts[group.Name] = group.Blurbs[gr.Results[i]]
			}
			// t, err := tk.Get("gicmp_results.tmpl")
			t, err := tk.Get("vote_complete.tmpl")
			if err != nil {
				logger.Printf("failed to get template: %v", err)
				return
			}
			if err := t.Execute(w, env); err != nil {
				logger.Printf("err: %v", err)
			}
			return
		}
		q := len(gr.Ratings)
		env := map[string]interface{}{
			"Question":     q + 1,
			"NumQuestions": len(g.Statements),
			"Statement":    g.Statements[q],
			"Links": map[string]string{
				"Rarely":    fmt.Sprintf("/gicmp?user=%s&s=0&q=%d", user.Key().Name, q),
				"Sometimes": fmt.Sprintf("/gicmp?user=%s&s=1&q=%d", user.Key().Name, q),
				"Often":     fmt.Sprintf("/gicmp?user=%s&s=2&q=%d", user.Key().Name, q),
			},
		}
		t, err := tk.Get("gicmp.tmpl")
		if err != nil {
			logger.Printf("failed to get template: %v", err)
			return
		}
		if err := t.Execute(w, env); err != nil {
			logger.Printf("%v", err)
		}
	}
}
