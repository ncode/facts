package engine

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
)

// ResolvedFact is a fact value after resolution and before presentation.
type ResolvedFact struct {
	Name      string
	Value     any
	UserQuery string
	Type      string
	File      string
}

// Collection builds the structured fact tree used when no explicit query is provided.
func Collection(facts []ResolvedFact) map[string]any {
	return CollectionWithDottedFacts(facts, false)
}

// CollectionWithDottedFacts builds the structured fact tree and optionally merges
// dotted custom and external facts into existing structured facts. It is
// diagnostic-silent: collisions are reported once at discovery (newSnapshot),
// not here, because the formatter and query paths re-run collection on every
// render and must not re-emit.
func CollectionWithDottedFacts(facts []ResolvedFact, includeTypedDotted bool) map[string]any {
	root, _ := collectFacts(facts, includeTypedDotted)
	return root
}

// collectFacts builds the structured fact tree and returns the facts that
// collided with an existing node (a scalar where a dotted child needs a map, or
// vice versa). Callers that hold a logger report the collisions; pure rendering
// callers discard them.
func collectFacts(facts []ResolvedFact, includeTypedDotted bool) (map[string]any, []ResolvedFact) {
	root := make(map[string]any, len(facts))
	var collisions []ResolvedFact
	for _, fact := range facts {
		if fact.Value == nil {
			continue
		}
		parts := strings.Split(fact.Name, ".")
		if len(parts) > 1 && fact.Type != "" && !includeTypedDotted {
			if _, ok := root[parts[0]]; ok {
				if !insert(root, parts, fact.Value) {
					collisions = append(collisions, fact)
				}
				continue
			}
			root[fact.Name] = fact.Value
			continue
		}
		if !insert(root, parts, fact.Value) {
			collisions = append(collisions, fact)
		}
	}
	return root, collisions
}

// ValueForQuery returns the value selected by fact.UserQuery from fact.Value.
func ValueForQuery(fact ResolvedFact) any {
	value, _ := valueForQuery(fact)
	return value
}

func valueForQuery(fact ResolvedFact) (any, bool) {
	query := fact.UserQuery
	if query == "" || query == fact.Name {
		if fact.Value == nil {
			return nil, fact.Type == "custom" || fact.Type == "external"
		}
		return fact.Value, true
	}
	if !strings.HasPrefix(query, fact.Name+".") {
		return digValue(fact.Value, strings.Split(query, "."))
	}
	return digValue(fact.Value, strings.Split(strings.TrimPrefix(query, fact.Name+"."), "."))
}

func insert(root map[string]any, parts []string, value any) bool {
	if len(parts) == 0 {
		return false
	}
	if len(parts) == 1 {
		if _, ok := root[parts[0]].(map[string]any); ok {
			return false
		}
		root[parts[0]] = value
		return true
	}

	next, ok := root[parts[0]].(map[string]any)
	if !ok {
		if _, exists := root[parts[0]]; exists {
			return false
		}
		next = make(map[string]any)
		root[parts[0]] = next
	}
	return insert(next, parts[1:], value)
}

func reportCollectionCollision(log *slog.Logger, fact ResolvedFact) {
	log.Error(fmt.Sprintf("%s fact `%s` cannot be added to collection. The format of this fact is incompatible with other facts that belong to `%s` group", factTypeLabel(fact.Type), fact.Name, strings.Split(fact.Name, ".")[0]))
}

func factTypeLabel(factType string) string {
	if factType == "" {
		return "Fact"
	}
	return strings.ToUpper(factType[:1]) + factType[1:]
}

func dig(value any, parts []string) any {
	value, _ = digValue(value, parts)
	return value
}

func digValue(value any, parts []string) (any, bool) {
	if len(parts) == 0 {
		return value, true
	}
	switch v := value.(type) {
	case map[string]any:
		next, ok := v[parts[0]]
		if !ok {
			return nil, false
		}
		return digValue(next, parts[1:])
	case map[any]any:
		next, ok := v[parts[0]]
		if !ok {
			for key, value := range v {
				if fmt.Sprint(key) == parts[0] {
					next = value
					ok = true
					break
				}
			}
		}
		if !ok {
			return nil, false
		}
		return digValue(next, parts[1:])
	case []any:
		index, err := strconv.Atoi(parts[0])
		if err != nil || index < 0 || index >= len(v) {
			return nil, false
		}
		return digValue(v[index], parts[1:])
	case []string:
		index, err := strconv.Atoi(parts[0])
		if err != nil || index < 0 || index >= len(v) {
			return nil, false
		}
		if len(parts) > 1 {
			return nil, false
		}
		return v[index], true
	case []int:
		index, err := strconv.Atoi(parts[0])
		if err != nil || index < 0 || index >= len(v) {
			return nil, false
		}
		if len(parts) > 1 {
			return nil, false
		}
		return v[index], true
	default:
		return nil, false
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
