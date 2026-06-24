package engine

import (
	"reflect"
	"testing"
)

func TestSelectWithDottedFacts_digsPartialQueriesThroughStructuredDottedFacts(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "a.b.c", Value: "external", Type: "external"},
	}

	selected := SelectWithDottedFacts(facts, []string{"a.b", "a"}, true)
	if len(selected) != 2 {
		t.Fatalf("Select() returned %d facts, want 2", len(selected))
	}

	if got, want := ValueForQuery(selected[0]), (map[string]any{"c": "external"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("ValueForQuery(a.b) = %#v, want %#v", got, want)
	}
	if got, want := ValueForQuery(selected[1]), (map[string]any{"b": map[string]any{"c": "external"}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("ValueForQuery(a) = %#v, want %#v", got, want)
	}
}

func TestSelect_keepsTypedDottedFactFlatByDefault(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "a.b.c", Value: "external", Type: "external"},
	}

	selected := Select(facts, []string{"a.b.c", "a.b", "a"})
	if len(selected) != 3 {
		t.Fatalf("Select() returned %d facts, want 3", len(selected))
	}
	if got := ValueForQuery(selected[0]); got != "external" {
		t.Fatalf("ValueForQuery(a.b.c) = %#v, want external", got)
	}
	if got := ValueForQuery(selected[1]); got != nil {
		t.Fatalf("ValueForQuery(a.b) = %#v, want nil", got)
	}
	if got := ValueForQuery(selected[2]); got != nil {
		t.Fatalf("ValueForQuery(a) = %#v, want nil", got)
	}
}

func TestSelect_unmatchedQueryWithRegexMetacharacterReturnsNilFact(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "a_loaded_fact", Type: "custom"},
	}

	selected := Select(facts, []string{"regex(string"})
	if len(selected) != 1 {
		t.Fatalf("Select() returned %d facts, want 1", len(selected))
	}

	got := selected[0]
	if got.Name != "regex(string" {
		t.Fatalf("Name = %q, want regex(string", got.Name)
	}
	if got.UserQuery != "regex(string" {
		t.Fatalf("UserQuery = %q, want regex(string", got.UserQuery)
	}
	if got.Type != "nil" {
		t.Fatalf("Type = %q, want nil", got.Type)
	}
	if got.Value != nil {
		t.Fatalf("Value = %#v, want nil", got.Value)
	}
}

func TestSelect_matchesWildcardFactNameLikeRubyQueryParser(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "ipaddress_.*", Value: "10.0.0.2", Type: "external"},
		{Name: "os.family", Value: "Debian", Type: "core"},
	}

	selected := Select(facts, []string{"ipaddress_ens160"})
	if len(selected) != 1 {
		t.Fatalf("Select() returned %d facts, want 1", len(selected))
	}

	got := selected[0]
	if got.Name != "ipaddress_.*" {
		t.Fatalf("Name = %q, want ipaddress_.*", got.Name)
	}
	if got.UserQuery != "ipaddress_ens160" {
		t.Fatalf("UserQuery = %q, want ipaddress_ens160", got.UserQuery)
	}
	if got.Value != "10.0.0.2" {
		t.Fatalf("Value = %#v, want 10.0.0.2", got.Value)
	}
}

func TestSelect_wildcardFactNameEscapesOtherRegexpCharacters(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "metric[prod].*", Value: "literal", Type: "external"},
	}

	selected := Select(facts, []string{"metric[prod]cpu"})
	if len(selected) != 1 {
		t.Fatalf("Select() returned %d facts, want 1", len(selected))
	}

	got := selected[0]
	if got.Name != "metric[prod].*" {
		t.Fatalf("Name = %q, want metric[prod].*", got.Name)
	}
	if got.Value != "literal" {
		t.Fatalf("Value = %#v, want literal", got.Value)
	}
}

func TestSelect_doesNotMatchWildcardNameForDottedStructuredQuery(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "ssh.*key", Value: "wildcard", Type: "external"},
		{Name: "ssh", Value: map[string]any{"rsa": map[string]any{"key": "structured"}}, Type: "core"},
	}

	selected := Select(facts, []string{"ssh.rsa.key"})
	if len(selected) != 1 {
		t.Fatalf("Select() returned %d facts, want 1", len(selected))
	}

	got := selected[0]
	if got.Name != "ssh" {
		t.Fatalf("Name = %q, want ssh", got.Name)
	}
	if got.UserQuery != "ssh.rsa.key" {
		t.Fatalf("UserQuery = %q, want ssh.rsa.key", got.UserQuery)
	}
	if ValueForQuery(got) != "structured" {
		t.Fatalf("ValueForQuery() = %#v, want structured", ValueForQuery(got))
	}
}

func TestCollectionWithDottedFactsKeepsExistingScalarOnNestedCollision(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "mygroup.fact1", Value: "g1_f1_value", Type: "custom"},
		{Name: "mygroup.fact1.subfact1", Value: "g1_sg1_f1_value", Type: "custom"},
	}

	got := CollectionWithDottedFacts(facts, true)
	want := map[string]any{"mygroup": map[string]any{"fact1": "g1_f1_value"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectionWithDottedFacts() = %#v, want %#v", got, want)
	}
}

func TestCollectionWithDottedFactsKeepsExistingMapOnScalarCollision(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "mygroup.fact1", Value: "g1_f1_value", Type: "custom"},
		{Name: "mygroup", Value: "scalar_value", Type: "custom"},
	}

	got := CollectionWithDottedFacts(facts, true)
	want := map[string]any{"mygroup": map[string]any{"fact1": "g1_f1_value"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectionWithDottedFacts() = %#v, want %#v", got, want)
	}
}

// Collisions are reported once at discovery (newSnapshot), not by the formatter
// path. CollectionWithDottedFacts itself is diagnostic-silent so the formatter
// and query paths that re-run collection never re-emit.
func TestNewSnapshotReportsCollisionOnceLikeRubyFactCollection(t *testing.T) {
	// A scalar fact whose name is also a dotted parent collides in the canonical
	// tree (includeTypedDotted == false, the shape newSnapshot builds).
	facts := []ResolvedFact{
		{Name: "mygroup", Value: "scalar_value"},
		{Name: "mygroup.fact1", Value: "g1_f1_value", Type: "custom"},
	}
	var errors []string
	logger := captureLogger(nil, nil, &errors)

	_ = newSnapshot(facts, logger)

	wantErrors := []string{"Custom fact `mygroup.fact1` cannot be added to collection. The format of this fact is incompatible with other facts that belong to `mygroup` group"}
	if !reflect.DeepEqual(errors, wantErrors) {
		t.Fatalf("errors = %#v, want %#v", errors, wantErrors)
	}
}

// The canonical tree (includeTypedDotted == false, the shape newSnapshot
// builds) stores prefix-related typed facts as flat keys, so they do NOT
// collide at discovery — both survive. The same pair DOES collide only when
// --force-dot-resolution expands them (see
// TestCollectionWithDottedFactsKeepsExistingScalarOnNestedCollision). That
// format-time-only collision is deliberately out of scope for discovery-time
// diagnostics: the canonical tree the Snapshot exposes has no collision, and
// surfacing the force-dot drop is a separate output-contract concern.
func TestNewSnapshotDoesNotReportFormatTimeOnlyCollision(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "mygroup.fact1", Value: "g1_f1_value", Type: "custom"},
		{Name: "mygroup.fact1.subfact1", Value: "g1_sg1_f1_value", Type: "custom"},
	}
	var errors []string
	logger := captureLogger(nil, nil, &errors)

	snap := newSnapshot(facts, logger)

	if len(errors) != 0 {
		t.Fatalf("errors = %#v, want none (no canonical-tree collision)", errors)
	}
	wantTree := map[string]any{
		"mygroup.fact1":          "g1_f1_value",
		"mygroup.fact1.subfact1": "g1_sg1_f1_value",
	}
	if !reflect.DeepEqual(snap.tree, wantTree) {
		t.Fatalf("tree = %#v, want %#v (both facts present as flat keys)", snap.tree, wantTree)
	}
}

// The formatter/query path re-runs collection on every render and must stay
// silent: it returns the merged collection without reporting collisions.
func TestCollectionWithDottedFactsIsSilentOnCollision(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "mygroup.fact1", Value: "g1_f1_value", Type: "custom"},
		{Name: "mygroup.fact1.subfact1", Value: "g1_sg1_f1_value", Type: "custom"},
	}
	got := CollectionWithDottedFacts(facts, true)
	want := map[string]any{"mygroup": map[string]any{"fact1": "g1_f1_value"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectionWithDottedFacts() = %#v, want %#v", got, want)
	}
}
