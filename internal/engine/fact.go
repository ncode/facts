package engine

import (
	"fmt"
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
// dotted custom and external facts into existing structured facts.
func CollectionWithDottedFacts(facts []ResolvedFact, includeTypedDotted bool) map[string]any {
	root := make(map[string]any, len(facts))
	for _, fact := range facts {
		if fact.Value == nil {
			continue
		}
		parts := strings.Split(fact.Name, ".")
		if len(parts) > 1 && fact.Type != "" && !includeTypedDotted {
			if _, ok := root[parts[0]]; ok {
				if !insert(root, parts, fact.Value) {
					reportCollectionCollision(fact)
				}
				continue
			}
			root[fact.Name] = fact.Value
			continue
		}
		if !insert(root, parts, fact.Value) {
			reportCollectionCollision(fact)
		}
	}
	return root
}

// ValueForQuery returns the value selected by fact.UserQuery from fact.Value.
func ValueForQuery(fact ResolvedFact) any {
	query := fact.UserQuery
	if query == "" || query == fact.Name {
		return fact.Value
	}
	if !strings.HasPrefix(query, fact.Name+".") {
		return dig(fact.Value, strings.Split(query, "."))
	}
	return dig(fact.Value, strings.Split(strings.TrimPrefix(query, fact.Name+"."), "."))
}

func insert(root map[string]any, parts []string, value any) bool {
	if len(parts) == 0 {
		return false
	}
	if len(parts) == 1 {
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

func reportCollectionCollision(fact ResolvedFact) {
	reportError(fmt.Sprintf("%s fact `%s` cannot be added to collection. The format of this fact is incompatible with other facts that belong to `%s` group", factTypeLabel(fact.Type), fact.Name, strings.Split(fact.Name, ".")[0]))
}

func factTypeLabel(factType string) string {
	if factType == "" {
		return "Fact"
	}
	return strings.ToUpper(factType[:1]) + factType[1:]
}

func dig(value any, parts []string) any {
	if len(parts) == 0 {
		return value
	}
	switch v := value.(type) {
	case map[string]any:
		next, ok := v[parts[0]]
		if !ok {
			return nil
		}
		return dig(next, parts[1:])
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
			return nil
		}
		return dig(next, parts[1:])
	case []any:
		index, err := strconv.Atoi(parts[0])
		if err != nil || index < 0 || index >= len(v) {
			return nil
		}
		return dig(v[index], parts[1:])
	case []string:
		index, err := strconv.Atoi(parts[0])
		if err != nil || index < 0 || index >= len(v) {
			return nil
		}
		if len(parts) > 1 {
			return nil
		}
		return v[index]
	case []int:
		index, err := strconv.Atoi(parts[0])
		if err != nil || index < 0 || index >= len(v) {
			return nil
		}
		if len(parts) > 1 {
			return nil
		}
		return v[index]
	default:
		return nil
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
