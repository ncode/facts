package engine

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

const rubyTTLUnits = "ns, nanos, nanoseconds, us, micros, microseconds, ms, milis, milliseconds, s, seconds, m, minutes, h, hours, d, days"

// FactGroup is a named set of facts used by Facter's block/cache group CLI.
type FactGroup struct {
	Name  string
	Facts []string
}

// BuiltinFactGroups returns the static fact group catalog for the Go port.
// Each group keeps only its structured root fact; the legacy flat aliases
// (memoryfree, hostname, operatingsystem, processorcount, …) were removed by
// ADR-0007 and never resolve, so listing them was a no-op. Disabling a group
// name still drops the whole subtree through the structured root.
func BuiltinFactGroups() []FactGroup {
	return cloneFactGroups(builtinFactGroupsFromDescriptors())
}

// MergeFactGroups returns defaults with configured groups replacing same-name defaults.
func MergeFactGroups(defaults, configured []FactGroup) []FactGroup {
	if len(configured) == 0 {
		return defaults
	}
	merged := append([]FactGroup(nil), defaults...)
	indexes := make(map[string]int, len(merged))
	for i, group := range merged {
		indexes[group.Name] = i
	}
	for _, group := range configured {
		if index, ok := indexes[group.Name]; ok {
			merged[index] = group
			continue
		}
		indexes[group.Name] = len(merged)
		merged = append(merged, group)
	}
	return merged
}

// FactGroupName returns the name of the group containing fact.
func FactGroupName(groups []FactGroup, fact string) (string, bool) {
	for _, group := range groups {
		for _, groupFact := range group.Facts {
			if groupFact == fact || strings.HasPrefix(fact, groupFact+".") {
				return group.Name, true
			}
		}
	}
	return "", false
}

// FormatFactGroups renders groups in the same YAML-like shape as Ruby Facter.
func FormatFactGroups(groups []FactGroup) string {
	var b strings.Builder
	for _, group := range groups {
		b.WriteString(group.Name)
		b.WriteByte('\n')
		for _, fact := range group.Facts {
			b.WriteString("- ")
			b.WriteString(fact)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// GroupTTLSeconds returns the configured TTL for fact in seconds.
func GroupTTLSeconds(ttls []FactTTL, fact string, log *slog.Logger) (int64, bool) {
	for _, ttl := range ttls {
		if ttl.Fact != fact {
			continue
		}
		seconds, ok := ttlSeconds(ttl.TTL, log)
		return seconds, ok
	}
	return 0, false
}

func ttlSeconds(value string, log *slog.Logger) (int64, bool) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	end := 0
	if value[0] == '-' || value[0] == '+' {
		end = 1
	}
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == 0 || value[:end] == "-" || value[:end] == "+" {
		return 0, false
	}
	n, err := strconv.ParseInt(value[:end], 10, 64)
	if err != nil {
		return 0, false
	}
	rest := strings.TrimSpace(value[end:])
	unit := "ms"
	if rest != "" {
		if strings.ContainsAny(rest, " \t\n\r") {
			return 0, false
		}
		unit = rest
	}
	multiplier, divisor, ok := ttlUnitScale(unit)
	if !ok {
		log.Error(fmt.Sprintf("Could not parse time unit %s (try %s)", rubyTTLLogUnit(unit), rubyTTLUnits))
		return 0, false
	}
	return n * multiplier / divisor, true
}

func rubyTTLLogUnit(unit string) string {
	if len(unit) > 2 && !strings.HasSuffix(unit, "s") {
		return unit + "s"
	}
	return unit
}

func ttlUnitScale(unit string) (multiplier, divisor int64, ok bool) {
	switch unit {
	case "ns", "nanos", "nanosecond", "nanoseconds":
		return 1, 1_000_000_000, true
	case "us", "micros", "microsecond", "microseconds":
		return 1, 1_000_000, true
	case "ms", "milis", "millisecond", "milliseconds":
		return 1, 1_000, true
	case "s", "second", "seconds":
		return 1, 1, true
	case "m", "minute", "minutes":
		return 60, 1, true
	case "h", "hour", "hours":
		return 3600, 1, true
	case "d", "day", "days":
		return 86400, 1, true
	}
	if strings.EqualFold(unit, "ns") || strings.EqualFold(unit, "nanos") || strings.EqualFold(unit, "nanosecond") || strings.EqualFold(unit, "nanoseconds") {
		return 1, 1_000_000_000, true
	}
	if strings.EqualFold(unit, "us") || strings.EqualFold(unit, "micros") || strings.EqualFold(unit, "microsecond") || strings.EqualFold(unit, "microseconds") {
		return 1, 1_000_000, true
	}
	if strings.EqualFold(unit, "ms") || strings.EqualFold(unit, "milis") || strings.EqualFold(unit, "millisecond") || strings.EqualFold(unit, "milliseconds") {
		return 1, 1_000, true
	}
	if strings.EqualFold(unit, "s") || strings.EqualFold(unit, "second") || strings.EqualFold(unit, "seconds") {
		return 1, 1, true
	}
	if strings.EqualFold(unit, "m") || strings.EqualFold(unit, "minute") || strings.EqualFold(unit, "minutes") {
		return 60, 1, true
	}
	if strings.EqualFold(unit, "h") || strings.EqualFold(unit, "hour") || strings.EqualFold(unit, "hours") {
		return 3600, 1, true
	}
	if strings.EqualFold(unit, "d") || strings.EqualFold(unit, "day") || strings.EqualFold(unit, "days") {
		return 86400, 1, true
	}
	return 0, 0, false
}

// DisabledFactsWithGroups expands the disabled set into concrete fact
// names using the built-in group catalog plus any configured groups.
func DisabledFactsWithGroups(entries []string, configured []FactGroup) map[string]bool {
	disabled := make(map[string]bool)
	groups := MergeFactGroups(BuiltinFactGroups(), configured)
	for _, entry := range entries {
		matchedGroup := false
		for _, group := range groups {
			if group.Name != entry {
				continue
			}
			matchedGroup = true
			for _, fact := range group.Facts {
				disabled[fact] = true
			}
		}
		if !matchedGroup {
			disabled[entry] = true
		}
	}
	return disabled
}

// DisabledUnion is the disabled-fact set both the version fast path and
// discovery planning derive from: the config disable/blocklist list, the
// --disable extraDisabled entries, and the FACTS_DISABLE control from environ,
// each expanded through the config's fact groups. Deriving both callers from
// this one function is what guarantees the fast path takes effect exactly when a
// full discovery would omit the queried fact. Pass a nil environ to exclude the
// environment source (the library default when SystemDefaults is off).
func DisabledUnion(config Config, extraDisabled []string, environ []string) map[string]bool {
	entries := append([]string(nil), config.Disabled...)
	entries = append(entries, extraDisabled...)
	entries = append(entries, environmentDisabledFacts(environ)...)
	return DisabledFactsWithGroups(entries, config.FactGroups)
}

func factHierarchyCovers(ancestor, name string) bool {
	return ancestor == name || strings.HasPrefix(name, ancestor+".")
}

func factHierarchyMatch[T any](name string, lookup func(string) (T, bool)) (string, T, bool) {
	for {
		if value, ok := lookup(name); ok {
			return name, value, true
		}
		cut := strings.LastIndex(name, ".")
		if cut < 0 {
			var zero T
			return "", zero, false
		}
		name = name[:cut]
	}
}

// FilterDisabledFacts removes facts whose root name is disabled.
func FilterDisabledFacts(facts []ResolvedFact, disabled map[string]bool) []ResolvedFact {
	if len(disabled) == 0 {
		return facts
	}
	filtered := make([]ResolvedFact, 0, len(facts))
	for _, fact := range facts {
		root, _, _ := strings.Cut(fact.Name, ".")
		if disabled[fact.Name] || disabled[root] {
			continue
		}
		fact.Value = pruneDisabledDescendants(fact.Name, fact.Value, disabled)
		filtered = append(filtered, fact)
	}
	return filtered
}

func pruneDisabledDescendants(name string, value any, disabled map[string]bool) any {
	var pruned any
	for disabledName := range disabled {
		if disabledName == name || !factHierarchyCovers(name, disabledName) {
			continue
		}
		if pruned == nil {
			pruned = deepCopyValue(value)
		}
		pruned = pruneDottedValue(pruned, strings.Split(strings.TrimPrefix(disabledName, name+"."), "."))
	}
	if pruned == nil {
		return value
	}
	return pruned
}

func pruneDottedValue(value any, parts []string) any {
	if len(parts) == 0 {
		return value
	}
	switch v := value.(type) {
	case map[string]any:
		if len(parts) == 1 {
			delete(v, parts[0])
			return v
		}
		child, ok := v[parts[0]]
		if !ok {
			return v
		}
		v[parts[0]] = pruneDottedValue(child, parts[1:])
		if childMap, ok := v[parts[0]].(map[string]any); ok && len(childMap) == 0 {
			delete(v, parts[0])
		}
	}
	return value
}
