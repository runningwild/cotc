package main

import (
	"fmt"
	"github.com/runningwild/cotc/keeper"
	"os"
	"time"
)

func main() {
	k, err := keeper.New("../../static")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v\n", k)

	for range time.Tick(time.Second){
		fmt.Printf("Ticking...\n")
		t, err := k.Get("gicmp.tmpl")
		if err != nil {
			fmt.Printf("failed to get: %v\n", err)
			continue
		}
		if err := t.Execute(os.Stdout, map[string]string{"Statement": "This is the statement"}); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		fmt.Printf("\n")
	}
}

