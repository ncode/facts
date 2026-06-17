package engine

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
)

var blocklistPattern = regexp.MustCompile(`(?is)blocklist\s*[:=]\s*\[(.*?)\]`)
var externalDirPattern = regexp.MustCompile(`(?is)(?:^|[\s{,])external-dir\s*[:=]\s*(\[(.*?)\]|"([^"]*)"|'([^']*)'|([^,\n\r}]+))`)
var debugPattern = regexp.MustCompile(`(?is)(?:^|[\s{,])debug\s*[:=]\s*(true|false)`)
var verbosePattern = regexp.MustCompile(`(?is)(?:^|[\s{,])verbose\s*[:=]\s*(true|false)`)
var logLevelPattern = regexp.MustCompile(`(?is)(?:^|[\s{,])log-level\s*[:=]\s*(?:"([^"]*)"|'([^']*)'|([A-Za-z_]+))`)
var noExternalFactsPattern = regexp.MustCompile(`(?is)(?:^|[\s{,])no-external-facts\s*[:=]\s*(true|false)`)
var forceDotResolutionPattern = regexp.MustCompile(`(?is)(?:^|[\s{,])force-dot-resolution\s*[:=]\s*(true|false)`)
var sequentialPattern = regexp.MustCompile(`(?is)(?:^|[\s{,])sequential\s*[:=]\s*(true|false)`)
var ttlsPattern = regexp.MustCompile(`(?is)(?:^|[\s{,])ttls\s*[:=]\s*\[(.*?)\]`)
var ttlEntryPattern = regexp.MustCompile(`(?is)\{\s*(?:"([^"]+)"|'([^']+)'|([A-Za-z0-9_.-]+))\s*[:=]\s*(?:"([^"]*)"|'([^']*)'|([^}]+?))\s*\}`)
var factGroupPattern = regexp.MustCompile(`(?is)(?:^|[\s{,])(?:"([^"]+)"|'([^']+)'|([A-Za-z0-9_-]+))\s*[:=]\s*(\[(.*?)\]|"([^"]*)"|'([^']*)'|([A-Za-z0-9_.-]+))`)
var configArrayValuePattern = regexp.MustCompile(`"([^"]*)"|'([^']*)'|([^,\]\s]+)`)

// NativeDefaultConfigPath returns the platform default facts-native
// facts.conf path, consulted before the facter-compatible default.
var NativeDefaultConfigPath = platformNativeDefaultConfigPath

// DefaultConfigPath returns the platform default facter-compatible
// facter.conf path, read when no facts-native config file exists.
var DefaultConfigPath = platformDefaultConfigPath

// Config contains the supported Facter config values loaded from a config file.
type Config struct {
	Blocklist          []string
	ExternalDirs       []string
	Debug              bool
	Verbose            bool
	LogLevel           string
	NoExternalFacts    bool
	ForceDotResolution bool
	Sequential         bool
	SequentialSet      bool
	TTLs               []FactTTL
	FactGroups         []FactGroup
}

// DefaultExternalFactDirs returns the default external fact directories:
// the facts-native locations first, then Ruby Facter's compatible locations.
// Facts found in both follow normal directory precedence.
func DefaultExternalFactDirs(windows, root bool, home, windowsDataDir string) []string {
	if windows {
		if windowsDataDir == "" {
			return nil
		}
		return []string{
			windowsDataDir + "/facts/facts.d",
			windowsDataDir + "/PuppetLabs/facter/facts.d",
		}
	}
	if root {
		return []string{
			"/etc/facts/facts.d",
			"/etc/puppetlabs/facter/facts.d",
			"/etc/facter/facts.d/",
			"/opt/puppetlabs/facter/facts.d",
		}
	}
	if home == "" {
		return nil
	}
	return []string{
		home + "/.facts/facts.d",
		home + "/.facter/facts.d",
		home + "/.puppetlabs/opt/facter/facts.d",
	}
}

// CurrentDefaultExternalFactDirs returns the default external fact
// directories (facts-native first, facter-compatible after) for the current
// process environment.
func CurrentDefaultExternalFactDirs() []string {
	return DefaultExternalFactDirs(
		runtime.GOOS == "windows",
		runtime.GOOS != "windows" && os.Geteuid() == 0,
		os.Getenv("HOME"),
		firstNonEmpty(os.Getenv("ProgramData"), os.Getenv("APPDATA")),
	)
}

// FactTTL is a configured cache duration for a fact or fact group.
type FactTTL struct {
	Fact string
	TTL  string
}

// ParseConfig returns every supported value from a Facter config file.
func ParseConfig(path string) (Config, error) {
	return readConfigOptionsFile(path)
}

func readConfigOptionsFile(path string) (Config, error) {
	if path == "" {
		path = readableDefaultConfigPath()
		if path == "" {
			return Config{}, nil
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		warn(fmt.Sprintf("Facts failed to read config file %s: %v", path, err))
		return Config{}, nil
	}
	content := stripConfigLineComments(string(data))
	if invalidConfigContent(content) {
		warn(fmt.Sprintf("Facts failed to read config file %s: invalid config", path))
		return Config{}, nil
	}
	cliSection := configSection(content, "cli")
	sequential, sequentialSet := configBoolValue(content, sequentialPattern)
	return Config{
		Blocklist:          lowerConfigValues(configList(content, blocklistPattern)),
		ExternalDirs:       configList(content, externalDirPattern),
		Debug:              configBool(cliSection, debugPattern),
		Verbose:            configBool(cliSection, verbosePattern),
		LogLevel:           configString(cliSection, logLevelPattern),
		NoExternalFacts:    configBool(content, noExternalFactsPattern),
		ForceDotResolution: configBool(content, forceDotResolutionPattern),
		Sequential:         sequential,
		SequentialSet:      sequentialSet,
		TTLs:               configTTLs(content),
		FactGroups:         configFactGroups(content),
	}, nil
}

func invalidConfigContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	return trimmed != "" && !strings.ContainsAny(trimmed, ":=")
}

func stripConfigLineComments(content string) string {
	var b strings.Builder
	inQuote := rune(0)
	escaped := false
	for i := 0; i < len(content); i++ {
		ch := rune(content[i])
		if inQuote != 0 {
			b.WriteByte(content[i])
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}

		if ch == '\'' || ch == '"' {
			inQuote = ch
			b.WriteByte(content[i])
			continue
		}
		if ch == '#' {
			i = skipConfigComment(content, i)
			if i < len(content) {
				b.WriteByte(content[i])
			}
			continue
		}
		if ch == '/' && i+1 < len(content) && content[i+1] == '/' {
			i = skipConfigComment(content, i)
			if i < len(content) {
				b.WriteByte(content[i])
			}
			continue
		}
		b.WriteByte(content[i])
	}
	return b.String()
}

func skipConfigComment(content string, start int) int {
	for i := start; i < len(content); i++ {
		if content[i] == '\n' {
			return i
		}
	}
	return len(content)
}

// readableDefaultConfigPath returns the first existing default config file:
// the facts-native facts.conf wins over the facter-compatible facter.conf.
// Both are parsed with identical semantics; an explicit path overrides both.
func readableDefaultConfigPath() string {
	for _, path := range []string{NativeDefaultConfigPath(), DefaultConfigPath()} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		return path
	}
	return ""
}

func platformNativeDefaultConfigPath() string {
	return platformNativeDefaultConfigPathFor(runtime.GOOS)
}

func platformNativeDefaultConfigPathFor(goos string) string {
	if goos == "windows" {
		return "C:/ProgramData/facts/facts.conf"
	}
	return "/etc/facts/facts.conf"
}

func platformDefaultConfigPath() string {
	return platformDefaultConfigPathFor(runtime.GOOS)
}

func platformDefaultConfigPathFor(goos string) string {
	if goos == "windows" {
		return "C:/ProgramData/PuppetLabs/facter/etc/facter.conf"
	}
	return "/etc/puppetlabs/facter/facter.conf"
}

func configList(content string, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	values := []string{}
	for _, match := range matches {
		value := match[1]
		if len(match) > 2 && match[2] != "" {
			value = match[2]
		}
		values = append(values, quotedConfigValues(value)...)
	}
	return values
}

func configTTLs(content string) []FactTTL {
	factsSection := configSection(content, "facts")
	matches := ttlsPattern.FindAllStringSubmatch(factsSection, -1)
	if len(matches) == 0 {
		return nil
	}
	ttls := []FactTTL{}
	for _, match := range matches {
		entries := ttlEntryPattern.FindAllStringSubmatch(match[1], -1)
		if len(entries) == 0 {
			continue
		}
		if ttls == nil {
			ttls = make([]FactTTL, 0, len(entries))
		}
		for _, entry := range entries {
			ttls = append(ttls, FactTTL{Fact: firstConfigValue(entry[1], entry[2], entry[3]), TTL: strings.TrimSpace(firstConfigValue(entry[4], entry[5], entry[6]))})
		}
	}
	return ttls
}

func configFactGroups(content string) []FactGroup {
	section := configSection(content, "fact-groups")
	matches := factGroupPattern.FindAllStringSubmatch(section, -1)
	if len(matches) == 0 {
		return nil
	}
	groups := make([]FactGroup, 0, len(matches))
	for _, match := range matches {
		facts := quotedConfigValues(match[5])
		if len(facts) == 0 {
			facts = []string{firstConfigValue(match[6], match[7], match[8])}
		}
		groups = append(groups, FactGroup{Name: firstConfigValue(match[1], match[2], match[3]), Facts: facts})
	}
	return groups
}

func quotedConfigValues(content string) []string {
	matches := configArrayValuePattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, firstConfigValue(match[1], match[2], match[3]))
	}
	return values
}

func configSection(content, name string) string {
	start := strings.Index(strings.ToLower(content), strings.ToLower(name))
	if start < 0 {
		return ""
	}
	open := strings.IndexByte(content[start:], '{')
	if open < 0 {
		return ""
	}
	open += start
	depth := 0
	for i := open; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[open+1 : i]
			}
		}
	}
	return content[open+1:]
}

func lowerConfigValues(values []string) []string {
	for i, value := range values {
		values[i] = strings.ToLower(value)
	}
	return values
}

func configBool(content string, pattern *regexp.Regexp) bool {
	matches := pattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 && strings.EqualFold(match[1], "true") {
			return true
		}
	}
	return false
}

func configBoolValue(content string, pattern *regexp.Regexp) (bool, bool) {
	matches := pattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return false, false
	}
	last := matches[len(matches)-1]
	if len(last) < 2 {
		return false, false
	}
	return strings.EqualFold(last[1], "true"), true
}

func configString(content string, pattern *regexp.Regexp) string {
	matches := pattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		value := firstConfigValue(match[1:]...)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstConfigValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
