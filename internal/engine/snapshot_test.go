package engine

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// NormalizeCustomValue is part of the input contract: a custom fact value must
// be canonicalized into tree-shaped data before it enters the snapshot. These
// tests pin the three transforms it performs (time → RFC3339, typed string-key
// maps → map[string]any, recursion) and the values it must leave untouched.
func TestNormalizeCustomValue(t *testing.T) {
	ts := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name string
		in   any
		want any
	}{
		{"time becomes RFC3339 string", ts, "2020-01-02T03:04:05Z"},
		{"plain int unchanged", 7, 7},
		{"plain string unchanged", "hi", "hi"},
		{"nil unchanged", nil, nil},
		{
			name: "time inside slice is normalized",
			in:   []any{ts, "x"},
			want: []any{"2020-01-02T03:04:05Z", "x"},
		},
		{
			name: "time inside map[string]any is normalized",
			in:   map[string]any{"when": ts, "who": "me"},
			want: map[string]any{"when": "2020-01-02T03:04:05Z", "who": "me"},
		},
		{
			name: "typed string-keyed map is widened to map[string]any",
			in:   map[string]int{"a": 1, "b": 2},
			want: map[string]any{"a": 1, "b": 2},
		},
		{
			name: "non-string-keyed map is left untouched",
			in:   map[int]string{1: "a"},
			want: map[int]string{1: "a"},
		},
		{
			name: "nested typed map with time is widened and normalized",
			in:   []any{map[string]any{"t": ts}},
			want: []any{map[string]any{"t": "2020-01-02T03:04:05Z"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeCustomValue(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeCustomValue(%#v) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

// CustomValueContainsNullByte guards a hard input-contract rule: NUL bytes are
// rejected. It must catch a NUL anywhere — in a scalar, a slice element, a map
// value, or a map *key* — and report clean values as clean.
func TestCustomValueContainsNullByte(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want bool
	}{
		{"clean string", "hello", false},
		{"string with NUL", "he\x00llo", true},
		{"clean slice", []any{"a", "b"}, false},
		{"slice with NUL element", []any{"a", "b\x00"}, true},
		{"clean map", map[string]any{"k": "v"}, false},
		{"map value with NUL", map[string]any{"k": "v\x00"}, true},
		{"map key with NUL", map[string]any{"k\x00": "v"}, true},
		{"nested NUL deep in slice-of-map", []any{map[string]any{"k": []any{"x\x00"}}}, true},
		{"nil", nil, false},
		{"clean typed array", [2]string{"a", "b"}, false},
		{"typed slice with NUL element", []string{"a", "b\x00"}, true},
		{"typed map with slice NUL element", map[string][]string{"k": {"v\x00"}}, true},
		{"typed map key with NUL", map[string]string{"k\x00": "v"}, true},
		{"pointer to string with NUL", ptrTo("x\x00"), true},
		{"nil pointer", (*string)(nil), false},
		{"non-string scalar is never a NUL", 42, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CustomValueContainsNullByte(tt.in); got != tt.want {
				t.Errorf("CustomValueContainsNullByte(%#v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSnapshotValueReportsMissingFactSentinel(t *testing.T) {
	snap := newSnapshot([]ResolvedFact{
		{Name: "present", Type: "external", UserQuery: "present", Value: nil},
	}, discardLog())

	got, err := snap.Value("present")
	if err != nil || got != nil {
		t.Fatalf("Value(present) = %#v, %v, want nil, nil", got, err)
	}

	got, err = snap.Value("missing")
	if got != nil || !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(missing) = %#v, %v, want ErrFactNotFound", got, err)
	}
}

func TestSnapshotTreeReturnsDeepCopy(t *testing.T) {
	snap := newSnapshot([]ResolvedFact{
		{Name: "root.child", Value: []any{"original"}},
	}, discardLog())

	tree := snap.Tree()
	tree["root"].(map[string]any)["child"].([]any)[0] = "mutated"

	fresh := snap.Tree()
	if got := fresh["root"].(map[string]any)["child"].([]any)[0]; got != "original" {
		t.Fatalf("fresh Tree() after Tree mutation = %#v, want original", got)
	}

	got, err := snap.Value("root.child.0")
	if err != nil || got != "original" {
		t.Fatalf("Value(root.child.0) after Tree mutation = %#v, %v, want original", got, err)
	}
}

func TestSnapshotOutputProjectionReturnsDeepCopies(t *testing.T) {
	type node struct {
		Value string
		Next  *node
	}
	type payload struct {
		First   map[string]any
		Second  map[string]any
		Slice   []string
		Array   [1]*node
		Pointer *node
	}

	shared := map[string]any{"value": "original"}
	cycle := &node{Value: "original"}
	cycle.Next = cycle
	snap := newSnapshot([]ResolvedFact{{
		Name: "root",
		Value: payload{
			First:   shared,
			Second:  shared,
			Slice:   []string{"original"},
			Array:   [1]*node{cycle},
			Pointer: cycle,
		},
	}}, discardLog())

	projected := snap.OutputProjection(false).FullTree()["root"].(payload)
	if reflect.ValueOf(projected.First).Pointer() != reflect.ValueOf(projected.Second).Pointer() {
		t.Fatal("OutputProjection() did not preserve a shared map alias")
	}
	if projected.Array[0] != projected.Pointer {
		t.Fatal("OutputProjection() did not preserve a shared pointer alias")
	}
	if projected.Pointer.Next != projected.Pointer {
		t.Fatal("OutputProjection() did not preserve a pointer cycle")
	}
	projected.First["value"] = "mutated"
	projected.Slice[0] = "mutated"
	projected.Pointer.Value = "mutated"

	fresh := snap.OutputProjection(false).FullTree()["root"].(payload)
	if got := fresh.First["value"]; got != "original" {
		t.Fatalf("fresh OutputProjection().First[value] = %#v, want original", got)
	}
	if got := fresh.Second["value"]; got != "original" {
		t.Fatalf("fresh OutputProjection().Second[value] = %#v, want original", got)
	}
	if got := fresh.Slice[0]; got != "original" {
		t.Fatalf("fresh OutputProjection().Slice[0] = %#v, want original", got)
	}
	if got := fresh.Array[0].Value; got != "original" {
		t.Fatalf("fresh OutputProjection().Array[0].Value = %#v, want original", got)
	}
	if got := fresh.Pointer.Value; got != "original" {
		t.Fatalf("fresh OutputProjection().Pointer.Value = %#v, want original", got)
	}
}

func TestSnapshotOutputProjectionKeepsForceDotSeparate(t *testing.T) {
	snap := newSnapshot([]ResolvedFact{{Name: "a.b.c", Value: "external", Type: "external"}}, discardLog())
	wantTree := snap.Tree()

	presentation := snap.OutputProjection(true)
	got := presentation.FullTree()["a"].(map[string]any)["b"].(map[string]any)["c"]
	if got != "external" {
		t.Fatalf("force-dot OutputProjection().FullTree()[a][b][c] = %#v, want external", got)
	}
	presentation.FullTree()["a"].(map[string]any)["b"].(map[string]any)["c"] = "mutated"

	if got := snap.Tree(); !reflect.DeepEqual(got, wantTree) {
		t.Fatalf("Tree() after force-dot presentation mutation = %#v, want %#v", got, wantTree)
	}
	if got, err := snap.Value("a.b"); got != nil || !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(a.b) = %#v, %v, want ErrFactNotFound", got, err)
	}
	var names []string
	for name := range snap.All() {
		names = append(names, name)
	}
	if want := []string{"a.b.c"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("All() names = %#v, want %#v", names, want)
	}
}

func TestSnapshotCopiesPointerValues(t *testing.T) {
	original := map[string]any{"child": "original"}
	snap := newSnapshot([]ResolvedFact{{Name: "root", Value: &original}}, discardLog())

	original["child"] = "outside"
	got, err := snap.Value("root")
	if err != nil {
		t.Fatal(err)
	}
	if got := (*got.(*map[string]any))["child"]; got != "original" {
		t.Fatalf("Value(root) after source mutation = %#v, want original", got)
	}
	(*got.(*map[string]any))["child"] = "mutated"
	freshValue, err := snap.Value("root")
	if err != nil {
		t.Fatal(err)
	}
	if got := (*freshValue.(*map[string]any))["child"]; got != "original" {
		t.Fatalf("fresh Value(root) after pointer mutation = %#v, want original", got)
	}

	projected := snap.OutputProjection(false).FullTree()["root"].(*map[string]any)
	(*projected)["child"] = "mutated"
	fresh := snap.OutputProjection(false).FullTree()["root"].(*map[string]any)
	if got := (*fresh)["child"]; got != "original" {
		t.Fatalf("fresh OutputProjection().FullTree()[root] after pointer mutation = %#v, want original", got)
	}

	tree := snap.Tree()
	(*tree["root"].(*map[string]any))["child"] = "mutated"
	freshTree := snap.Tree()
	if got := (*freshTree["root"].(*map[string]any))["child"]; got != "original" {
		t.Fatalf("fresh Tree()[root] after pointer mutation = %#v, want original", got)
	}
}

func TestSnapshotValuePreservesTypedNilCollections(t *testing.T) {
	var items []any
	var labels map[string]string
	snap := newSnapshot([]ResolvedFact{
		{Name: "items", Value: items},
		{Name: "labels", Value: labels},
	}, discardLog())

	gotItems, err := snap.Value("items")
	if err != nil {
		t.Fatal(err)
	}
	itemsValue, ok := gotItems.([]any)
	if !ok {
		t.Fatalf("Value(items) = %T, want []any", gotItems)
	}
	if itemsValue != nil {
		t.Fatalf("Value(items) = %#v, want typed nil slice", itemsValue)
	}

	gotLabels, err := snap.Value("labels")
	if err != nil {
		t.Fatal(err)
	}
	labelsValue, ok := gotLabels.(map[string]string)
	if !ok {
		t.Fatalf("Value(labels) = %T, want map[string]string", gotLabels)
	}
	if labelsValue != nil {
		t.Fatalf("Value(labels) = %#v, want typed nil map", labelsValue)
	}
}

// deepCopyValue backs Snapshot.Tree/All, which promise that mutating the
// returned value cannot affect the snapshot. This proves the copy is deep:
// mutating a nested map and slice in the copy leaves the original intact.
func TestDeepCopyValueIsIndependent(t *testing.T) {
	original := map[string]any{
		"nested": map[string]any{"inner": "orig"},
		"list":   []any{"a", map[string]any{"deep": "orig"}},
	}

	copied, ok := deepCopyValue(original).(map[string]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want map[string]any", deepCopyValue(original))
	}

	// Mutate every nesting level of the copy.
	copied["nested"].(map[string]any)["inner"] = "mutated"
	copied["list"].([]any)[0] = "mutated"
	copied["list"].([]any)[1].(map[string]any)["deep"] = "mutated"

	if got := original["nested"].(map[string]any)["inner"]; got != "orig" {
		t.Errorf("original nested map mutated through copy: inner = %q, want %q", got, "orig")
	}
	if got := original["list"].([]any)[0]; got != "a" {
		t.Errorf("original slice element mutated through copy: [0] = %v, want %q", got, "a")
	}
	if got := original["list"].([]any)[1].(map[string]any)["deep"]; got != "orig" {
		t.Errorf("original slice-of-map mutated through copy: deep = %q, want %q", got, "orig")
	}
}

// deepCopyValue must also handle map[any]any (YAML-decoded maps), not just
// map[string]any, or external YAML facts would alias the snapshot.
func TestDeepCopyValueHandlesAnyKeyedMap(t *testing.T) {
	original := map[any]any{1: map[any]any{"inner": "orig"}}

	copied, ok := deepCopyValue(original).(map[any]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want map[any]any", deepCopyValue(original))
	}
	copied[1].(map[any]any)["inner"] = "mutated"

	if got := original[1].(map[any]any)["inner"]; got != "orig" {
		t.Errorf("original map[any]any mutated through copy: inner = %q, want %q", got, "orig")
	}
}

func TestDeepCopyValueHandlesNilAnyMapKey(t *testing.T) {
	originalValue := map[string]any{"name": "original"}
	original := map[any]any{nil: originalValue}

	copied, ok := deepCopyValue(original).(map[any]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want map[any]any", deepCopyValue(original))
	}
	copiedValue, ok := copied[nil].(map[string]any)
	if !ok {
		t.Fatalf("copied nil-key value = %T, want map[string]any", copied[nil])
	}
	copiedValue["name"] = "mutated"

	if got := originalValue["name"]; got != "original" {
		t.Fatalf("mutating copied nil-key map value changed original: %v", got)
	}
}

func TestDeepCopyValueHandlesNilSliceElements(t *testing.T) {
	original := []any{nil, map[string]any{"name": "original"}}

	copied, ok := deepCopyValue(original).([]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want []any", deepCopyValue(original))
	}
	if copied[0] != nil {
		t.Fatalf("copied[0] = %#v, want nil", copied[0])
	}
	copied[1].(map[string]any)["name"] = "mutated"

	if got := original[1].(map[string]any)["name"]; got != "original" {
		t.Fatalf("mutating copied map after nil element changed original: %v", got)
	}
}

func TestDeepCopyValuePreservesEmptyNonNilSlice(t *testing.T) {
	original := make([]any, 0)

	copied, ok := deepCopyValue(original).([]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want []any", deepCopyValue(original))
	}
	if copied == nil {
		t.Fatal("deepCopyValue returned nil for empty non-nil slice")
	}
	if len(copied) != 0 {
		t.Fatalf("len(copied) = %d, want 0", len(copied))
	}
}

func TestDeepCopyValueHandlesTypedSlicesInMaps(t *testing.T) {
	original := map[string][]int{"numbers": {1, 2}}

	copied, ok := deepCopyValue(original).(map[string][]int)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want map[string][]int", deepCopyValue(original))
	}
	copied["numbers"][0] = 99

	if got := original["numbers"][0]; got != 1 {
		t.Errorf("original typed slice mutated through copy: numbers[0] = %d, want 1", got)
	}
}

func TestDeepCopyValueHandlesTypedSliceReflectPath(t *testing.T) {
	type typedMaps []map[string]any
	original := typedMaps{{"name": "first"}}

	copied, ok := deepCopyValue(original).(typedMaps)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want typedMaps", deepCopyValue(original))
	}
	copied[0]["name"] = "mutated"

	if got := original[0]["name"]; got != "first" {
		t.Fatalf("original typed slice element mutated through copy: %q, want first", got)
	}
}

func TestDeepCopyValueHandlesReflectMapWithNilAndNestedValues(t *testing.T) {
	original := map[int]any{
		1: nil,
		2: map[string]any{"name": "second"},
	}

	copied, ok := deepCopyValue(original).(map[int]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want map[int]any", deepCopyValue(original))
	}
	if copied[1] != nil {
		t.Fatalf("copied nil value = %#v, want nil", copied[1])
	}
	copied[2].(map[string]any)["name"] = "mutated"

	if got := original[2].(map[string]any)["name"]; got != "second" {
		t.Fatalf("original reflected map value mutated through copy: %q, want second", got)
	}
}

func TestDeepCopyValueCopiesPointerMapKeys(t *testing.T) {
	type payload struct {
		Name string
	}
	originalKey := &payload{Name: "original"}
	original := map[*payload]any{originalKey: originalKey}

	copied, ok := deepCopyValue(original).(map[*payload]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want map[*payload]any", deepCopyValue(original))
	}
	if len(copied) != 1 {
		t.Fatalf("copied map length = %d, want 1", len(copied))
	}
	for copiedKey, copiedValue := range copied {
		if copiedKey == originalKey {
			t.Fatal("deepCopyValue reused original pointer map key")
		}
		if copiedValue.(*payload) != copiedKey {
			t.Fatal("deepCopyValue did not preserve key/value aliasing inside copied graph")
		}
		copiedKey.Name = "mutated"
	}
	if originalKey.Name != "original" {
		t.Fatalf("mutating copied map key changed original key: %q", originalKey.Name)
	}
}

func TestSnapshotAllIteratesSortedCopies(t *testing.T) {
	snap := newSnapshot([]ResolvedFact{
		{Name: "zeta", Value: map[string]any{"nested": "z"}},
		{Name: "beta", Value: "b"},
		{Name: "alpha", Value: []any{"a"}},
		{Name: "delta", Value: "d"},
		{Name: "gamma", Value: "g"},
		{Name: "omega.child", Value: map[string]any{"nested": "o"}},
	}, discardLog())

	var names []string
	for name, value := range snap.All() {
		names = append(names, name)
		switch name {
		case "alpha":
			value.([]any)[0] = "mutated"
		case "zeta":
			value.(map[string]any)["nested"] = "mutated"
		case "omega":
			value.(map[string]any)["child"].(map[string]any)["nested"] = "mutated"
		}
	}

	if want := []string{"alpha", "beta", "delta", "gamma", "omega", "zeta"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("All() names = %#v, want %#v", names, want)
	}
	if got, err := snap.Value("alpha.0"); err != nil || got != "a" {
		t.Fatalf("Value(alpha.0) after All mutation = %#v, %v, want a", got, err)
	}
	if got, err := snap.Value("zeta.nested"); err != nil || got != "z" {
		t.Fatalf("Value(zeta.nested) after All mutation = %#v, %v, want z", got, err)
	}
	if got, err := snap.Value("omega.child.nested"); err != nil || got != "o" {
		t.Fatalf("Value(omega.child.nested) after All mutation = %#v, %v, want o", got, err)
	}
}

func TestSnapshotAllStopsWhenYieldReturnsFalse(t *testing.T) {
	snap := newSnapshot([]ResolvedFact{
		{Name: "alpha", Value: "a"},
		{Name: "beta", Value: "b"},
		{Name: "gamma", Value: "g"},
	}, discardLog())

	var names []string
	for name := range snap.All() {
		names = append(names, name)
		break
	}

	if want := []string{"alpha"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("All() yielded %#v before stop, want %#v", names, want)
	}
}

func TestDeepCopyValueHandlesArrays(t *testing.T) {
	original := [2]map[string]any{{"name": "first"}, {"name": "second"}}

	copied, ok := deepCopyValue(original).([2]map[string]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want [2]map[string]any", deepCopyValue(original))
	}
	copied[0]["name"] = "mutated"

	if got := original[0]["name"]; got != "first" {
		t.Fatalf("original array element mutated through copy: %q, want first", got)
	}
}

func TestDeepCopyValueHandlesPointers(t *testing.T) {
	original := map[string]any{"name": "first"}

	copied, ok := deepCopyValue(&original).(*map[string]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want *map[string]any", deepCopyValue(&original))
	}
	(*copied)["name"] = "mutated"

	if got := original["name"]; got != "first" {
		t.Fatalf("original pointer target mutated through copy: %q, want first", got)
	}
	if copied == &original {
		t.Fatal("deepCopyValue returned original pointer")
	}
}

func TestDeepCopyValueHandlesPointersToStructs(t *testing.T) {
	type payload struct {
		Data map[string]any
	}
	original := payload{Data: map[string]any{"name": "first"}}

	copied, ok := deepCopyValue(&original).(*payload)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want *payload", deepCopyValue(&original))
	}
	copied.Data["name"] = "mutated"

	if got := original.Data["name"]; got != "first" {
		t.Fatalf("original pointer target mutated through struct copy: %q, want first", got)
	}
	if copied == &original {
		t.Fatal("deepCopyValue returned original pointer")
	}
}

func TestDeepCopyValuePreservesUnexportedStructFieldsByValueOnly(t *testing.T) {
	type payload struct {
		data map[string]any
	}
	original := payload{data: map[string]any{"name": "first"}}

	copied, ok := deepCopyValue(original).(payload)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want payload", deepCopyValue(original))
	}
	copied.data["name"] = "mutated"

	if got := original.data["name"]; got != "mutated" {
		t.Fatalf("unexported map field was deep-copied, got original %q; want shallow value copy", got)
	}
}

func TestDeepCopyValueHandlesPointerCycles(t *testing.T) {
	type node struct {
		Next *node
	}
	original := &node{}
	original.Next = original

	copied, ok := deepCopyValue(original).(*node)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want *node", deepCopyValue(original))
	}
	if copied == original {
		t.Fatal("deepCopyValue returned original pointer")
	}
	if copied.Next == nil {
		t.Fatal("deepCopyValue cycle copy has nil Next")
	}
	if copied.Next != copied {
		t.Fatal("deepCopyValue cycle copy points outside the copied graph")
	}
	copied.Next.Next = nil
	if original.Next != original {
		t.Fatal("mutating copied cycle changed original pointer graph")
	}
}

func TestDeepCopyValueHandlesMapCycles(t *testing.T) {
	original := map[string]any{"name": "original"}
	original["self"] = original

	copied, ok := deepCopyValue(original).(map[string]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want map[string]any", deepCopyValue(original))
	}
	self, ok := copied["self"].(map[string]any)
	if !ok {
		t.Fatalf("copied self = %T, want map[string]any", copied["self"])
	}
	if reflect.ValueOf(self).Pointer() != reflect.ValueOf(copied).Pointer() {
		t.Fatal("deepCopyValue map cycle points outside the copied graph")
	}
	self["name"] = "mutated"
	if got := copied["name"]; got != "mutated" {
		t.Fatalf("copied self mutation did not affect copied map: %v", got)
	}
	if got := original["name"]; got != "original" {
		t.Fatalf("mutating copied map cycle changed original: %v", got)
	}
}

func TestDeepCopyValueHandlesSliceCycles(t *testing.T) {
	original := []any{"placeholder"}
	original[0] = original

	copied, ok := deepCopyValue(original).([]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want []any", deepCopyValue(original))
	}
	self, ok := copied[0].([]any)
	if !ok {
		t.Fatalf("copied self = %T, want []any", copied[0])
	}
	if reflect.ValueOf(self).Pointer() != reflect.ValueOf(copied).Pointer() {
		t.Fatal("deepCopyValue slice cycle points outside the copied graph")
	}
	self[0] = "mutated"
	if got := copied[0]; got != "mutated" {
		t.Fatalf("copied self mutation did not affect copied slice: %v", got)
	}
	originalSelf, ok := original[0].([]any)
	if !ok {
		t.Fatalf("original self = %T, want []any", original[0])
	}
	if reflect.ValueOf(originalSelf).Pointer() != reflect.ValueOf(original).Pointer() {
		t.Fatal("mutating copied slice cycle changed original graph")
	}
}

func TestDeepCopyValueDoesNotConflateOverlappingSlices(t *testing.T) {
	base := []any{"first", "second"}
	original := []any{base[:1], base[:2]}

	copied, ok := deepCopyValue(original).([]any)
	if !ok {
		t.Fatalf("deepCopyValue returned %T, want []any", deepCopyValue(original))
	}
	first, ok := copied[0].([]any)
	if !ok {
		t.Fatalf("copied[0] = %T, want []any", copied[0])
	}
	second, ok := copied[1].([]any)
	if !ok {
		t.Fatalf("copied[1] = %T, want []any", copied[1])
	}
	if len(first) != 1 {
		t.Fatalf("len(copied[0]) = %d, want 1", len(first))
	}
	if len(second) != 2 {
		t.Fatalf("len(copied[1]) = %d, want 2", len(second))
	}
	second[0] = "mutated"
	if got := base[0]; got != "first" {
		t.Fatalf("mutating copied overlapping slice changed original: %v", got)
	}
}

func ptrTo[T any](value T) *T {
	return &value
}
