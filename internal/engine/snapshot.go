package engine

import (
	"errors"
	"fmt"
	"iter"
	"log/slog"
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
//
// Returned values are defensive copies of the public fact graph: maps, slices,
// arrays, pointers, and exported struct fields are cloned. Unexported struct
// fields in custom fact values are preserved by shallow value copy and are
// outside the deep-clone guarantee.
type Snapshot struct {
	facts      []ResolvedFact
	tree       map[string]any
	projection *Projection
}

func newSnapshot(facts []ResolvedFact, log *slog.Logger) *Snapshot {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	facts = cloneFacts(facts)
	tree, collisions := collectFacts(facts, false)
	for _, fact := range collisions {
		reportCollectionCollision(log, fact)
	}
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
		return deepCopyValue(value), nil
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
	return cloneFacts(sn.facts)
}

func cloneFacts(facts []ResolvedFact) []ResolvedFact {
	out := slices.Clone(facts)
	for i := range out {
		out[i].Value = deepCopyValue(out[i].Value)
	}
	return out
}

func deepCopyValue(value any) any {
	return deepCopyValueSeen(value, map[deepCopyVisit]reflect.Value{})
}

type deepCopyVisit struct {
	typ    reflect.Type
	ptr    uintptr
	length int
}

func deepCopyValueSeen(value any, seen map[deepCopyVisit]reflect.Value) any {
	return deepCopyReflect(value, seen)
}

func deepCopyReflect(value any, seen map[deepCopyVisit]reflect.Value) any {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return value
	}
	switch rv.Kind() {
	case reflect.Interface:
		if rv.IsNil() {
			return value
		}
		return deepCopyValueSeen(rv.Elem().Interface(), seen)
	case reflect.Slice:
		if rv.IsNil() {
			return value
		}
		if copied, ok := deepCopySeenValue(rv, seen); ok {
			return copied.Interface()
		}
		out := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
		rememberDeepCopy(rv, out, seen)
		for i := range rv.Len() {
			setReflectValue(out.Index(i), deepCopyValueSeen(rv.Index(i).Interface(), seen))
		}
		return out.Interface()
	case reflect.Array:
		out := reflect.New(rv.Type()).Elem()
		for i := range rv.Len() {
			setReflectValue(out.Index(i), deepCopyValueSeen(rv.Index(i).Interface(), seen))
		}
		return out.Interface()
	case reflect.Map:
		if rv.IsNil() {
			return value
		}
		if copied, ok := deepCopySeenValue(rv, seen); ok {
			return copied.Interface()
		}
		out := reflect.MakeMapWithSize(rv.Type(), rv.Len())
		rememberDeepCopy(rv, out, seen)
		for _, key := range rv.MapKeys() {
			copiedKey := deepCopyMapKey(key, rv.Type().Key(), seen)
			item := deepCopyValueSeen(rv.MapIndex(key).Interface(), seen)
			itemValue := reflect.ValueOf(item)
			if item == nil {
				itemValue = reflect.Zero(rv.Type().Elem())
			}
			if itemValue.IsValid() && itemValue.Type().AssignableTo(rv.Type().Elem()) {
				out.SetMapIndex(copiedKey, itemValue)
			} else if itemValue.IsValid() && itemValue.Type().ConvertibleTo(rv.Type().Elem()) {
				out.SetMapIndex(copiedKey, itemValue.Convert(rv.Type().Elem()))
			} else {
				out.SetMapIndex(copiedKey, rv.MapIndex(key))
			}
		}
		return out.Interface()
	case reflect.Struct:
		out := reflect.New(rv.Type()).Elem()
		// Preserve unexported fields by value. Deep-copying private reference
		// fields without unsafe is not possible; exported fields are copied below.
		// Pointer-rooted struct cycles are memoized by the pointer case; a top-level
		// value struct has no stable source pointer to register here.
		out.Set(rv)
		for i := range rv.NumField() {
			dst := out.Field(i)
			if !dst.CanSet() {
				continue
			}
			src := rv.Field(i)
			if src.CanInterface() {
				setReflectValue(dst, deepCopyValueSeen(src.Interface(), seen))
				continue
			}
			dst.Set(src)
		}
		return out.Interface()
	case reflect.Pointer:
		if rv.IsNil() {
			return value
		}
		if copied, ok := deepCopySeenValue(rv, seen); ok {
			return copied.Interface()
		}
		out := reflect.New(rv.Type().Elem())
		rememberDeepCopy(rv, out, seen)
		setReflectValue(out.Elem(), deepCopyValueSeen(rv.Elem().Interface(), seen))
		return out.Interface()
	default:
		return value
	}
}

func deepCopyMapKey(key reflect.Value, keyType reflect.Type, seen map[deepCopyVisit]reflect.Value) reflect.Value {
	item := deepCopyValueSeen(key.Interface(), seen)
	if item == nil {
		return reflect.Zero(keyType)
	}
	itemValue := reflect.ValueOf(item)
	if !itemValue.IsValid() || !itemValue.Type().Comparable() {
		return key
	}
	if itemValue.Type().AssignableTo(keyType) {
		return itemValue
	}
	if itemValue.Type().ConvertibleTo(keyType) {
		converted := itemValue.Convert(keyType)
		if converted.Type().Comparable() {
			return converted
		}
	}
	return key
}

func deepCopySeenValue(rv reflect.Value, seen map[deepCopyVisit]reflect.Value) (reflect.Value, bool) {
	visit, ok := deepCopyVisitFor(rv)
	if !ok {
		return reflect.Value{}, false
	}
	copied, ok := seen[visit]
	return copied, ok
}

func rememberDeepCopy(source, copied reflect.Value, seen map[deepCopyVisit]reflect.Value) {
	visit, ok := deepCopyVisitFor(source)
	if ok {
		seen[visit] = copied
	}
}

func deepCopyVisitFor(rv reflect.Value) (deepCopyVisit, bool) {
	var ptr uintptr
	switch rv.Kind() {
	case reflect.Map, reflect.Pointer:
		ptr = rv.Pointer()
	case reflect.Slice:
		if rv.Len() == 0 {
			return deepCopyVisit{}, false
		}
		ptr = uintptr(rv.UnsafePointer())
	default:
		return deepCopyVisit{}, false
	}
	if ptr == 0 {
		return deepCopyVisit{}, false
	}
	visit := deepCopyVisit{typ: rv.Type(), ptr: ptr}
	if rv.Kind() == reflect.Slice {
		visit.length = rv.Len()
	}
	return visit, true
}

func setReflectValue(dst reflect.Value, value any) {
	if value == nil {
		dst.SetZero()
		return
	}
	rv := reflect.ValueOf(value)
	if rv.Type().AssignableTo(dst.Type()) {
		dst.Set(rv)
		return
	}
	if rv.Type().ConvertibleTo(dst.Type()) {
		dst.Set(rv.Convert(dst.Type()))
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
		return customValueReflectContainsNullByte(value)
	}
}

func customValueReflectContainsNullByte(value any) bool {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return false
	}
	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		if rv.IsNil() {
			return false
		}
		return customValueContainsNullByte(rv.Elem().Interface())
	case reflect.Slice, reflect.Array:
		for i := range rv.Len() {
			if customValueContainsNullByte(rv.Index(i).Interface()) {
				return true
			}
		}
		return false
	case reflect.Map:
		for _, key := range rv.MapKeys() {
			if key.Kind() == reflect.String && strings.ContainsRune(key.String(), '\x00') {
				return true
			}
			if customValueContainsNullByte(rv.MapIndex(key).Interface()) {
				return true
			}
		}
		return false
	default:
		return false
	}
}
