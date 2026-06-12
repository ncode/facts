package engine

import (
	"regexp"
	"slices"
	"strings"
)

// Select returns resolved facts for the user-provided queries.
func Select(facts []ResolvedFact, queries []string) []ResolvedFact {
	return SelectWithDottedFacts(facts, queries, false)
}

// SelectWithDottedFacts returns resolved facts and optionally merges dotted custom
// and external facts into structured facts for partial queries.
func SelectWithDottedFacts(facts []ResolvedFact, queries []string, includeTypedDotted bool) []ResolvedFact {
	if len(queries) == 0 {
		return facts
	}

	selected := make([]ResolvedFact, 0, len(queries))
	collection := CollectionWithDottedFacts(facts, includeTypedDotted)
	for _, query := range queries {
		selected = append(selected, findFact(facts, collection, query))
	}
	return selected
}

func findFact(facts []ResolvedFact, collection map[string]any, query string) ResolvedFact {
	query = strings.ToLower(query)
	for _, fact := range slices.Backward(facts) {

		if factMatchesQuery(fact.Name, query) {
			fact.UserQuery = query
			return fact
		}
	}
	if value := dig(collection, strings.Split(query, ".")); value != nil {
		return ResolvedFact{Name: query, UserQuery: query, Value: value}
	}
	return ResolvedFact{Name: query, UserQuery: query, Type: "nil"}
}

func factMatchesQuery(factName, query string) bool {
	if strings.Contains(factName, ".*") && !strings.Contains(query, ".") {
		matched, err := regexp.MatchString("^"+factName+"$", query)
		return err == nil && matched
	}
	return query == factName || strings.HasPrefix(query, factName+".")
}
