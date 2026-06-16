package engine

import (
	"reflect"
	"testing"
)

// dig is the dot-notation traversal behind every fact query (Snapshot.Value,
// findFact, ValueForQuery). The integration tests only exercise the common
// map[string]any / []any shapes; this pins the branches that handle the other
// real value shapes a fact tree can hold — YAML-decoded map[any]any nodes whose
// keys are not strings, and typed []string / []int slices — plus every way a
// path can dead-end (missing key, out-of-range or non-numeric index, descending
// past a scalar). A bug in any of these returns the wrong value for a query.
func TestDig(t *testing.T) {
	tests := []struct {
		name  string
		value any
		parts []string
		want  any
	}{
		{"empty path returns value unchanged", map[string]any{"a": 1}, nil, map[string]any{"a": 1}},
		{"scalar with empty path", "leaf", nil, "leaf"},

		{"string map one level", map[string]any{"a": 1}, []string{"a"}, 1},
		{"string map nested", map[string]any{"a": map[string]any{"b": 2}}, []string{"a", "b"}, 2},
		{"string map missing key", map[string]any{"a": 1}, []string{"z"}, nil},

		// map[any]any is what YAML external facts decode to.
		{"any map direct string key", map[any]any{"a": 1}, []string{"a"}, 1},
		{"any map fuzzy-matches a non-string key by string form", map[any]any{1: "x"}, []string{"1"}, "x"},
		{"any map missing key", map[any]any{"a": 1}, []string{"z"}, nil},
		{"any map nested via fuzzy key", map[any]any{2: map[any]any{"b": "deep"}}, []string{"2", "b"}, "deep"},

		{"slice valid index", []any{"a", "b"}, []string{"1"}, "b"},
		{"slice out of range", []any{"a"}, []string{"5"}, nil},
		{"slice negative index", []any{"a"}, []string{"-1"}, nil},
		{"slice non-numeric index", []any{"a"}, []string{"x"}, nil},
		{"slice recurses into element", []any{map[string]any{"k": "v"}}, []string{"0", "k"}, "v"},

		{"string slice valid index", []string{"a", "b"}, []string{"0"}, "a"},
		{"string slice out of range", []string{"a"}, []string{"9"}, nil},
		{"string slice cannot descend past a string leaf", []string{"a"}, []string{"0", "x"}, nil},

		{"int slice valid index", []int{10, 20}, []string{"1"}, 20},
		{"int slice out of range", []int{10}, []string{"2"}, nil},
		{"int slice cannot descend past an int leaf", []int{10}, []string{"0", "x"}, nil},

		{"descending past a scalar dead-ends", 5, []string{"x"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dig(tt.value, tt.parts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dig(%#v, %#v) = %#v, want %#v", tt.value, tt.parts, got, tt.want)
			}
		})
	}
}
