package engine

import (
	"regexp"
	"strings"
)

// Select returns resolved facts for the user-provided queries.
func Select(facts []ResolvedFact, queries []string) []ResolvedFact {
	return SelectWithDottedFacts(facts, queries, false)
}

// SelectWithDottedFacts returns resolved facts and optionally merges dotted custom
// and external facts into structured facts for partial queries.
func SelectWithDottedFacts(facts []ResolvedFact, queries []string, includeTypedDotted bool) []ResolvedFact {
	return NewProjection(facts, includeTypedDotted).Select(queries)
}

func factMatchesQuery(factName, query string) bool {
	if strings.Contains(factName, ".*") && !strings.Contains(query, ".") {
		pattern := strings.ReplaceAll(regexp.QuoteMeta(factName), `\.\*`, `.*`)
		matched, err := regexp.MatchString("^"+pattern+"$", query)
		return err == nil && matched
	}
	return query == factName || strings.HasPrefix(query, factName+".")
}
