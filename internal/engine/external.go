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
	"time"

	"gopkg.in/yaml.v3"
)

// ErrNullByte reports an external fact name or value containing a NUL byte.
var ErrNullByte = errors.New("external fact contains a null byte reference")

// ErrExternalFactTooLarge reports an external fact source exceeding the byte cap.
var ErrExternalFactTooLarge = errors.New("external fact exceeds size limit")

const externalFactResolutionEnv = "FACTER_EXTERNAL_FACTS_RUNNING"

var externalFactCommandTimeout = 30 * time.Second
var externalFactMaxBytes = 1 << 20

type externalFactLoaderMode int

const (
	externalFactLoaderCLI externalFactLoaderMode = iota
	externalFactLoaderLibrary
)

type externalFactLoaderHost interface {
	readDir(string) ([]os.DirEntry, error)
	open(string) (io.ReadCloser, error)
	fileReadable(string) bool
	environ() []string
	goos() string
	runCommand(context.Context, string, ...string) ([]byte, []byte, error)
	externalFactResolutionRunning() bool
}

type externalFactOSHost struct{}

func (externalFactOSHost) readDir(dir string) ([]os.DirEntry, error) {
	return os.ReadDir(dir)
}

func (externalFactOSHost) open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (externalFactOSHost) fileReadable(path string) bool {
	return fileReadable(path)
}

func (externalFactOSHost) environ() []string {
	return os.Environ()
}

func (externalFactOSHost) goos() string {
	return runtime.GOOS
}

func (externalFactOSHost) runCommand(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	return runExternalFactCommand(ctx, name, args...)
}

func (externalFactOSHost) externalFactResolutionRunning() bool {
	return ExternalFactResolutionRunning()
}

type externalFactLoader struct {
	s          *Session
	mode       externalFactLoaderMode
	dirs       []string
	blocked    map[string]bool
	host       externalFactLoaderHost
	includeEnv bool
}

func (l externalFactLoader) withDefaults() externalFactLoader {
	if l.s == nil {
		l.s = NewSession()
	}
	if l.host == nil {
		l.host = externalFactOSHost{}
	}
	return l
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
	return externalFactLoader{
		s:          s,
		mode:       externalFactLoaderCLI,
		dirs:       dirs,
		blocked:    blocked,
		includeEnv: true,
	}.load()
}

func (l externalFactLoader) load() ([]ResolvedFact, error) {
	l = l.withDefaults()
	facts, failures, err := l.loadDirFacts()
	if err != nil {
		return nil, err
	}
	if l.includeEnv {
		envFacts, err := loadExternalEnvFacts(l.host.environ())
		if err != nil {
			if l.mode == externalFactLoaderCLI {
				return nil, err
			}
			failures = append(failures, err)
		} else {
			facts = append(facts, envFacts...)
		}
	}
	if l.mode == externalFactLoaderLibrary {
		return facts, errors.Join(failures...)
	}
	return facts, nil
}

func (l externalFactLoader) loadDirFacts() ([]ResolvedFact, []error, error) {
	facts := []ResolvedFact{}
	var failures []error
	for _, dir := range l.dirs {
		entries, err := l.host.readDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			err = fmt.Errorf("read external dir %s: %w", dir, err)
			if l.mode == externalFactLoaderCLI {
				return nil, nil, err
			}
			failures = append(failures, err)
			continue
		}
		slices.SortFunc(entries, func(a, b os.DirEntry) int {
			return strings.Compare(b.Name(), a.Name())
		})
		for _, entry := range entries {
			if l.blocked[entry.Name()] {
				l.s.debug(fmt.Sprintf("External fact file %s blocked.", entry.Name()))
				continue
			}
			if entry.IsDir() {
				continue
			}
			if ignoredBackupExternalFactFile(entry.Name()) {
				l.s.debug(fmt.Sprintf("External fact file %s ignored: %s extension.", entry.Name(), strings.ToLower(filepath.Ext(entry.Name()))))
				continue
			}
			if ignoredExternalFactFile(entry.Name()) {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				err = fmt.Errorf("stat external fact %s: %w", path, err)
				if l.mode == externalFactLoaderCLI {
					return nil, nil, err
				}
				failures = append(failures, err)
				continue
			}
			loaded, err := l.loadExternalFactFile(path, info.Mode())
			if err != nil {
				if l.mode == externalFactLoaderCLI {
					if errors.Is(err, errExternalFactExec) || l.s.Context().Err() != nil && errors.Is(err, l.s.Context().Err()) {
						continue
					}
					return nil, nil, err
				}
				failures = append(failures, err)
				continue
			}
			facts = append(facts, loaded...)
		}
	}
	return facts, failures, nil
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

func (l externalFactLoader) loadExternalFactFile(path string, mode os.FileMode) ([]ResolvedFact, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt":
		return l.loadExternalTxtFacts(path)
	case ".json":
		return l.loadExternalJSONFacts(path)
	case ".yaml", ".yml":
		return l.loadExternalYAMLFacts(path)
	case ".rb":
		l.s.warn(fmt.Sprintf("Ruby fact files are not supported by the Go port; skipping %s. Rewrite it as an executable external fact (see docs/CUSTOM_FACT_MIGRATION.md).", path))
		return nil, nil
	case ".ps1":
		if l.host.goos() == "windows" {
			return l.loadExternalPowerShellFacts(path)
		}
		if mode.IsRegular() && mode&0o111 != 0 && !l.host.externalFactResolutionRunning() {
			return l.loadExternalExecutableFacts(path)
		}
		return nil, nil
	default:
		if l.host.goos() != "windows" && windowsExecutableExternalFactExt(ext) {
			return nil, nil
		}
		if l.host.goos() == "windows" && windowsExecutableExternalFactExt(ext) && mode.IsRegular() && !l.host.externalFactResolutionRunning() {
			return l.loadExternalExecutableFacts(path)
		}
		if mode.IsRegular() && mode&0o111 != 0 && !l.host.externalFactResolutionRunning() {
			return l.loadExternalExecutableFacts(path)
		}
		if mode.IsRegular() {
			reportEmptyStructuredExternalFact(l.s, path)
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

func (l externalFactLoader) loadExternalTxtFacts(path string) ([]ResolvedFact, error) {
	data, err := l.readExternalFactFile(path)
	if err != nil {
		if errors.Is(err, ErrExternalFactTooLarge) {
			return nil, err
		}
		return nil, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
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

func (l externalFactLoader) loadExternalExecutableFacts(path string) ([]ResolvedFact, error) {
	return l.loadExternalCommandFacts(externalExecutableCommandName(path), path)
}

func externalExecutableCommandName(path string) string {
	if strings.ContainsAny(path, " \t") {
		return strconv.Quote(path)
	}
	return path
}

func (l externalFactLoader) loadExternalPowerShellFacts(path string) ([]ResolvedFact, error) {
	powershell := currentPowerShellPath(systemRootFromEnv(l.host.environ()), l.host.fileReadable)
	return l.loadExternalCommandFacts(
		powershell,
		`"`+powershell+`" -NoProfile -NonInteractive -NoLogo -ExecutionPolicy Bypass -File "`+path+`"`,
		"-NoProfile", "-NonInteractive", "-NoLogo", "-ExecutionPolicy", "Bypass", "-File", path,
	)
}

func systemRootFromEnv(env []string) string {
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(name, "SYSTEMROOT") {
			return value
		}
	}
	return ""
}

func currentPowerShellPath(systemRoot string, readable func(string) bool) string {
	if systemRoot != "" {
		sysnative := systemRoot + `\sysnative\WindowsPowershell\v1.0\powershell.exe`
		if readable(sysnative) {
			return sysnative
		}
		system32 := systemRoot + `\system32\WindowsPowershell\v1.0\powershell.exe`
		if readable(system32) {
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

func (l externalFactLoader) loadExternalCommandFacts(name, warningName string, args ...string) ([]ResolvedFact, error) {
	ctx, cancel := context.WithTimeout(l.s.Context(), externalFactCommandTimeout)
	defer cancel()

	out, stderr, err := l.host.runCommand(ctx, name, args...)
	if parentErr := l.s.Context().Err(); parentErr != nil {
		return nil, fmt.Errorf("execute external fact %s: %w", warningName, parentErr)
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w: %s: %v", errExternalFactExec, warningName, ctx.Err())
	}
	if errors.Is(err, ErrExternalFactTooLarge) {
		return nil, fmt.Errorf("%w: %s: %w", errExternalFactExec, warningName, err)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", errExternalFactExec, warningName, err)
	}
	if message := strings.TrimSpace(string(stderr)); message != "" {
		l.s.warn(fmt.Sprintf("Command %s completed with the following stderr message: %s", warningName, message))
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
	var stdout, stderr limitedBuffer
	stdout.limit = externalFactMaxBytes
	stderr.limit = externalFactMaxBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if stdout.tooLarge || stderr.tooLarge {
		err = ErrExternalFactTooLarge
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

func (l externalFactLoader) loadExternalJSONFacts(path string) ([]ResolvedFact, error) {
	data, err := l.readExternalFactFile(path)
	if err != nil {
		if errors.Is(err, ErrExternalFactTooLarge) {
			return nil, err
		}
		return nil, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, nil
	}

	values, ok := value.(map[string]any)
	if !ok {
		reportStructuredExternalFactWithoutKeyValueData(l.s, path)
		return nil, nil
	}
	if len(values) == 0 {
		reportEmptyStructuredExternalFact(l.s, path)
		return nil, nil
	}
	return externalFactsFromMap(values)
}

func (l externalFactLoader) loadExternalYAMLFacts(path string) ([]ResolvedFact, error) {
	data, err := l.readExternalFactFile(path)
	if err != nil {
		if errors.Is(err, ErrExternalFactTooLarge) {
			return nil, err
		}
		return nil, nil
	}

	var value any
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&value); err != nil {
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
		reportStructuredExternalFactWithoutKeyValueData(l.s, path)
		return nil, nil
	}
	if len(values) == 0 {
		reportEmptyStructuredExternalFact(l.s, path)
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

func (l externalFactLoader) readExternalFactFile(path string) ([]byte, error) {
	file, err := l.host.open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := readExternalFactData(file)
	if err != nil {
		return nil, fmt.Errorf("read external fact %s: %w", path, err)
	}
	return data, nil
}

func readExternalFactData(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(externalFactMaxBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > externalFactMaxBytes {
		return nil, ErrExternalFactTooLarge
	}
	return data, nil
}

type limitedBuffer struct {
	buf      bytes.Buffer
	limit    int
	tooLarge bool
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		b.tooLarge = true
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) < remaining {
			remaining = len(p)
		}
		_, _ = b.buf.Write(p[:remaining])
	}
	if len(p) > remaining {
		b.tooLarge = true
	}
	return len(p), nil
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
