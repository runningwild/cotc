package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"cloud.google.com/go/datastore"

	"github.com/runningwild/cotc/types"
	"github.com/runningwild/cotc/vote"
)

func main() {
	flag.Parse()
	ctx := context.Background()
	if err := doit(ctx); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func doit(ctx context.Context) error {
	client, err := datastore.NewClient(ctx, "churchofthecity")
	if err != nil {
		panic(fmt.Sprintf("WHAT: %v", err))
	}
	q := datastore.NewQuery("user")
	var u types.User
	it := client.Run(ctx, q)
	for key, err := it.Next(&u); err == nil; key, err = it.Next(&u) {
		fmt.Printf("user: %v key: %v\n", u.Email, key)
		{
			var gicmp types.GICMPResponse
			if err := client.Get(ctx, datastore.NameKey("GICMPResponse", "highlander", key), &gicmp); err == nil {
				fmt.Printf("GICMP: %v\n", gicmp.Results)
			}
		}
		q2 := datastore.NewQuery("SurveyResponse").Ancestor(key)
		it2 := client.Run(ctx, q2)
		var _sr types.SurveyResponse
		for key, err := it2.Next(&_sr); err == nil; key, err = it2.Next(&_sr) {
			sr := _sr
			_sr = types.SurveyResponse{}
			data, err := ioutil.ReadFile(filepath.Join("../static", key.Name+".json"))
			if err != nil {
				panic(err)
			}
			var v vote.Vote
			if err := json.Unmarshal(data, &v); err != nil {
				panic(err)
			}
			fmt.Printf("  %s\n", key.Name)
			for _, r := range sr.Results {
				fmt.Printf("    %s\n", v.Candidates[r].Name)
			}
		}
	}
	return err
}
