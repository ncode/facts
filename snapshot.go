package facts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"

	"github.com/ncode/facts/internal/engine"
)

// Snapshot is the immutable result of one discovery run: the canonical fact
// tree plus pure query and decode operations over it. Facts within a Snapshot
// are mutually consistent; freshness is obtained by discovering again, never
// by mutating. Safe for concurrent use.
//
// Returned values are defensive copies of the public fact graph: maps, slices,
// arrays, pointers, and exported struct fields are cloned. Unexported struct
// fields in custom fact values are preserved by shallow value copy and are
// outside the deep-clone guarantee.
type Snapshot struct {
	inner *engine.Snapshot
}

// Value returns the canonical-tree node selected by the dot-notation query —
// the same value the facts CLI reports for the same query. A name no fact
// resolved returns an error satisfying errors.Is(err, ErrFactNotFound); a
// custom or external fact that legitimately resolved to nil returns
// (nil, nil).
func (s *Snapshot) Value(query string) (any, error) {
	return s.inner.Value(query)
}

// Tree returns a copy of the canonical tree — the same names, nesting, and
// value normalization the facts CLI reports. Mutating the returned tree does
// not affect the Snapshot.
func (s *Snapshot) Tree() map[string]any {
	return s.inner.Tree()
}

// All iterates the top-level canonical-tree entries in sorted name order.
// Yielded values are copies; mutating them does not affect the Snapshot.
func (s *Snapshot) All() iter.Seq2[string, any] {
	return s.inner.All()
}

// As decodes the canonical-tree subtree selected by query into T, reading
// only from the resolved Snapshot — it never resolves facts itself. It works
// uniformly for core, custom, and external facts, whatever source won
// precedence. A shape mismatch between the subtree and T returns a non-nil
// error and the zero T — never a partially decoded value. A missing fact
// returns an error satisfying errors.Is(err, ErrFactNotFound).
func As[T any](s *Snapshot, query string) (T, error) {
	var zero T
	value, err := s.Value(query)
	if err != nil {
		return zero, err
	}
	jsonReady, err := jsonValue(value)
	if err != nil {
		return zero, fmt.Errorf("fact %q: encode canonical value: %w", query, err)
	}
	encoded, err := json.Marshal(jsonReady)
	if err != nil {
		return zero, fmt.Errorf("fact %q: encode canonical value: %w", query, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	var out T
	if err := decoder.Decode(&out); err != nil {
		return zero, fmt.Errorf("fact %q: decode into %T: %w", query, zero, err)
	}
	return out, nil
}

// jsonValue rewrites map[any]any nodes (YAML decoding artifacts) into
// map[string]any so the value round-trips through encoding/json.
func jsonValue(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			value, err := jsonValue(item)
			if err != nil {
				return nil, err
			}
			out[key] = value
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			name := fmt.Sprint(key)
			if _, exists := out[name]; exists {
				return nil, fmt.Errorf("duplicate map key after string normalization: %q", name)
			}
			value, err := jsonValue(item)
			if err != nil {
				return nil, err
			}
			out[name] = value
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			value, err := jsonValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = value
		}
		return out, nil
	default:
		return value, nil
	}
}
