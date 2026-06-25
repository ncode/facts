package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// FormatOptions selects the presentation format for resolved facts.
type FormatOptions struct {
	JSON               bool
	YAML               bool
	HOCON              bool
	IncludeTypedDotted bool
	Colorize           bool
}

// Formatter renders resolved facts in one presentation format.
type Formatter interface {
	Name() string
	Format([]ResolvedFact) (string, error)
}

type formatterFunc struct {
	name   string
	format func([]ResolvedFact) (string, error)
}

func (f formatterFunc) Name() string { return f.name }

func (f formatterFunc) Format(facts []ResolvedFact) (string, error) {
	return f.format(facts)
}

// BuildFormatter selects a formatter using Ruby's formatter factory precedence.
func BuildFormatter(opts FormatOptions) Formatter {
	switch {
	case opts.JSON:
		return formatterFunc{name: "json", format: func(facts []ResolvedFact) (string, error) {
			return FormatJSONWithDottedFacts(facts, opts.IncludeTypedDotted)
		}}
	case opts.YAML:
		return formatterFunc{name: "yaml", format: func(facts []ResolvedFact) (string, error) {
			return FormatYAMLWithDottedFacts(facts, opts.IncludeTypedDotted), nil
		}}
	case opts.HOCON:
		return formatterFunc{name: "hocon", format: func(facts []ResolvedFact) (string, error) {
			return FormatHOCONWithDottedFacts(facts, opts.IncludeTypedDotted), nil
		}}
	default:
		return formatterFunc{name: "legacy", format: func(facts []ResolvedFact) (string, error) {
			return FormatLegacyColored(facts, opts.IncludeTypedDotted, opts.Colorize), nil
		}}
	}
}

// FormatJSON renders facts using Facter's JSON presentation contract.
func FormatJSON(facts []ResolvedFact) (string, error) {
	return FormatJSONWithDottedFacts(facts, false)
}

// FormatJSONWithDottedFacts renders JSON and optionally merges dotted custom and external facts.
func FormatJSONWithDottedFacts(facts []ResolvedFact, includeTypedDotted bool) (string, error) {
	projection := NewProjection(facts, includeTypedDotted)
	value := any(projection.MultiQueryValues())
	if projection.Shape() == ShapeFullTree {
		value = projection.FullTree()
	}

	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("format json: %w", err)
	}
	return string(out), nil
}

// FormatYAML renders facts using Facter's YAML presentation contract.
func FormatYAML(facts []ResolvedFact) string {
	return FormatYAMLWithDottedFacts(facts, false)
}

// FormatYAMLWithDottedFacts renders YAML and optionally merges dotted custom and external facts.
func FormatYAMLWithDottedFacts(facts []ResolvedFact, includeTypedDotted bool) string {
	projection := NewProjection(facts, includeTypedDotted)
	value := any(projection.MultiQueryValues())
	if projection.Shape() == ShapeFullTree {
		value = projection.FullTree()
	}
	out := strings.Join(yamlLines(value, 0), "\n")
	if out == "" {
		return ""
	}
	return out + "\n"
}

// FormatHOCON renders facts using Facter's HOCON presentation contract.
func FormatHOCON(facts []ResolvedFact) string {
	return FormatHOCONWithDottedFacts(facts, false)
}

// FormatHOCONWithDottedFacts renders HOCON and optionally merges dotted custom and external facts.
func FormatHOCONWithDottedFacts(facts []ResolvedFact, includeTypedDotted bool) string {
	projection := NewProjection(facts, includeTypedDotted)
	switch projection.Shape() {
	case ShapeEmpty:
		return ""
	case ShapeFullTree:
		return strings.Join(hoconLines(projection.FullTree(), 0, false), "\n") + "\n"
	case ShapeSingleQuery:
		return hoconScalar(projection.SingleQueryValue())
	default:
		values := projection.MultiQueryValues()
		lines := make([]string, 0, len(values))
		for _, key := range sortedKeys(values) {
			lines = append(lines, strconv.Quote(key)+"="+hoconScalar(values[key]))
		}
		return strings.Join(lines, "\n") + "\n"
	}
}

// FormatLegacy renders facts using the original key => value text format.
func FormatLegacy(facts []ResolvedFact) string {
	return FormatLegacyColored(facts, false, false)
}

// FormatLegacyColored renders legacy text and, when colorize is set, wraps each
// key in an ANSI color chosen by its nesting depth. The rendering replicates
// Ruby Facter's LegacyFactFormatter byte for byte: pretty-printed JSON rewritten
// through Ruby's exact transform pipeline, quirks included.
func FormatLegacyColored(facts []ResolvedFact, includeTypedDotted, colorize bool) string {
	projection := NewProjection(facts, includeTypedDotted)
	switch projection.Shape() {
	case ShapeEmpty:
		return ""
	case ShapeFullTree:
		return legacyCollectionText(projection.FullTree(), colorize)
	case ShapeSingleQuery:
		return legacySingleQueryText(projection.SingleQueryValue(), colorize)
	default:
		values := projection.MultiQueryValues()
		for key, value := range values {
			if value == nil {
				values[key] = ""
			}
		}
		out := legacyCollectionText(values, colorize)
		return legacyTopLevelStringRE.ReplaceAllString(out, "$1 => $2")
	}
}

func yamlLines(value any, depth int) []string {
	indent := strings.Repeat("  ", depth)
	switch v := value.(type) {
	case map[string]any:
		lines := make([]string, 0, len(v))
		for _, key := range sortedKeys(v) {
			child := v[key]
			if childMap, ok := child.(map[string]any); ok {
				lines = append(lines, indent+yamlKey(key)+":")
				lines = append(lines, yamlLines(childMap, depth+1)...)
				continue
			}
			if childSlice, ok := child.([]any); ok {
				lines = append(lines, indent+yamlKey(key)+":")
				lines = append(lines, yamlSequenceLines(childSlice, depth)...)
				continue
			}
			lines = append(lines, indent+yamlKey(key)+": "+yamlScalar(child))
		}
		return lines
	case []any:
		return yamlSequenceLines(v, depth)
	default:
		return []string{indent + yamlScalar(value)}
	}
}

func yamlSequenceLines(values []any, depth int) []string {
	indent := strings.Repeat("  ", depth)
	lines := make([]string, 0, len(values))
	for _, value := range values {
		if childMap, ok := value.(map[string]any); ok {
			lines = append(lines, indent+"- "+yamlSequenceMap(childMap))
			continue
		}
		lines = append(lines, indent+"- "+yamlScalar(value))
	}
	return lines
}

func hoconLines(m map[string]any, depth int, braces bool) []string {
	indent := strings.Repeat("    ", depth)
	capacity := len(m)
	if braces {
		capacity += 2
	}
	lines := make([]string, 0, capacity)
	if braces {
		lines = append(lines, indent+"{")
		depth++
		indent = strings.Repeat("    ", depth)
	}
	for _, key := range sortedKeys(m) {
		value := m[key]
		if child, ok := value.(map[string]any); ok {
			lines = append(lines, indent+hoconKey(key)+"={")
			lines = append(lines, hoconLines(child, depth+1, false)...)
			lines = append(lines, indent+"}")
			continue
		}
		lines = append(lines, indent+hoconKey(key)+"="+hoconScalar(value))
	}
	if braces {
		lines = append(lines, strings.Repeat("    ", depth-1)+"}")
	}
	return lines
}

func yamlKey(value string) string {
	if isPlainOutputKey(value, true) && isPlainYAMLKey(value) {
		return value
	}
	return quoteOutputString(value)
}

func hoconKey(value string) string {
	if isPlainOutputKey(value, false) {
		return value
	}
	return quoteOutputString(value)
}

func isPlainOutputKey(value string, allowDot bool) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			continue
		}
		if allowDot && r == '.' {
			continue
		}
		return false
	}
	return true
}

func isPlainYAMLKey(value string) bool {
	switch strings.ToLower(value) {
	case "true", "false", "null", "~", "yes", "no", "on", "off":
		return false
	}
	_, err := strconv.ParseFloat(value, 64)
	return err != nil
}

func quoteOutputString(value string) string {
	encoded, _ := json.Marshal(value) // json.Marshal cannot fail for a concrete string.
	return string(encoded)
}

func hoconScalar(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		if !isPlainHOCONString(v) {
			return strconv.Quote(v)
		}
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case map[string]any:
		return strings.Join(hoconLines(v, 0, true), "\n")
	case []any:
		return hoconArray(v)
	case []string:
		return hoconStringArray(v)
	case []int:
		return hoconIntArray(v)
	default:
		return fmt.Sprint(v)
	}
}

func hoconArray(values []any) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, hoconArrayScalar(value))
	}
	return "[" + strings.Join(items, ",") + "]"
}

func hoconStringArray(values []string) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.Quote(value))
	}
	return "[" + strings.Join(items, ",") + "]"
}

func hoconIntArray(values []int) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.Itoa(value))
	}
	return "[" + strings.Join(items, ",") + "]"
}

func hoconArrayScalar(value any) string {
	if s, ok := value.(string); ok {
		return strconv.Quote(s)
	}
	return hoconScalar(value)
}

func yamlScalar(value any) string {
	switch v := value.(type) {
	case nil:
		return strconv.Quote("")
	case string:
		if v == "true" || v == "false" {
			return "'" + v + "'"
		}
		if isPlainYAMLString(v) {
			return v
		}
		return strconv.Quote(v)
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case map[string]any:
		return yamlInlineMap(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, yamlScalar(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []string:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, yamlScalar(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []int:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, strconv.Itoa(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprint(v)
	}
}

func isPlainHOCONString(value string) bool {
	if value == "" || strings.Contains(value, "_") {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func yamlInlineMap(value map[string]any) string {
	return "{" + yamlInlineMapContents(value) + "}"
}

func yamlSequenceMap(value map[string]any) string {
	if len(value) == 1 {
		return yamlInlineMapContents(value)
	}
	return yamlInlineMap(value)
}

func yamlInlineMapContents(value map[string]any) string {
	parts := make([]string, 0, len(value))
	for _, key := range sortedKeys(value) {
		parts = append(parts, yamlKey(key)+": "+yamlScalar(value[key]))
	}
	return strings.Join(parts, ", ")
}

func isPlainYAMLString(value string) bool {
	if value == "" {
		return false
	}
	if needsQuotedYAMLString(value) {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == ' ' || r == '/' {
			continue
		}
		return false
	}
	return true
}

func needsQuotedYAMLString(value string) bool {
	if strings.ContainsAny(value, "yYnN:") {
		return true
	}
	for _, marker := range []string{"True", "TRUE", "False", "FALSE", "On", "ON", "Off", "OFF", "off"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func uniqueQueries(facts []ResolvedFact) []string {
	seen := make(map[string]bool, len(facts))
	queries := make([]string, 0, len(facts))
	for _, fact := range facts {
		if seen[fact.UserQuery] {
			continue
		}
		seen[fact.UserQuery] = true
		queries = append(queries, fact.UserQuery)
	}
	return queries
}

func factsForQueries(facts []ResolvedFact) map[string]any {
	values := make(map[string]any, len(facts))
	for _, fact := range facts {
		values[fact.UserQuery] = ValueForQuery(fact)
	}
	return values
}

// legacyKeyPalette cycles per nesting depth when key coloring is enabled:
// cyan, yellow, green, magenta, blue.
var legacyKeyPalette = [...]string{"\x1b[36m", "\x1b[33m", "\x1b[32m", "\x1b[35m", "\x1b[34m"}

const legacyKeyColorReset = "\x1b[0m"

// The regexes below transliterate Ruby Facter's LegacyFactFormatter gsub
// pipeline (lib/facter/framework/formatters/legacy_fact_formatter.rb).
var (
	// `":` followed by whitespace becomes `" => ` (Ruby /":\s/).
	legacyKeyDelimiterRE = regexp.MustCompile(`":\s`)
	// Greedy per-line key unquote (Ruby /"(.*)"\ =>/).
	legacyKeyQuoteRE = regexp.MustCompile(`"(.*)" =>`)
	// Empty-line removal after stripping the enclosing braces (Ruby /^$\n/).
	legacyEmptyLineRE = regexp.MustCompile(`(?m)^\n`)
	// Multi-query top-level string unquote (Ruby /^(\S*) => "(.*)"/).
	legacyTopLevelStringRE = regexp.MustCompile(`(?m)^(\S*) => "(.*)"`)
	// Single-query whole-result unquote (Ruby /^"(.*)"/).
	legacySingleQueryQuoteRE = regexp.MustCompile(`(?m)^"(.*)"`)
)

// legacyCollectionText renders the full-output and multi-query map through
// Ruby's format_for_no_query pipeline.
func legacyCollectionText(value map[string]any, colorize bool) string {
	pretty := hashToFacterFormat(value, colorize, -1)
	pretty = removeEnclosingAccolades(pretty)
	pretty = removeCommaAndQuotation(pretty)
	return handleNewlines(pretty)
}

// legacySingleQueryText renders one queried value through Ruby's
// format_for_single_user_query: plain strings print raw, nil prints nothing,
// and structures keep their enclosing braces, nested commas, and quotes.
func legacySingleQueryText(value any, colorize bool) string {
	if s, ok := value.(string); ok {
		return s
	}
	if value == nil {
		return ""
	}
	pretty := hashToFacterFormat(value, colorize, 0)
	return legacySingleQueryQuoteRE.ReplaceAllString(pretty, "$1")
}

// hashToFacterFormat is Ruby's hash_to_facter_format: 2-space-indented JSON,
// then in order `": ` => `" => `, per-line greedy key unquote, and doubled
// backslash collapse. depthShift adjusts the colored key depth for pipelines
// that de-indent the output afterwards.
func hashToFacterFormat(value any, colorize bool, depthShift int) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return fmt.Sprint(value)
	}
	pretty := strings.TrimSuffix(buf.String(), "\n")
	pretty = legacyKeyDelimiterRE.ReplaceAllString(pretty, `" => `)
	pretty = rewriteLegacyKeys(pretty, colorize, depthShift)
	return strings.ReplaceAll(pretty, `\\`, `\`)
}

// rewriteLegacyKeys strips the JSON quotes around keys (Ruby's greedy
// /"(.*)"\ =>/ per line) and, when colorize is set, wraps each key in the
// palette color for its nesting depth (indentation/2 plus depthShift).
func rewriteLegacyKeys(pretty string, colorize bool, depthShift int) string {
	if !colorize {
		return legacyKeyQuoteRE.ReplaceAllString(pretty, "$1 =>")
	}
	lines := strings.Split(pretty, "\n")
	for i, line := range lines {
		match := legacyKeyQuoteRE.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}
		depth := max(0, legacyIndentWidth(line)/2+depthShift)
		color := legacyKeyPalette[depth%len(legacyKeyPalette)]
		lines[i] = line[:match[0]] + color + line[match[2]:match[3]] + legacyKeyColorReset + " =>" + line[match[1]:]
	}
	return strings.Join(lines, "\n")
}

func legacyIndentWidth(line string) int {
	width := 0
	for width < len(line) && line[width] == ' ' {
		width++
	}
	return width
}

// removeEnclosingAccolades is Ruby's remove_enclosing_accolades: strip the
// first and last character (the enclosing braces), drop empty lines, de-indent
// every line by exactly two whitespace characters, and drop the comma after a
// top-level closing brace.
func removeEnclosingAccolades(pretty string) string {
	if len(pretty) < 2 {
		return ""
	}
	pretty = pretty[1 : len(pretty)-1]
	pretty = legacyEmptyLineRE.ReplaceAllString(pretty, "")
	lines := strings.Split(pretty, "\n")
	for i, line := range lines {
		if len(line) >= 2 && isLegacySpace(line[0]) && isLegacySpace(line[1]) {
			line = line[2:]
		}
		if strings.HasPrefix(line, "},") {
			line = "}" + line[2:]
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// removeCommaAndQuotation is Ruby's remove_comma_and_quotation: on lines that
// do not start with whitespace, strip the trailing comma and every unescaped
// double quote, then unescape the remaining quotes. Ruby's split("\n") drops
// trailing empty lines, so the result carries no trailing newline.
func removeCommaAndQuotation(pretty string) string {
	lines := strings.Split(pretty, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		if len(line) > 0 && isLegacySpace(line[0]) {
			continue
		}
		lines[i] = stripCommaAndUnescapedQuotes(line)
	}
	return strings.Join(lines, "\n")
}

// stripCommaAndUnescapedQuotes mirrors Ruby's
// gsub(/,$|(?<!\\)"/, ”).gsub('\\"', '"'): Go regexp has no lookbehind, so the
// quote stripping keeps quotes whose preceding byte in the original line is a
// backslash, then rewrites those `\"` sequences to bare quotes.
func stripCommaAndUnescapedQuotes(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == ',' && i == len(line)-1 {
			continue
		}
		if c == '"' && (i == 0 || line[i-1] != '\\') {
			continue
		}
		b.WriteByte(c)
	}
	return strings.ReplaceAll(b.String(), `\"`, `"`)
}

// handleNewlines is Ruby's handle_newlines: the literal two-character
// sequence \n becomes a real newline across the whole output.
func handleNewlines(pretty string) string {
	return strings.ReplaceAll(pretty, `\n`, "\n")
}

// isLegacySpace matches Ruby's regex \s class (ASCII whitespace).
func isLegacySpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}
