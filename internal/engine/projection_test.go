package engine

import (
	"reflect"
	"testing"
)

// Task 1.1: selected-query values, full canonical tree output, dotted fact mode.

func TestProjectionSelectExtractsSelectedQueryValues(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os", Value: map[string]any{"release": map[string]any{"major": "13"}}, Type: "core"},
		{Name: "kernel", Value: "Darwin", Type: "core"},
	}
	projection := NewProjection(facts, false)

	selected := projection.Select([]string{"os.release.major", "kernel"})
	if len(selected) != 2 {
		t.Fatalf("Select() returned %d facts, want 2", len(selected))
	}
	if got := ValueForQuery(selected[0]); got != "13" {
		t.Fatalf("ValueForQuery(os.release.major) = %#v, want 13", got)
	}
	if got := ValueForQuery(selected[1]); got != "Darwin" {
		t.Fatalf("ValueForQuery(kernel) = %#v, want Darwin", got)
	}
}

func TestProjectionFullTreeMatchesCollection(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os", Value: map[string]any{"name": "Darwin", "family": "Darwin"}, Type: "core"},
		{Name: "kernel", Value: "Darwin", Type: "core"},
	}
	projection := NewProjection(facts, false)

	if projection.Shape() != ShapeFullTree {
		t.Fatalf("Shape() = %v, want ShapeFullTree", projection.Shape())
	}
	// FullTree must equal the canonical tree the existing Collection builder
	// produces for the same facts.
	want := CollectionWithDottedFacts(facts, false)
	if got := projection.FullTree(); !reflect.DeepEqual(got, want) {
		t.Fatalf("FullTree() = %#v, want %#v", got, want)
	}
	if want := (map[string]any{"os": map[string]any{"name": "Darwin", "family": "Darwin"}, "kernel": "Darwin"}); !reflect.DeepEqual(projection.FullTree(), want) {
		t.Fatalf("FullTree() = %#v, want %#v", projection.FullTree(), want)
	}
}

func TestProjectionDottedFactModeMergesPartialQuery(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "a.b.c", Value: "external", Type: "external"},
	}

	dotted := NewProjection(facts, true).Select([]string{"a.b", "a"})
	if got, want := ValueForQuery(dotted[0]), (map[string]any{"c": "external"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("dotted ValueForQuery(a.b) = %#v, want %#v", got, want)
	}
	if got, want := ValueForQuery(dotted[1]), (map[string]any{"b": map[string]any{"c": "external"}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("dotted ValueForQuery(a) = %#v, want %#v", got, want)
	}

	flat := NewProjection(facts, false).Select([]string{"a.b", "a"})
	if got := ValueForQuery(flat[0]); got != nil {
		t.Fatalf("flat ValueForQuery(a.b) = %#v, want nil", got)
	}
	if got := ValueForQuery(flat[1]); got != nil {
		t.Fatalf("flat ValueForQuery(a) = %#v, want nil", got)
	}
}

func TestProjectionShapeClassification(t *testing.T) {
	tests := []struct {
		name  string
		facts []ResolvedFact
		want  OutputShape
	}{
		{
			name:  "no facts is empty",
			facts: nil,
			want:  ShapeEmpty,
		},
		{
			name:  "no user query is full tree",
			facts: []ResolvedFact{{Name: "kernel", Value: "Darwin"}},
			want:  ShapeFullTree,
		},
		{
			name:  "single user query is single",
			facts: []ResolvedFact{{Name: "kernel", Value: "Darwin", UserQuery: "kernel"}},
			want:  ShapeSingleQuery,
		},
		{
			name: "multiple user queries is multi",
			facts: []ResolvedFact{
				{Name: "kernel", Value: "Darwin", UserQuery: "kernel"},
				{Name: "os.name", Value: "Darwin", UserQuery: "os.name"},
			},
			want: ShapeMultiQuery,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewProjection(tt.facts, false).Shape(); got != tt.want {
				t.Fatalf("Shape() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Task 1.2: CLI strict missing-query behavior, including resolved nil
// external/registered facts. Strict mode treats a selected nil value as
// missing even when the fact itself resolved to nil.

func TestProjectionMissingQueriesReportsUnresolvedQueries(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "kernel", Value: "Darwin", UserQuery: "kernel"},
		{Name: "nope", UserQuery: "nope", Type: "nil"},
	}
	missing := NewProjection(facts, false).MissingQueries(facts)
	if want := []string{"nope"}; !reflect.DeepEqual(missing, want) {
		t.Fatalf("MissingQueries() = %#v, want %#v", missing, want)
	}
}

func TestProjectionMissingQueriesTreatsResolvedNilSelectedValueAsMissing(t *testing.T) {
	// An external/registered fact that resolved to nil is missing for CLI
	// strict mode, unlike Snapshot lookup which treats it as found.
	facts := []ResolvedFact{
		{Name: "blank", Value: nil, UserQuery: "blank", Type: "external"},
		{Name: "blank_custom", Value: nil, UserQuery: "blank_custom", Type: "custom"},
	}
	missing := NewProjection(facts, false).MissingQueries(facts)
	if want := []string{"blank", "blank_custom"}; !reflect.DeepEqual(missing, want) {
		t.Fatalf("MissingQueries() = %#v, want %#v", missing, want)
	}
}

func TestProjectionMissingQueriesIgnoresFullOutputFacts(t *testing.T) {
	// Facts with empty UserQuery (full output) are never missing.
	facts := []ResolvedFact{
		{Name: "kernel", Value: "Darwin"},
		{Name: "blank", Value: nil, Type: "external"},
	}
	missing := NewProjection(facts, false).MissingQueries(facts)
	if len(missing) != 0 {
		t.Fatalf("MissingQueries() = %#v, want empty", missing)
	}
}

// Task 1.3: Snapshot lookup keeps missing facts distinct from resolved nil
// registered/external facts.

func TestProjectionLookupValueDistinguishesMissingFromResolvedNil(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "kernel", Value: "Darwin", Type: "core"},
		{Name: "blank_external", Value: nil, Type: "external"},
		{Name: "blank_custom", Value: nil, Type: "custom"},
	}
	projection := NewProjection(facts, false)

	if value, found := projection.LookupValue("kernel"); !found || value != "Darwin" {
		t.Fatalf("LookupValue(kernel) = (%#v, %v), want (Darwin, true)", value, found)
	}
	// Resolved nil external/custom facts are found, with a nil value.
	if value, found := projection.LookupValue("blank_external"); !found || value != nil {
		t.Fatalf("LookupValue(blank_external) = (%#v, %v), want (nil, true)", value, found)
	}
	if value, found := projection.LookupValue("blank_custom"); !found || value != nil {
		t.Fatalf("LookupValue(blank_custom) = (%#v, %v), want (nil, true)", value, found)
	}
	// A query no fact resolved is missing.
	if value, found := projection.LookupValue("does.not.exist"); found || value != nil {
		t.Fatalf("LookupValue(does.not.exist) = (%#v, %v), want (nil, false)", value, found)
	}
	// A core fact that has no value is missing, not a resolved nil.
	facts = append(facts, ResolvedFact{Name: "blank_core", Value: nil, Type: "core"})
	core := NewProjection(facts, false)
	if value, found := core.LookupValue("blank_core"); found || value != nil {
		t.Fatalf("LookupValue(blank_core) = (%#v, %v), want (nil, false)", value, found)
	}
}

// Task 3.1a acceptance: Snapshot.Value resolves a query without rebuilding the
// canonical tree per call. The projection memoizes the tree, and the Snapshot
// reuses the tree built once at newSnapshot.

func TestProjectionMemoizesCanonicalTree(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin", Type: "core"},
	}
	projection := NewProjection(facts, false)

	first := projection.Collection()
	second := projection.Collection()
	// Same map instance: identity proves the tree was built once, not rebuilt.
	if !sameMap(first, second) {
		t.Fatal("Collection() rebuilt the canonical tree on the second call")
	}
}

func TestSnapshotValueReusesSnapshotTree(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os", Value: map[string]any{"release": map[string]any{"major": "13"}}, Type: "core"},
	}
	sn := newSnapshot(facts, nil)

	// The Snapshot's projection collection must be the very tree built once at
	// newSnapshot, so repeated Value calls never rebuild it.
	if !sameMap(sn.projection.Collection(), sn.tree) {
		t.Fatal("Snapshot projection does not reuse the tree built at newSnapshot")
	}
	for i := range 3 {
		value, err := sn.Value("os.release.major")
		if err != nil {
			t.Fatalf("Value() error = %v", err)
		}
		if value != "13" {
			t.Fatalf("Value() = %#v, want 13", value)
		}
		if !sameMap(sn.projection.Collection(), sn.tree) {
			t.Fatalf("Value() call %d rebuilt the canonical tree", i)
		}
	}
}

func TestSnapshotValueDistinguishesNestedNilFromMissing(t *testing.T) {
	sn := newSnapshot([]ResolvedFact{
		{Name: "external", Value: map[string]any{"blank": nil}, Type: "external"},
	}, nil)

	value, err := sn.Value("external.blank")
	if err != nil {
		t.Fatalf("Value() error = %v, want nil", err)
	}
	if value != nil {
		t.Fatalf("Value() = %#v, want nil", value)
	}
}

func TestSnapshotReturnedMutableValuesDoNotAffectSnapshot(t *testing.T) {
	sn := newSnapshot([]ResolvedFact{
		{Name: "site", Value: map[string]any{"roles": []string{"web"}}, Type: "external"},
	}, nil)

	tree := sn.Tree()
	tree["site"].(map[string]any)["roles"].([]string)[0] = "db"
	value, err := sn.Value("site.roles.0")
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if value != "web" {
		t.Fatalf("Value() after Tree mutation = %#v, want web", value)
	}

	facts := sn.Facts()
	facts[0].Value.(map[string]any)["roles"].([]string)[0] = "db"
	value, err = sn.Value("site.roles.0")
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if value != "web" {
		t.Fatalf("Value() after Facts mutation = %#v, want web", value)
	}
}

// sameMap reports whether a and b are the same underlying map instance.
func sameMap(a, b map[string]any) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}
