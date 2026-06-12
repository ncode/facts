package engine

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDirectedGraphMatchesRubyCoreBehavior(t *testing.T) {
	t.Run("empty and self edges are acyclic", func(t *testing.T) {
		if !(directedGraph{}).acyclic() {
			t.Fatal("empty graph is cyclic")
		}

		graph := directedGraph{"one": []string{"one"}}
		if !graph.acyclic() {
			t.Fatal("self edge is cyclic")
		}
	})

	t.Run("topological sort returns dependencies first", func(t *testing.T) {
		graph := directedGraph{
			"one":   []string{"two", "three"},
			"two":   []string{"three"},
			"three": []string{},
		}

		got, err := graph.topologicalSort()
		if err != nil {
			t.Fatalf("topologicalSort(): %v", err)
		}
		want := []string{"three", "two", "one"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("topologicalSort() = %v, want %v", got, want)
		}
	})

	t.Run("cycle detection ignores self edges and reports loops", func(t *testing.T) {
		graph := directedGraph{
			"one":   []string{"two", "one"},
			"two":   []string{"one"},
			"three": []string{"four"},
			"four":  []string{"three"},
		}

		if graph.acyclic() {
			t.Fatal("graph is acyclic, want cyclic")
		}
		cycles := graph.cycles()
		if !containsSameStrings(cycles, []string{"one", "two"}) {
			t.Fatalf("cycles() = %v, want cycle containing one and two", cycles)
		}
	})

	t.Run("sort reports cycles and missing vertices", func(t *testing.T) {
		cyclic := directedGraph{"one": []string{"two"}, "two": []string{"one"}}
		_, err := cyclic.topologicalSort()
		if !errors.Is(err, errDirectedGraphCycle) || !strings.Contains(err.Error(), "found the following cycles:") {
			t.Fatalf("topologicalSort() error = %v, want cycle error", err)
		}

		missing := directedGraph{"one": []string{"two", "three"}, "two": []string{"three"}}
		_, err = missing.topologicalSort()
		if !errors.Is(err, errDirectedGraphMissingVertex) || !strings.Contains(err.Error(), "missing elements") || !strings.Contains(err.Error(), "three") {
			t.Fatalf("topologicalSort() error = %v, want missing vertex error", err)
		}
	})
}

func containsSameStrings(groups [][]string, want []string) bool {
	wantCounts := make(map[string]int, len(want))
	for _, value := range want {
		wantCounts[value]++
	}
	for _, group := range groups {
		if len(group) != len(want) {
			continue
		}
		gotCounts := make(map[string]int, len(group))
		for _, value := range group {
			gotCounts[value]++
		}
		if reflect.DeepEqual(gotCounts, wantCounts) {
			return true
		}
	}
	return false
}
