package engine

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var errDirectedGraphCycle = errors.New("directed graph: cycle")
var errDirectedGraphMissingVertex = errors.New("directed graph: missing vertex")

type directedGraph map[string][]string

func (g directedGraph) acyclic() bool {
	return len(g.cycles()) == 0
}

func (g directedGraph) cycles() [][]string {
	cycles := [][]string{}
	seenCycles := map[string]bool{}
	visited := map[string]bool{}
	onStack := map[string]int{}
	stack := []string{}

	var visit func(string)
	visit = func(vertex string) {
		visited[vertex] = true
		onStack[vertex] = len(stack)
		stack = append(stack, vertex)

		for _, next := range g[vertex] {
			if next == vertex {
				continue
			}
			if index, ok := onStack[next]; ok {
				cycle := append([]string(nil), stack[index:]...)
				key := cycleKey(cycle)
				if !seenCycles[key] {
					seenCycles[key] = true
					cycles = append(cycles, cycle)
				}
				continue
			}
			if !visited[next] {
				visit(next)
			}
		}

		stack = stack[:len(stack)-1]
		delete(onStack, vertex)
	}

	for _, vertex := range g.vertices() {
		if !visited[vertex] {
			visit(vertex)
		}
	}
	return cycles
}

func (g directedGraph) topologicalSort() ([]string, error) {
	if missing := g.missingVertices(); len(missing) > 0 {
		return nil, fmt.Errorf("%w: missing elements %s", errDirectedGraphMissingVertex, strings.Join(missing, ", "))
	}
	cycles := g.cycles()
	if len(cycles) > 0 {
		return nil, fmt.Errorf("%w: found the following cycles: %v", errDirectedGraphCycle, cycles)
	}

	visited := map[string]bool{}
	order := make([]string, 0, len(g))
	var visit func(string)
	visit = func(vertex string) {
		if visited[vertex] {
			return
		}
		visited[vertex] = true
		for _, next := range sortedStrings(g[vertex]) {
			if next != vertex {
				visit(next)
			}
		}
		order = append(order, vertex)
	}

	for _, vertex := range g.vertices() {
		visit(vertex)
	}
	return order, nil
}

func (g directedGraph) missingVertices() []string {
	missing := map[string]bool{}
	for _, edges := range g {
		for _, edge := range edges {
			if _, ok := g[edge]; !ok {
				missing[edge] = true
			}
		}
	}
	return sortedStringSet(missing)
}

func (g directedGraph) vertices() []string {
	keys := make([]string, 0, len(g))
	for key := range g {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cycleKey(cycle []string) string {
	return strings.Join(sortedStrings(cycle), "\x00")
}

func sortedStrings(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
}

func sortedStringSet(values map[string]bool) []string {
	sorted := make([]string, 0, len(values))
	for value := range values {
		sorted = append(sorted, value)
	}
	sort.Strings(sorted)
	return sorted
}
