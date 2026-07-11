package engine

import (
	"regexp"
	"strings"
)

// Projection owns query selection and output projection over one set of
// resolved facts. It centralizes the rules that Snapshot value lookup, the CLI
// strict-mode path, and the formatter adapters previously reassembled
// independently: reverse-precedence selection, wildcard matching, canonical
// tree fallback for nested queries, dotted external/registered fact merge mode,
// selected-query value extraction, and strict missing-query detection.
//
// A Projection memoizes the canonical tree it builds for fallback digs, so
// repeated lookups over one Projection do not rebuild the tree per call.
// Discovery Projections select queries with the discovery plan's dotted mode;
// a Snapshot retains a canonical non-force-dot Projection; OutputProjection
// creates a defensive CLI presentation Projection; and the version fast path
// builds its own synthetic one-fact Projection.
type Projection struct {
	facts              []ResolvedFact
	includeTypedDotted bool

	tree    map[string]any
	treeSet bool
}

// NewProjection returns a Projection over facts. includeTypedDotted selects the
// dotted external/registered fact merge mode used for the canonical tree and
// the nested-query fallback dig.
func NewProjection(facts []ResolvedFact, includeTypedDotted bool) *Projection {
	return &Projection{facts: facts, includeTypedDotted: includeTypedDotted}
}

// newProjectionWithTree returns a Projection that reuses an already-built
// canonical tree instead of building one on first use. The supplied tree MUST
// match CollectionWithDottedFacts(facts, includeTypedDotted); callers that hold
// a precomputed tree (Snapshot) use this to avoid rebuilding it per lookup.
func newProjectionWithTree(facts []ResolvedFact, includeTypedDotted bool, tree map[string]any) *Projection {
	return &Projection{facts: facts, includeTypedDotted: includeTypedDotted, tree: tree, treeSet: true}
}

// Collection returns the canonical fact tree, built once and memoized. The
// returned map is the live memoized tree, shared with FullTree and (for a
// Snapshot-bound projection) with the Snapshot's own tree; callers MUST treat
// it as read-only. Mutating it corrupts the projection and any tree it was
// constructed from.
func (p *Projection) Collection() map[string]any {
	if !p.treeSet {
		p.tree = CollectionWithDottedFacts(p.facts, p.includeTypedDotted)
		p.treeSet = true
	}
	return p.tree
}

// selectFacts returns one resolved fact per query, applying reverse-precedence
// selection, wildcard matching, and canonical tree fallback. With no queries it
// returns the backing facts unchanged, matching the full-output contract.
func (p *Projection) selectFacts(queries []string) []ResolvedFact {
	if len(queries) == 0 {
		return p.facts
	}
	collection := p.Collection()
	selected := make([]ResolvedFact, 0, len(queries))
	for _, query := range queries {
		selected = append(selected, findFactIn(p.facts, collection, query))
	}
	return selected
}

// LookupValue resolves a single query to its canonical-tree value. found
// reports whether any fact resolved the query: a fact that legitimately
// resolved to a nil value is found (value nil, found true), while a query no
// fact resolved is missing (value nil, found false). This is the Snapshot
// missing-vs-nil contract.
func (p *Projection) LookupValue(query string) (value any, found bool) {
	fact := p.selectFacts([]string{query})[0]
	if v, found := valueForQuery(fact); found {
		return v, true
	}
	if (fact.Type == "custom" || fact.Type == "external") && fact.Value == nil && fact.UserQuery == fact.Name {
		return nil, true
	}
	return nil, false
}

// MissingQueries returns the user queries among selected facts that no fact
// resolved to a non-nil value, in order, for CLI strict-mode reporting. A
// selected fact with an empty UserQuery (full output) is never missing.
func (p *Projection) MissingQueries() []string {
	missing := make([]string, 0)
	for _, fact := range p.facts {
		if fact.UserQuery != "" && ValueForQuery(fact) == nil {
			missing = append(missing, fact.UserQuery)
		}
	}
	return missing
}

// PresentationNames returns one display name per backing fact in order. Query
// output uses the original user query; full output falls back to the fact name.
func (p *Projection) PresentationNames() []string {
	names := make([]string, 0, len(p.facts))
	for _, fact := range p.facts {
		name := fact.UserQuery
		if name == "" {
			name = fact.Name
		}
		names = append(names, name)
	}
	return names
}

// OutputShape names the projection shape a formatter renders.
type OutputShape int

const (
	// ShapeFullTree is the no-query, full canonical tree output.
	ShapeFullTree OutputShape = iota
	// ShapeSingleQuery is one selected query rendered as a bare value.
	ShapeSingleQuery
	// ShapeMultiQuery is multiple selected queries rendered as a query map.
	ShapeMultiQuery
	// ShapeEmpty is no output (no facts and no queries).
	ShapeEmpty
)

// Shape reports which output shape the backing facts project to, following the
// formatter contract: no user queries with facts present is the full tree, a
// single user query is a bare value, multiple user queries are a query map, and
// no facts at all is empty.
func (p *Projection) Shape() OutputShape {
	queries := uniqueQueries(p.facts)
	switch {
	case len(queries) == 0:
		return ShapeEmpty
	case len(queries) == 1 && queries[0] == "":
		return ShapeFullTree
	case len(queries) == 1:
		return ShapeSingleQuery
	default:
		return ShapeMultiQuery
	}
}

// FullTree returns the canonical tree for ShapeFullTree rendering.
func (p *Projection) FullTree() map[string]any {
	return p.Collection()
}

// SingleQueryValue returns the lone selected query's value for ShapeSingleQuery
// rendering.
func (p *Projection) SingleQueryValue() any {
	return ValueForQuery(p.facts[0])
}

// MultiQueryValues returns the user-query-to-value map for ShapeMultiQuery
// rendering.
func (p *Projection) MultiQueryValues() map[string]any {
	return factsForQueries(p.facts)
}

func uniqueQueries(facts []ResolvedFact) []string {
	seen := make(map[string]bool, len(facts))
	queries := make([]string, 0, len(facts))
	for _, fact := range facts {
		if seen[fact.UserQuery] {
			continue
		}
		seen[fact.UserQuery] = true
		queries = append(queries, fact.UserQuery)
	}
	return queries
}

func factsForQueries(facts []ResolvedFact) map[string]any {
	values := make(map[string]any, len(facts))
	for _, fact := range facts {
		values[fact.UserQuery] = ValueForQuery(fact)
	}
	return values
}

// findFactIn returns the resolved fact for query using a precomputed
// collection for the nested-query fallback dig, so callers that already hold
// the canonical tree do not rebuild it.
func findFactIn(facts []ResolvedFact, collection map[string]any, query string) ResolvedFact {
	query = strings.ToLower(query)
	for i := len(facts) - 1; i >= 0; i-- {
		fact := facts[i]
		if factMatchesQuery(fact.Name, query) {
			fact.UserQuery = query
			return fact
		}
	}
	if value, found := digValue(collection, strings.Split(query, ".")); found {
		return ResolvedFact{Name: query, UserQuery: query, Value: value}
	}
	return ResolvedFact{Name: query, UserQuery: query, Type: "nil"}
}

func factMatchesQuery(factName, query string) bool {
	if strings.Contains(factName, ".*") && !strings.Contains(query, ".") {
		pattern := strings.ReplaceAll(regexp.QuoteMeta(factName), `\.\*`, `.*`)
		matched, err := regexp.MatchString("^"+pattern+"$", query)
		return err == nil && matched
	}
	return query == factName || strings.HasPrefix(query, factName+".")
}
