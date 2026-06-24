package schema

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	targets "github.com/ncode/facts/internal/platform"
	"gopkg.in/yaml.v3"
)

const DefaultPath = "docs/schema/facts.yaml"

// Entry is one documented fact: a dotted path or `*` pattern mapped to its
// type, description, platform list, and schema matching metadata.
type Entry struct {
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Platforms   []string `yaml:"platforms"`
	Conditional bool     `yaml:"conditional"`
	OpenSubtree bool     `yaml:"open_subtree"`
}

// Schema is the parsed facts schema keyed by dotted fact path or pattern.
type Schema map[string]Entry

// Platform is one schema-visible platform target.
type Platform struct {
	ID    string
	Label string
}

// Item is one schema entry paired with its path.
type Item struct {
	Path  string
	Entry Entry
}

var schemaTypes = map[string]bool{
	"string":  true,
	"integer": true,
	"double":  true,
	"boolean": true,
	"map":     true,
	"array":   true,
}

// Platforms returns the schema-visible platform vocabulary.
func Platforms() []Platform {
	profiles := targets.SchemaVisibleProfiles()
	out := make([]Platform, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, Platform{ID: profile.ID, Label: profile.Label})
	}
	return out
}

// LoadFile reads and validates a facts schema YAML file.
func LoadFile(path string) (Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	schema, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	return schema, nil
}

// Parse decodes and validates facts schema YAML data.
func Parse(data []byte) (Schema, error) {
	var schema Schema
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&schema); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("schema contains multiple YAML documents")
		}
		return nil, err
	}
	if err := schema.Validate(); err != nil {
		return nil, err
	}
	return schema, nil
}

// Validate checks the schema file's own shape.
func (s Schema) Validate() error {
	if len(s) == 0 {
		return errors.New("schema has no entries")
	}

	var errs []error
	platformIDs := schemaPlatformIDs()
	for _, pattern := range s.Patterns() {
		entry := s[pattern]
		if strings.TrimSpace(pattern) == "" {
			errs = append(errs, errors.New("entry has empty path"))
		}
		if !schemaTypes[entry.Type] {
			errs = append(errs, fmt.Errorf("entry %q has invalid type %q", pattern, entry.Type))
		}
		if strings.TrimSpace(entry.Description) == "" {
			errs = append(errs, fmt.Errorf("entry %q has no description", pattern))
		}
		if len(entry.Platforms) == 0 {
			errs = append(errs, fmt.Errorf("entry %q lists no platforms", pattern))
		}
		seen := map[string]bool{}
		for _, platform := range entry.Platforms {
			if !platformIDs[platform] {
				errs = append(errs, fmt.Errorf("entry %q lists invalid platform %q", pattern, platform))
			}
			if seen[platform] {
				errs = append(errs, fmt.Errorf("entry %q lists platform %q twice", pattern, platform))
			}
			seen[platform] = true
		}
		if entry.OpenSubtree && entry.Type != "map" && entry.Type != "array" {
			errs = append(errs, fmt.Errorf("entry %q open_subtree requires type map or array", pattern))
		}
	}
	return errors.Join(errs...)
}

func schemaPlatformIDs() map[string]bool {
	ids := make(map[string]bool)
	for _, platform := range Platforms() {
		ids[platform.ID] = true
	}
	return ids
}

// Patterns returns the schema paths in stable order.
func (s Schema) Patterns() []string {
	patterns := make([]string, 0, len(s))
	for pattern := range s {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)
	return patterns
}

// EntriesForPlatform returns schema entries for platform in stable path order.
func (s Schema) EntriesForPlatform(platform string) []Item {
	items := make([]Item, 0, len(s))
	for _, path := range s.Patterns() {
		entry := s[path]
		if PlatformsInclude(entry.Platforms, platform) {
			items = append(items, Item{Path: path, Entry: entry})
		}
	}
	return items
}

// UndocumentedPaths returns the emitted leaf paths no platform-applicable
// schema entry covers.
func (s Schema) UndocumentedPaths(paths []string, platform string) []string {
	var unmatched []string
	for _, path := range paths {
		documented := false
		for pattern, entry := range s {
			if !PlatformsInclude(entry.Platforms, platform) {
				continue
			}
			if MatchesPath(pattern, entry, path) {
				documented = true
				break
			}
		}
		if !documented {
			unmatched = append(unmatched, path)
		}
	}
	return unmatched
}

// MissingEntries returns the non-conditional schema entries for platform that
// no emitted path satisfies.
func (s Schema) MissingEntries(paths []string, platform string) []string {
	var missing []string
	for _, pattern := range s.Patterns() {
		entry := s[pattern]
		if entry.Conditional || !PlatformsInclude(entry.Platforms, platform) {
			continue
		}
		if wildcardPrefixAbsent(pattern, paths) {
			continue
		}
		if !schemaEntryPresent(pattern, entry, paths) {
			missing = append(missing, pattern)
		}
	}
	return missing
}

func schemaEntryPresent(pattern string, entry Entry, paths []string) bool {
	patternSegments := splitPath(pattern)
	lastWildcard := lastSegmentIndex(patternSegments, "*")
	if lastWildcard == -1 || lastWildcard == len(patternSegments)-1 {
		for _, path := range paths {
			if MatchesPath(pattern, entry, path) {
				return true
			}
		}
		return false
	}

	concretePatterns := concreteWildcardPatterns(patternSegments, paths)
	if len(concretePatterns) == 0 {
		return false
	}
	for _, concrete := range concretePatterns {
		present := false
		for _, path := range paths {
			if MatchesPath(concrete, entry, path) {
				present = true
				break
			}
		}
		if !present {
			return false
		}
	}
	return true
}

func concreteWildcardPatterns(patternSegments []string, paths []string) []string {
	lastWildcard := lastSegmentIndex(patternSegments, "*")
	seen := make(map[string]bool)
	var out []string
	for _, path := range paths {
		pathSegments := splitPath(path)
		if len(pathSegments) <= lastWildcard {
			continue
		}
		concrete := make([]string, len(patternSegments))
		matches := true
		for i, segment := range patternSegments {
			if i > lastWildcard {
				concrete[i] = segment
				continue
			}
			if segment == "*" {
				concrete[i] = pathSegments[i]
				continue
			}
			concrete[i] = segment
			if segment != pathSegments[i] {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		pattern := joinEscapedSegments(concrete)
		if !seen[pattern] {
			seen[pattern] = true
			out = append(out, pattern)
		}
	}
	sort.Strings(out)
	return out
}

func joinEscapedSegments(segments []string) string {
	escaped := make([]string, len(segments))
	for i, segment := range segments {
		escaped[i] = escapeSegment(segment)
	}
	return strings.Join(escaped, ".")
}

func lastSegmentIndex(segments []string, target string) int {
	for i := len(segments) - 1; i >= 0; i-- {
		if segments[i] == target {
			return i
		}
	}
	return -1
}

// FlattenTree reduces the canonical tree to sorted leaf paths: maps recurse
// with one segment per key, empty maps are leaves, arrays contribute a single
// path.* marker, and scalars are leaves.
func FlattenTree(tree map[string]any) []string {
	leaves := make([]string, 0, 256)
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch v := value.(type) {
		case map[string]any:
			if len(v) == 0 {
				leaves = append(leaves, prefix)
				return
			}
			for key, item := range v {
				walk(joinPath(prefix, key), item)
			}
		default:
			if value != nil {
				kind := reflect.TypeOf(value).Kind()
				if kind == reflect.Slice || kind == reflect.Array {
					leaves = append(leaves, prefix+".*")
					return
				}
			}
			leaves = append(leaves, prefix)
		}
	}
	for key, value := range tree {
		walk(escapeSegment(key), value)
	}
	sort.Strings(leaves)
	return leaves
}

// MatchesPath reports whether a schema entry covers a flattened leaf path.
// `*` matches exactly one path segment. Map entries are not open subtrees
// unless OpenSubtree is set; array entries cover their flattened path.* item.
func MatchesPath(pattern string, entry Entry, path string) bool {
	patternSegments := splitPath(pattern)
	pathSegments := splitPath(path)
	if matchSegments(patternSegments, pathSegments) {
		return true
	}

	if entry.Type == "array" && len(pathSegments) == len(patternSegments)+1 && pathSegments[len(pathSegments)-1] == "*" {
		return matchSegments(patternSegments, pathSegments[:len(patternSegments)])
	}

	if !entry.OpenSubtree || (entry.Type != "map" && entry.Type != "array") || len(patternSegments) >= len(pathSegments) {
		return false
	}
	return matchSegments(patternSegments, pathSegments[:len(patternSegments)])
}

// PlatformsInclude reports whether platforms contains platform.
func PlatformsInclude(platforms []string, platform string) bool {
	for _, candidate := range platforms {
		if candidate == platform {
			return true
		}
	}
	return false
}

func wildcardPrefixAbsent(pattern string, paths []string) bool {
	patternSegments := splitPath(pattern)
	wildcard := -1
	for i, segment := range patternSegments {
		if segment == "*" {
			wildcard = i
		}
	}
	if wildcard == -1 {
		return false
	}
	if wildcard == len(patternSegments)-1 {
		return false
	}

	for _, path := range paths {
		pathSegments := splitPath(path)
		if len(pathSegments) > wildcard && matchSegments(patternSegments[:wildcard], pathSegments[:wildcard]) {
			return false
		}
	}
	return true
}

func matchSegments(pattern []string, segments []string) bool {
	if len(pattern) != len(segments) {
		return false
	}
	for i, part := range pattern {
		if part != "*" && part != segments[i] {
			return false
		}
	}
	return true
}

func joinPath(prefix, key string) string {
	key = escapeSegment(key)
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func escapeSegment(segment string) string {
	segment = strings.ReplaceAll(segment, `\`, `\\`)
	return strings.ReplaceAll(segment, `.`, `\.`)
}

func splitPath(path string) []string {
	var segments []string
	var segment strings.Builder
	escaped := false
	for _, r := range path {
		switch {
		case escaped:
			segment.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '.':
			segments = append(segments, segment.String())
			segment.Reset()
		default:
			segment.WriteRune(r)
		}
	}
	if escaped {
		segment.WriteByte('\\')
	}
	return append(segments, segment.String())
}
