package engine

import (
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
		{"typed slice with NUL element", []string{"a", "b\x00"}, true},
		{"typed map with slice NUL element", map[string][]string{"k": {"v\x00"}}, true},
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
