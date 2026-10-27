package app

import (
	"maps"
	"testing"
)

func TestRefreshedPartialKeepsOld(t *testing.T) {
	old := map[int]string{0: "A", 1: "B", 2: "C"}
	got := map[int]string{0: "A2"}
	next := refreshed(old, got)
	want := map[int]string{0: "A2", 1: "B", 2: "C"}
	if !maps.Equal(next, want) {
		t.Fatalf("got %v, want %v", next, want)
	}
	if refreshed(old, map[int]string{}) != nil {
		t.Fatal("empty refresh must keep old names")
	}
	if refreshed(old, maps.Clone(old)) != nil {
		t.Fatal("unchanged refresh must report no change")
	}
	grown := map[int]string{0: "A", 1: "B", 2: "C", 3: "D"}
	if !maps.Equal(refreshed(old, grown), grown) {
		t.Fatal("full refresh with more names must replace")
	}
}
