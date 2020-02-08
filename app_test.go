package main

import (
	"testing"
)

func TestBean(t *testing.T) {
	var v Vote
	if v == (Vote{}) {
		t.Errorf("D")
	}
}
