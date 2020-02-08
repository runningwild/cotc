package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"cloud.google.com/go/datastore"

	"github.com/runningwild/cotc/types"
)

func main() {
	flag.Parse()
	ctx := context.Background()

	for _, person := range list {
		parts := strings.Split(person, " ")
		email := strings.ToLower(parts[len(parts)-1])
		first := parts[0]
		last := strings.Join(parts[1:len(parts)-1], " ")
		if err := doit(ctx, email, first, last); err != nil {
			fmt.Printf("%v\n", err)
		}
	}
}

func doit(ctx context.Context, email, first, last string) error {
	client, err := datastore.NewClient(ctx, "churchofthecity")
	if err != nil {
		panic(fmt.Sprintf("WHAT: %v", err))
	}
	u := &types.User{
		Email:     email,
		FirstName: first,
		LastName:  last,
	}
	m := datastore.NewInsert(u.Key(), u)
	if keys, err := client.Mutate(ctx, m); err != nil {
		return fmt.Errorf("failed to insert new user: %w", err)
	} else {
		for _, key := range keys {
			fmt.Printf("%q churchofthecity.appspot.com/core?user=%s\n", email, key.Name)
		}
	}
	return nil
}

var list = []string{
	"Lauren Cook lecook10@comcast.net",
	"Ben Goff benjaminygoff@gmail.com",
	"Dan Andrews dandrews412@gmail.com",
}
