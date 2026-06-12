package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const cacheFormatVersion = 1

var (
	cacheRemove    = os.Remove
	cacheWriteFile = os.WriteFile
)

// DefaultCachePath returns the platform default directory for cached fact groups.
var DefaultCachePath = platformDefaultCachePath

// FactCache reads and writes Facter-compatible cached fact groups.
type FactCache struct {
	dir    string
	groups map[string]FactGroup
	ttls   map[string]time.Duration
}

// NewFactCache returns a cache using configured TTLs and custom fact groups.
func NewFactCache(dir string, ttls []FactTTL, groups []FactGroup) *FactCache {
	cache := &FactCache{
		dir:    dir,
		groups: make(map[string]FactGroup),
		ttls:   make(map[string]time.Duration),
	}
	for _, group := range BuiltinFactGroups() {
		cache.groups[group.Name] = group
	}
	for _, group := range groups {
		cache.groups[group.Name] = group
	}
	for _, ttl := range ttls {
		duration, ok := parseTTLDuration(ttl.TTL)
		if ok {
			cache.ttls[ttl.Fact] = duration
		}
	}
	return cache
}

// ResolveFacts returns facts still requiring resolution plus facts loaded from a fresh cache.
func (fc *FactCache) ResolveFacts(searched []ResolvedFact) ([]ResolvedFact, []ResolvedFact) {
	if fc == nil || fc.dir == "" || len(fc.ttls) == 0 {
		return searched, nil
	}
	remaining := make([]ResolvedFact, 0, len(searched))
	cached := []ResolvedFact{}
	// Facts in the same cache group share one stat and one file read/parse
	// per ResolveFacts call instead of repeating them per fact.
	type freshKey struct {
		group string
		ttl   time.Duration
	}
	type groupRead struct {
		data map[string]any
		ok   bool
	}
	freshness := make(map[freshKey]bool)
	reads := make(map[string]groupRead)
	for _, fact := range searched {
		group, ttl, ok := fc.cacheGroupForResolvedFact(fact)
		if !ok {
			remaining = append(remaining, fact)
			continue
		}
		fresh, seen := freshness[freshKey{group, ttl}]
		if !seen {
			fresh = fc.cacheFresh(group, ttl)
			freshness[freshKey{group, ttl}] = fresh
		}
		if externalFactInCustomGroup(fact, group) || !fresh {
			remaining = append(remaining, fact)
			continue
		}
		read, seen := reads[group]
		if !seen {
			read.data, read.ok = fc.readCache(group)
			reads[group] = read
		}
		if !read.ok {
			remaining = append(remaining, fact)
			continue
		}
		data := read.data
		if fact.Type == "file" {
			cached = append(cached, cachedExternalFacts(fact, data)...)
			continue
		}
		if !cacheDataHasKeyMatchingFact(data, fact.Name) {
			deleteCacheFile(filepath.Join(fc.dir, group))
			// The file is gone: later facts in this group must miss, as they
			// would if they re-read the cache.
			reads[group] = groupRead{}
			remaining = append(remaining, fact)
			continue
		}
		value, ok := data[fact.Name]
		if !ok {
			remaining = append(remaining, fact)
			continue
		}
		cached = append(cached, ResolvedFact{Name: fact.Name, Value: value, UserQuery: fact.UserQuery, Type: fact.Type, File: fact.File})
	}
	return remaining, cached
}

func cachedExternalFacts(searched ResolvedFact, data map[string]any) []ResolvedFact {
	names := make([]string, 0, len(data))
	for name := range data {
		if name != "cache_format_version" {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	facts := make([]ResolvedFact, 0, len(names))
	for _, name := range names {
		facts = append(facts, ResolvedFact{Name: name, Value: data[name], UserQuery: searched.UserQuery, Type: searched.Type, File: searched.File})
	}
	return facts
}

func cacheDataHasKeyMatchingFact(data map[string]any, name string) bool {
	pattern, err := regexp.Compile(name)
	if err != nil {
		for key := range data {
			if strings.Contains(key, name) {
				return true
			}
		}
		return false
	}
	for key := range data {
		if pattern.MatchString(key) {
			return true
		}
	}
	return false
}

func externalFactInCustomGroup(fact ResolvedFact, group string) bool {
	if fact.Type != "file" || fact.File == "" {
		return false
	}
	factName := filepath.Base(fact.File)
	if group == factName {
		return false
	}
	reportError(fmt.Sprintf("Cannot cache '%s' fact from '%s' group. Caching custom group is not supported for external facts.", factName, group))
	return true
}

// CacheFacts writes resolved facts into configured cache groups.
func (fc *FactCache) CacheFacts(facts []ResolvedFact) error {
	if fc == nil || fc.dir == "" || len(fc.ttls) == 0 {
		return nil
	}
	grouped := make(map[string]map[string]any)
	for _, fact := range facts {
		group, _, ok := fc.cacheGroupForFact(fact.Name)
		if !ok {
			continue
		}
		if grouped[group] == nil {
			grouped[group] = make(map[string]any)
		}
		grouped[group][fact.Name] = fact.Value
	}
	if len(grouped) == 0 {
		return nil
	}
	if err := os.MkdirAll(fc.dir, 0o755); err != nil {
		if warnCacheWriteFailure(err) {
			return nil
		}
		return err
	}
	for group, data := range grouped {
		ttl := fc.ttls[group]
		if !fc.shouldWriteCache(group, ttl, data) {
			continue
		}
		data["cache_format_version"] = cacheFormatVersion
		encoded, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		if err := cacheWriteFile(filepath.Join(fc.dir, group), encoded, 0o600); err != nil {
			if warnCacheWriteFailure(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

func warnCacheWriteFailure(err error) bool {
	if !errors.Is(err, os.ErrPermission) {
		return false
	}
	warn(fmt.Sprintf("Could not write cache: %v", err))
	return true
}

func (fc *FactCache) cacheGroupForFact(name string) (string, time.Duration, bool) {
	bestGroup := ""
	bestTTL := time.Duration(0)
	bestLen := -1
	for configured, ttl := range fc.ttls {
		if group, ok := fc.groups[configured]; ok {
			for _, prefix := range group.Facts {
				if factMatchesPrefix(name, prefix) && len(prefix) > bestLen {
					bestGroup = configured
					bestTTL = ttl
					bestLen = len(prefix)
				}
			}
			continue
		}
		if factMatchesPrefix(name, configured) && len(configured) > bestLen {
			bestGroup = configured
			bestTTL = ttl
			bestLen = len(configured)
		}
	}
	return bestGroup, bestTTL, bestLen >= 0
}

func (fc *FactCache) cacheGroupForResolvedFact(fact ResolvedFact) (string, time.Duration, bool) {
	if fact.Type == "file" && fact.File != "" {
		if group, ttl, ok := fc.cacheGroupForFact(filepath.Base(fact.File)); ok {
			return group, ttl, ok
		}
	}
	return fc.cacheGroupForFact(fact.Name)
}

func factMatchesPrefix(name, prefix string) bool {
	return name == prefix || strings.HasPrefix(name, prefix+".")
}

func (fc *FactCache) cacheFresh(group string, ttl time.Duration) bool {
	path := filepath.Join(fc.dir, group)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.ModTime().Add(ttl).After(time.Now()) {
		return true
	}
	_ = os.Remove(path)
	return false
}

func (fc *FactCache) shouldWriteCache(group string, ttl time.Duration, data map[string]any) bool {
	path := filepath.Join(fc.dir, group)
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	if !info.ModTime().Add(ttl).After(time.Now()) {
		_ = os.Remove(path)
		return true
	}
	return !cacheContainsFacts(path, data)
}

func cacheContainsFacts(path string, data map[string]any) bool {
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var decoded any
	if err := json.Unmarshal(existing, &decoded); err != nil {
		debug(fmt.Sprintf("Failed to read cache file %s. Detail: %v", path, err))
		return false
	}
	if decoded == nil {
		debug(fmt.Sprintf("No keys found in %s. Detail: cached data is nil", path))
		return false
	}
	cached, ok := decoded.(map[string]any)
	if !ok {
		debug(fmt.Sprintf("No keys found in %s. Detail: cached data has no object keys", path))
		return false
	}
	for key := range data {
		if _, ok := cached[key]; !ok {
			return false
		}
	}
	return true
}

func (fc *FactCache) readCache(group string) (map[string]any, bool) {
	path := filepath.Join(fc.dir, group)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		deleteCacheFile(path)
		return nil, false
	}
	if decoded["cache_format_version"] != float64(cacheFormatVersion) {
		deleteCacheFile(path)
		return nil, false
	}
	return decoded, true
}

func deleteCacheFile(path string) {
	if err := cacheRemove(path); err != nil {
		warn(fmt.Sprintf("Could not delete cache: %v", err))
	}
}

func parseTTLDuration(value string) (time.Duration, bool) {
	fields := strings.Fields(value)
	if len(fields) == 0 || len(fields) > 2 {
		return 0, false
	}
	amount, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, false
	}
	unit := ""
	if len(fields) == 2 {
		unit = fields[1]
	}
	if len(unit) > 2 && !strings.HasSuffix(unit, "s") {
		unit += "s"
	}
	multiplier, ok := ttlUnitMultiplier(unit)
	if !ok {
		return 0, false
	}
	duration := time.Duration(amount) * multiplier
	if multiplier < time.Second {
		duration = duration.Truncate(time.Second)
	}
	return duration, true
}

func ttlUnitMultiplier(unit string) (time.Duration, bool) {
	switch unit {
	case "ns", "nanos", "nanoseconds":
		return time.Nanosecond, true
	case "us", "micros", "microseconds":
		return time.Microsecond, true
	case "", "ms", "milis", "milliseconds":
		return time.Millisecond, true
	case "s", "seconds":
		return time.Second, true
	case "m", "minutes":
		return time.Minute, true
	case "h", "hours":
		return time.Hour, true
	case "d", "days":
		return 24 * time.Hour, true
	default:
		return 0, false
	}
}

func platformDefaultCachePath() string {
	return platformDefaultCachePathFor(runtime.GOOS, os.Getenv("ProgramData"), os.Getenv("APPDATA"))
}

// platformDefaultCachePathFor mirrors Ruby Facter's default cache layout with
// the facter-named segment renamed to facts (ADR-0008). There is no compat
// read of the old facter-named location — caches regenerate.
func platformDefaultCachePathFor(goos, programData, appData string) string {
	if goos == "windows" {
		if dir := programData; dir != "" {
			return dir + "/PuppetLabs/facts/cache/cached_facts"
		}
		if dir := appData; dir != "" {
			return dir + "/PuppetLabs/facts/cache/cached_facts"
		}
	}
	return "/opt/puppetlabs/facts/cache/cached_facts"
}
