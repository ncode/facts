package engine

import (
	"errors"
	"fmt"
	"iter"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
)

// ErrFactNotFound reports a query that no fact resolved. A fact that resolved
// to a nil value is found: Value returns (nil, nil) for it.
var ErrFactNotFound = errors.New("fact not found")

// Snapshot is the immutable result of one discovery run: the canonical tree
// plus pure query operations over it. Safe for concurrent use.
type Snapshot struct {
	facts      []ResolvedFact
	tree       map[string]any
	projection *Projection
}

func newSnapshot(facts []ResolvedFact) *Snapshot {
	tree := Collection(facts)
	return &Snapshot{
		facts:      facts,
		tree:       tree,
		projection: newProjectionWithTree(facts, false, tree),
	}
}

// Value returns the canonical-tree node selected by the dot-notation query —
// the same value the CLI reports for the same query. A query no fact resolved
// returns an error satisfying errors.Is(err, ErrFactNotFound); a custom or
// external fact that legitimately resolved to nil returns (nil, nil).
//
// Lookup reuses the canonical tree built once for the Snapshot; it does not
// rebuild the tree per call.
func (sn *Snapshot) Value(query string) (any, error) {
	if value, found := sn.projection.LookupValue(query); found {
		return value, nil
	}
	return nil, fmt.Errorf("fact %q: %w", query, ErrFactNotFound)
}

// Tree returns a copy of the canonical tree. Mutating the returned value does
// not affect the Snapshot.
func (sn *Snapshot) Tree() map[string]any {
	tree, _ := deepCopyValue(sn.tree).(map[string]any)
	return tree
}

// All iterates the top-level canonical-tree entries in sorted name order.
// Yielded values are copies; mutating them does not affect the Snapshot.
func (sn *Snapshot) All() iter.Seq2[string, any] {
	names := make([]string, 0, len(sn.tree))
	for name := range sn.tree {
		names = append(names, name)
	}
	sort.Strings(names)
	return func(yield func(string, any) bool) {
		for _, name := range names {
			if !yield(name, deepCopyValue(sn.tree[name])) {
				return
			}
		}
	}
}

// Facts returns the resolved facts backing the Snapshot, for the CLI's
// formatter pipeline.
func (sn *Snapshot) Facts() []ResolvedFact {
	return slices.Clone(sn.facts)
}

func deepCopyValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = deepCopyValue(item)
		}
		return out
	case map[any]any:
		out := make(map[any]any, len(v))
		for key, item := range v {
			out[key] = deepCopyValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = deepCopyValue(item)
		}
		return out
	default:
		return value
	}
}

// NormalizeCustomValue canonicalizes a custom fact value: time.Time values
// become RFC 3339 strings and string-keyed maps become map[string]any, so the
// canonical tree holds only tree-shaped data.
func NormalizeCustomValue(value any) any {
	return normalizeCustomValue(value)
}

func normalizeCustomValue(value any) any {
	switch v := value.(type) {
	case time.Time:
		return v.Format(time.RFC3339)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeCustomValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = normalizeCustomValue(item)
		}
		return out
	default:
		if out, ok := normalizeStringKeyMap(v); ok {
			return out
		}
		return value
	}
}

func normalizeStringKeyMap(value any) (map[string]any, bool) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}

	out := make(map[string]any, rv.Len())
	for _, key := range rv.MapKeys() {
		out[key.String()] = normalizeCustomValue(rv.MapIndex(key).Interface())
	}
	return out, true
}

// CustomValueContainsNullByte reports whether any string within value
// contains a null byte, which the input contract rejects.
func CustomValueContainsNullByte(value any) bool {
	return customValueContainsNullByte(value)
}

func customValueContainsNullByte(value any) bool {
	switch v := value.(type) {
	case string:
		return strings.ContainsRune(v, '\x00')
	case []any:
		return slices.ContainsFunc(v, customValueContainsNullByte)
	case map[string]any:
		for key, item := range v {
			if strings.ContainsRune(key, '\x00') || customValueContainsNullByte(item) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
