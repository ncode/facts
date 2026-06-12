package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrNullByte reports an external fact name or value containing a NUL byte.
var ErrNullByte = errors.New("external fact contains a null byte reference")

const externalFactResolutionEnv = "FACTER_EXTERNAL_FACTS_RUNNING"

var externalFactCommandTimeout = 30 * time.Second
var externalFactGOOS = runtime.GOOS
var externalFactRunCommand = runExternalFactCommand
var externalFactFileReadable = fileReadable
var externalFactOpen = os.Open

var diagnosticState struct {
	mu             sync.Mutex
	debugHandler   func(string)
	warningHandler func(string)
	errorHandler   func(string)
}

// SetDebugHandler registers a process-wide debug callback for internal diagnostics.
func SetDebugHandler(handler func(string)) {
	diagnosticState.mu.Lock()
	defer diagnosticState.mu.Unlock()
	diagnosticState.debugHandler = handler
}

// SetWarningHandler registers a process-wide warning callback for internal diagnostics.
func SetWarningHandler(handler func(string)) {
	diagnosticState.mu.Lock()
	defer diagnosticState.mu.Unlock()
	diagnosticState.warningHandler = handler
}

// SetErrorHandler registers a process-wide error callback for internal diagnostics.
func SetErrorHandler(handler func(string)) {
	diagnosticState.mu.Lock()
	defer diagnosticState.mu.Unlock()
	diagnosticState.errorHandler = handler
}

func warn(message string) {
	diagnosticState.mu.Lock()
	handler := diagnosticState.warningHandler
	diagnosticState.mu.Unlock()
	if handler != nil {
		handler(message)
	}
}

func debug(message string) {
	diagnosticState.mu.Lock()
	handler := diagnosticState.debugHandler
	diagnosticState.mu.Unlock()
	if handler != nil {
		handler(message)
	}
}

func reportError(message string) {
	diagnosticState.mu.Lock()
	handler := diagnosticState.errorHandler
	diagnosticState.mu.Unlock()
	if handler != nil {
		handler(message)
	}
}

// ExternalFactResolutionRunning reports whether Facts is already resolving
// executable external facts in this process tree.
func ExternalFactResolutionRunning() bool {
	return os.Getenv(externalFactResolutionEnv) != ""
}

// LoadExternalFacts loads static external facts from the provided directories.
func LoadExternalFacts(s *Session, dirs []string) ([]ResolvedFact, error) {
	return LoadExternalFactsWithBlocklist(s, dirs, nil)
}

// LoadExternalFactsWithBlocklist loads external facts from dirs plus the
// FACTS_*/FACTER_* environment variables — the CLI's system-following
// semantics — skipping files whose base name is blocklisted by the Facter
// config.
func LoadExternalFactsWithBlocklist(s *Session, dirs []string, blocked map[string]bool) ([]ResolvedFact, error) {
	facts, err := loadExternalDirFacts(s, dirs, blocked, nil)
	if err != nil {
		return nil, err
	}
	envFacts, err := loadExternalEnvFacts(os.Environ())
	if err != nil {
		return nil, err
	}
	facts = append(facts, envFacts...)
	return facts, nil
}

// LoadExternalFactsFromDirs loads external facts from exactly the given
// directories — no environment variables — returning every fact that loaded
// together with the per-source failures joined. Library engines use this to
// keep opted-in sources hermetic and discovery failures partial.
func LoadExternalFactsFromDirs(s *Session, dirs []string, blocked map[string]bool) ([]ResolvedFact, error) {
	var failures []error
	facts, err := loadExternalDirFacts(s, dirs, blocked, &failures)
	if err != nil {
		failures = append(failures, err)
	}
	return facts, errors.Join(failures...)
}

// loadExternalDirFacts walks dirs loading external fact files. With failures
// nil it keeps the CLI contract: executable and cancelled-context failures are
// skipped silently and the first hard error aborts the load. With failures
// non-nil every failure is collected and loading continues, returning partial
// results.
func loadExternalDirFacts(s *Session, dirs []string, blocked map[string]bool, failures *[]error) ([]ResolvedFact, error) {
	facts := []ResolvedFact{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			err = fmt.Errorf("read external dir %s: %w", dir, err)
			if failures == nil {
				return nil, err
			}
			*failures = append(*failures, err)
			continue
		}
		slices.SortFunc(entries, func(a, b os.DirEntry) int {
			return strings.Compare(b.Name(), a.Name())
		})
		for _, entry := range entries {
			if blocked[entry.Name()] {
				s.debug(fmt.Sprintf("External fact file %s blocked.", entry.Name()))
				continue
			}
			if entry.IsDir() {
				continue
			}
			if ignoredBackupExternalFactFile(entry.Name()) {
				s.debug(fmt.Sprintf("External fact file %s ignored: %s extension.", entry.Name(), strings.ToLower(filepath.Ext(entry.Name()))))
				continue
			}
			if ignoredExternalFactFile(entry.Name()) {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				err = fmt.Errorf("stat external fact %s: %w", path, err)
				if failures == nil {
					return nil, err
				}
				*failures = append(*failures, err)
				continue
			}
			loaded, err := loadExternalFactFile(s, path, info.Mode())
			if err != nil {
				if failures == nil {
					if errors.Is(err, errExternalFactExec) || s.Context().Err() != nil && errors.Is(err, s.Context().Err()) {
						continue
					}
					return nil, err
				}
				*failures = append(*failures, err)
				continue
			}
			facts = append(facts, loaded...)
		}
	}
	return facts, nil
}

func loadExternalEnvFacts(env []string) ([]ResolvedFact, error) {
	values := make(map[string]string)
	nativeNames := make(map[string]bool)
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if name == externalFactResolutionEnv {
			continue
		}
		factName, native, ok := environmentFactName(name)
		if !ok {
			continue
		}
		factName = strings.ToLower(factName)
		if err := validateExternalString(factName); err != nil {
			return nil, fmt.Errorf("external fact name %q: %w", factName, err)
		}
		if err := validateExternalString(value); err != nil {
			return nil, fmt.Errorf("external fact %s value: %w", factName, err)
		}
		// Facts-native FACTS_* variables win name collisions with the
		// facter-compatible FACTER_* variables, regardless of env order.
		if nativeNames[factName] && !native {
			continue
		}
		values[factName] = value
		if native {
			nativeNames[factName] = true
		}
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	facts := make([]ResolvedFact, 0, len(keys))
	for _, key := range keys {
		facts = append(facts, ResolvedFact{Name: key, Value: values[key], Type: "external"})
	}
	return facts, nil
}

// environmentFactName maps an environment variable name to an external fact
// name. The facts-native FACTS_* prefix and the facter-compatible FACTER_*
// prefix are accepted with identical semantics; native reports whether the
// variable used the facts-native prefix.
func environmentFactName(name string) (factName string, native, ok bool) {
	lowerName := strings.ToLower(name)
	prefixes := []struct {
		prefix string
		native bool
	}{
		{prefix: "facts_", native: true},
		{prefix: "facts", native: true},
		{prefix: "facter_", native: false},
		{prefix: "facter", native: false},
	}
	for _, p := range prefixes {
		if rest, ok := strings.CutPrefix(lowerName, p.prefix); ok {
			return name[len(name)-len(rest):], p.native, isRubyWord(rest)
		}
	}
	return "", false, false
}

func isRubyWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' {
			continue
		}
		return false
	}
	return true
}

func ignoredExternalFactFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return ignoredBackupExternalFactFile(name)
}

func ignoredBackupExternalFactFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".bak", ".orig":
		return true
	default:
		return false
	}
}

// ExternalFactGroups returns fact group entries for loadable external fact files.
func ExternalFactGroups(dirs []string) ([]FactGroup, error) {
	groups := []FactGroup{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read external dir %s: %w", dir, err)
		}
		for _, entry := range entries {
			groups = append(groups, FactGroup{Name: entry.Name()})
		}
	}
	slices.SortFunc(groups, func(a, b FactGroup) int {
		return strings.Compare(a.Name, b.Name)
	})
	return groups, nil
}

func loadExternalFactFile(s *Session, path string, mode os.FileMode) ([]ResolvedFact, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt":
		return loadExternalTxtFacts(path)
	case ".json":
		return loadExternalJSONFacts(s, path)
	case ".yaml", ".yml":
		return loadExternalYAMLFacts(s, path)
	case ".rb":
		s.warn(fmt.Sprintf("Ruby fact files are not supported by the Go port; skipping %s. Rewrite it as an executable external fact (see docs/CUSTOM_FACT_MIGRATION.md).", path))
		return nil, nil
	case ".ps1":
		if externalFactGOOS == "windows" {
			return loadExternalPowerShellFacts(s, path)
		}
		if mode.IsRegular() && mode&0o111 != 0 && !ExternalFactResolutionRunning() {
			return loadExternalExecutableFacts(s, path)
		}
		return nil, nil
	default:
		if externalFactGOOS != "windows" && windowsExecutableExternalFactExt(ext) {
			return nil, nil
		}
		if externalFactGOOS == "windows" && windowsExecutableExternalFactExt(ext) && mode.IsRegular() && !ExternalFactResolutionRunning() {
			return loadExternalExecutableFacts(s, path)
		}
		if mode.IsRegular() && mode&0o111 != 0 && !ExternalFactResolutionRunning() {
			return loadExternalExecutableFacts(s, path)
		}
		if mode.IsRegular() {
			reportEmptyStructuredExternalFact(s, path)
		}
		return nil, nil
	}
}

func windowsExecutableExternalFactExt(ext string) bool {
	switch ext {
	case ".bat", ".cmd", ".exe", ".com":
		return true
	default:
		return false
	}
}

func loadExternalTxtFacts(path string) ([]ResolvedFact, error) {
	file, err := externalFactOpen(path)
	if err != nil {
		return nil, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	facts, err := parseKeyValueFacts(scanner)
	if err != nil {
		return nil, fmt.Errorf("scan external fact %s: %w", path, err)
	}
	return facts, nil
}

func parseKeyValueFacts(scanner *bufio.Scanner) ([]ResolvedFact, error) {
	facts := []ResolvedFact{}
	for scanner.Scan() {
		name, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok || value == "" {
			continue
		}
		name = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "\ufeff"))
		if name == "" {
			continue
		}
		if err := validateExternalString(name); err != nil {
			return nil, fmt.Errorf("external fact name %q: %w", name, err)
		}
		if err := validateExternalString(value); err != nil {
			return nil, fmt.Errorf("external fact %s value: %w", name, err)
		}
		facts = append(facts, ResolvedFact{Name: name, Value: value, Type: "external"})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return facts, nil
}

func loadExternalExecutableFacts(s *Session, path string) ([]ResolvedFact, error) {
	return loadExternalCommandFacts(s, externalExecutableCommandName(path), path)
}

func externalExecutableCommandName(path string) string {
	if strings.ContainsAny(path, " \t") {
		return strconv.Quote(path)
	}
	return path
}

func loadExternalPowerShellFacts(s *Session, path string) ([]ResolvedFact, error) {
	powershell := currentPowerShellPath(os.Getenv("SYSTEMROOT"))
	return loadExternalCommandFacts(
		s,
		powershell,
		`"`+powershell+`" -NoProfile -NonInteractive -NoLogo -ExecutionPolicy Bypass -File "`+path+`"`,
		"-NoProfile", "-NonInteractive", "-NoLogo", "-ExecutionPolicy", "Bypass", "-File", path,
	)
}

func currentPowerShellPath(systemRoot string) string {
	if systemRoot != "" {
		sysnative := systemRoot + `\sysnative\WindowsPowershell\v1.0\powershell.exe`
		if externalFactFileReadable(sysnative) {
			return sysnative
		}
		system32 := systemRoot + `\system32\WindowsPowershell\v1.0\powershell.exe`
		if externalFactFileReadable(system32) {
			return system32
		}
	}
	return "powershell.exe"
}

func fileReadable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// errExternalFactExec marks an executable external fact that failed to run;
// the CLI path skips these silently while engines surface them as discovery
// failures.
var errExternalFactExec = errors.New("executable external fact failed")

func loadExternalCommandFacts(s *Session, name, warningName string, args ...string) ([]ResolvedFact, error) {
	ctx, cancel := context.WithTimeout(s.Context(), externalFactCommandTimeout)
	defer cancel()

	out, stderr, err := externalFactRunCommand(ctx, name, args...)
	if parentErr := s.Context().Err(); parentErr != nil {
		return nil, fmt.Errorf("execute external fact %s: %w", warningName, parentErr)
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w: %s: %v", errExternalFactExec, warningName, ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", errExternalFactExec, warningName, err)
	}
	if message := strings.TrimSpace(string(stderr)); message != "" {
		s.warn(fmt.Sprintf("Command %s completed with the following stderr message: %s", warningName, message))
	}
	if len(out) == 0 {
		return nil, nil
	}

	var values map[string]any
	if err := yaml.NewDecoder(bytes.NewReader(out)).Decode(&values); err == nil && len(values) > 0 {
		return externalExecutableYAMLFactsFromMap(values)
	} else if yamlErrorIsNullByte(err) {
		return nil, fmt.Errorf("decode executable external fact %s: %w", warningName, ErrNullByte)
	}
	facts, err := parseKeyValueFacts(bufio.NewScanner(bytes.NewReader(out)))
	if err != nil {
		return nil, fmt.Errorf("parse executable external fact %s: %w", warningName, err)
	}
	return facts, nil
}

func runExternalFactCommand(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmdName := name
	if unquoted, err := strconv.Unquote(name); err == nil {
		cmdName = unquoted
	}
	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Env = append(os.Environ(), externalFactResolutionEnv+"=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	return out, stderr.Bytes(), err
}

func loadExternalJSONFacts(s *Session, path string) ([]ResolvedFact, error) {
	file, err := externalFactOpen(path)
	if err != nil {
		return nil, nil
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, nil
	}

	values, ok := value.(map[string]any)
	if !ok {
		reportStructuredExternalFactWithoutKeyValueData(s, path)
		return nil, nil
	}
	if len(values) == 0 {
		reportEmptyStructuredExternalFact(s, path)
		return nil, nil
	}
	return externalFactsFromMap(values)
}

func loadExternalYAMLFacts(s *Session, path string) ([]ResolvedFact, error) {
	file, err := externalFactOpen(path)
	if err != nil {
		return nil, nil
	}
	defer file.Close()

	var value any
	if err := yaml.NewDecoder(file).Decode(&value); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if yamlErrorIsNullByte(err) {
			return nil, fmt.Errorf("decode external fact %s: %w", path, ErrNullByte)
		}
		return nil, nil
	}
	values, ok := value.(map[string]any)
	if !ok {
		reportStructuredExternalFactWithoutKeyValueData(s, path)
		return nil, nil
	}
	if len(values) == 0 {
		reportEmptyStructuredExternalFact(s, path)
		return nil, nil
	}
	return externalYAMLFactsFromMap(values)
}

func reportEmptyStructuredExternalFact(s *Session, path string) {
	s.debug(fmt.Sprintf("Structured data fact file %s was parsed but was either empty or an invalid filetype (valid filetypes are .yaml, .json, and .txt).", path))
}

func reportStructuredExternalFactWithoutKeyValueData(s *Session, path string) {
	s.reportError(fmt.Sprintf("Structured data fact file %s was parsed but no key=>value data was returned.", path))
}

func externalYAMLFactsFromMap(values map[string]any) ([]ResolvedFact, error) {
	return externalFactsFromMap(normalizeYAMLSymbolMap(values))
}

func externalExecutableYAMLFactsFromMap(values map[string]any) ([]ResolvedFact, error) {
	return externalFactsFromMap(normalizeExecutableYAMLValue(normalizeYAMLSymbolMap(values)).(map[string]any))
}

func normalizeExecutableYAMLValue(value any) any {
	switch v := value.(type) {
	case string:
		if parsed, ok := parseExecutableYAMLTime(v); ok {
			return parsed.Format(time.RFC3339)
		}
		return v
	case []any:
		for i := range v {
			v[i] = normalizeExecutableYAMLValue(v[i])
		}
		return v
	case map[string]any:
		for key := range v {
			v[key] = normalizeExecutableYAMLValue(v[key])
		}
		return v
	default:
		return v
	}
}

func parseExecutableYAMLTime(value string) (time.Time, bool) {
	layouts := []string{
		"2006-01-02 15:04:05.999999999 -07:00",
		"2006-01-02 15:04:05 -07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02T15:04:05-07:00",
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999 Z",
		"2006-01-02 15:04:05 Z",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func normalizeYAMLSymbolMap(values map[string]any) map[string]any {
	normalized := make(map[string]any, len(values))
	for key, value := range values {
		normalized[trimYAMLSymbol(key)] = normalizeYAMLSymbolValue(value)
	}
	return normalized
}

func normalizeYAMLSymbolValue(value any) any {
	switch v := value.(type) {
	case string:
		return trimYAMLSymbol(v)
	case []any:
		for i := range v {
			v[i] = normalizeYAMLSymbolValue(v[i])
		}
		return v
	case map[string]any:
		return normalizeYAMLSymbolMap(v)
	default:
		return value
	}
}

func trimYAMLSymbol(value string) string {
	trimmed, ok := strings.CutPrefix(value, ":")
	if !ok || trimmed == "" {
		return value
	}
	return trimmed
}

func externalFactsFromMap(values map[string]any) ([]ResolvedFact, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	facts := make([]ResolvedFact, 0, len(values))
	for _, key := range keys {
		name := strings.ToLower(key)
		if err := validateExternalString(name); err != nil {
			return nil, fmt.Errorf("external fact name %q: %w", name, err)
		}
		value, err := normalizeStructuredValue(values[key])
		if err != nil {
			return nil, fmt.Errorf("external fact %s value: %w", name, err)
		}
		facts = append(facts, ResolvedFact{Name: name, Value: value, Type: "external"})
	}
	return facts, nil
}

func validateExternalString(value string) error {
	if strings.ContainsRune(value, '\x00') {
		return ErrNullByte
	}
	return nil
}

func yamlErrorIsNullByte(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "control") || strings.Contains(message, "#x0000")
}

func normalizeStructuredValue(value any) (any, error) {
	switch v := value.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), nil
		}
		if f, err := v.Float64(); err == nil {
			return f, nil
		}
		return v.String(), nil
	case string:
		if err := validateExternalString(v); err != nil {
			return nil, err
		}
		return v, nil
	case []any:
		for i := range v {
			value, err := normalizeStructuredValue(v[i])
			if err != nil {
				return nil, err
			}
			v[i] = value
		}
		return v, nil
	case map[string]any:
		for key := range v {
			if err := validateExternalString(key); err != nil {
				return nil, err
			}
			value, err := normalizeStructuredValue(v[key])
			if err != nil {
				return nil, err
			}
			v[key] = value
		}
		return v, nil
	case time.Time:
		if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 && v.Nanosecond() == 0 {
			return v, nil
		}
		return v.Format("2006-01-02 15:04:05.000000000"), nil
	default:
		return value, nil
	}
}
