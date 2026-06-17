package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/ncode/facts/internal/engine"
)

func TestMain(m *testing.M) {
	defaultExternalFactDirs = func() []string { return nil }
	os.Exit(m.Run())
}

func TestRun_version(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_shortVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"-v"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_facterversionQueryAllowsExternalOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "version.txt"), []byte("facterversion=external\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(stdout.String()), "external"; got != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_acceptsCompatibilityFlagsAndConfigToggles(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "cli : {\n  trace : true,\n}\nglobal : {\n  sequential : false,\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"--http-debug", "--sequential", "facterversion"},
		{"--config", configPath, "facterversion"},
		{"--log-level", "trace", "facterversion"},
	} {
		var stdout, stderr bytes.Buffer
		if err := Run(&stdout, &stderr, args); err != nil {
			t.Fatalf("Run(%v) err = %v, want compatibility options accepted", args, err)
		}
		if got, want := stdout.String(), engine.Version+"\n"; got != want {
			t.Fatalf("Run(%v) stdout = %q, want %q", args, got, want)
		}
	}
}

func TestRun_helpListsSupportedCompatibilityOptions(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[--color]",
		"-c [--config]",
		"-d [--debug]",
		"[--external-dir]",
		"[--no-external-facts]",
		"[--strict]",
		"-t [--timing]",
		"--version, -v",
		"--list-block-groups",
		"--list-cache-groups",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help output missing %q:\n%s", want, stdout.String())
		}
	}
	for _, removed := range []string{"--custom-dir", "--no-custom-facts", "--no-ruby", "--show-legacy", "--no-show-legacy"} {
		if strings.Contains(stdout.String(), removed) {
			t.Fatalf("help output lists removed option %q:\n%s", removed, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_manPrintsManual(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--man"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"facts - collect and display facts about the current system",
		"SYNOPSIS",
		"OPTIONS",
		"--json",
		"--version, -v",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("manual output missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_queryJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--json", "os.name"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["os.name"] == "" {
		t.Fatalf("stdout = %q, want non-empty os.name", stdout.String())
	}
}

func TestRun_warnsAndIgnoresUnreadableConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.conf")
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != engine.Version {
		t.Fatalf("stdout = %q, want facterversion", stdout.String())
	}
	warningCount := strings.Count(stderr.String(), "WARN Facts - Facts failed to read config file")
	if warningCount != 1 {
		t.Fatalf("stderr = %q, want one config read warning, got %d", stderr.String(), warningCount)
	}
}

func TestRun_noQueryJSONReturnsFullFactMap(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--json"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	for _, key := range []string{"facterversion", "os", "processors"} {
		if got[key] == nil {
			t.Fatalf("stdout = %q, want %q root fact", stdout.String(), key)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_configBlocklistSkipsExternalFactFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data.yaml"), []byte("site_location: lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "owner.yaml"), []byte("site_owner: platform\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "facts : {\n  blocklist : [ \"data.yaml\" ],\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--external-dir", dir, "--json", "site_location", "site_owner"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["site_location"] != nil {
		t.Fatalf("stdout = %q, want data.yaml fact blocklisted", stdout.String())
	}
	if got["site_owner"] != "platform" {
		t.Fatalf("stdout = %q, want owner.yaml fact preserved", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_configBlocklistExpandsConfiguredFactGroup(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), []byte("site_role: web\nsite_owner: platform\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := `facts : {
  blocklist : [ "site" ],
}
fact-groups : {
  site : [ "site_role" ],
}
`
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--external-dir", dir, "--json", "site_role", "site_owner"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["site_role"] != nil {
		t.Fatalf("stdout = %q, want configured group to block site_role", stdout.String())
	}
	if got["site_owner"] != "platform" {
		t.Fatalf("stdout = %q, want site_owner preserved", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_rejectsInvalidOptionPairsFromConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := `global : {
  external-dir : [ "/facts" ],
  no-external-facts : true,
}
`
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--config", configPath, "facterversion"})
	if err == nil {
		t.Fatal("Run() err = nil, want configured option conflict")
	}
	if got, want := err.Error(), "--no-external-facts and --external-dir options conflict: please specify only one"; got != want {
		t.Fatalf("Run() err = %q, want %q", got, want)
	}
	assertUsageOutput(t, stdout.String())
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_externalExecutableStderrIsWrittenAsWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exercises POSIX execute-bit and shebang semantics; covered on the POSIX platform gates")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "site_fact")
	content := "#!/bin/sh\nprintf 'script_one=two\\n'\nprintf 'some error' >&2\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "script_one"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "two\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	wantWarning := "WARN Facts - Command " + path + " completed with the following stderr message: some error\n"
	if got := stderr.String(); got != wantWarning {
		t.Fatalf("stderr = %q, want %q", got, wantWarning)
	}
}

func TestRun_colorOutputsYellowWarnings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exercises POSIX execute-bit and shebang semantics; covered on the POSIX platform gates")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "site_fact")
	content := "#!/bin/sh\nprintf 'script_one=two\\n'\nprintf 'some error' >&2\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--color", "--external-dir", dir, "script_one"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "two\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	wantWarning := "\x1b[33mWARN Facts - Command " + path + " completed with the following stderr message: some error\x1b[0m\n"
	if got := stderr.String(); got != wantWarning {
		t.Fatalf("stderr = %q, want %q", got, wantWarning)
	}
}

func TestRun_loadsDefaultConfigWhenConfigFlagIsOmitted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("site_location=lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "global : {\n  external-dir : [ \"" + dir + "\" ],\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	oldDefaultConfigPath := engine.DefaultConfigPath
	engine.DefaultConfigPath = func() string { return configPath }
	t.Cleanup(func() { engine.DefaultConfigPath = oldDefaultConfigPath })
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"site_location"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "lab\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_configCliDebugEmitsDebugLogs(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "cli : {\n  debug : true,\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "DEBUG Facts - resolving facts") {
		t.Fatalf("stderr = %q, want configured debug log", stderr.String())
	}
}

func TestRun_configCliVerboseEmitsInfoLogs(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "cli : {\n  verbose : true,\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if !strings.Contains(stderr.String(), "INFO Facts - executed with command line: --config "+configPath+" facterversion") {
		t.Fatalf("stderr = %q, want configured verbose command line INFO output", stderr.String())
	}
	if !strings.Contains(stderr.String(), "INFO Facts - resolving facts") {
		t.Fatalf("stderr = %q, want configured verbose resolving INFO output", stderr.String())
	}
}

func TestRun_configCliLogLevelDebugEmitsDebugLogs(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "cli : {\n  log-level : debug,\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if !strings.Contains(stderr.String(), "DEBUG Facts - resolving facts") {
		t.Fatalf("stderr = %q, want configured log-level debug output", stderr.String())
	}
}

func TestRun_rejectsUnsupportedConfiguredLogLevel(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "cli : {\n  log-level : loud,\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--config", configPath, "facterversion"})
	if err == nil {
		t.Fatal("Run() err = nil, want unsupported configured log-level error")
	}
	if got, want := err.Error(), "unsupported log level loud"; got != want {
		t.Fatalf("Run() err = %q, want %q", got, want)
	}
	assertUsageOutput(t, stdout.String())
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_rejectsConflictingConfiguredLogOptions(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "cli : {\n  debug : true,\n  log-level : info,\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--config", configPath, "facterversion"})
	if err == nil {
		t.Fatal("Run() err = nil, want configured log option conflict")
	}
	if got, want := err.Error(), "debug, verbose, and log-level options conflict: please specify only one."; got != want {
		t.Fatalf("Run() err = %q, want %q", got, want)
	}
	assertUsageOutput(t, stdout.String())
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_cliExternalDirOverridesConfiguredExternalDir(t *testing.T) {
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "site.txt"), []byte("site_location=config\nconfig_only=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cliDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cliDir, "site.txt"), []byte("site_location=cli\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "global : {\n  external-dir : [ \"" + configDir + "\" ],\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--external-dir", cliDir, "--json", "site_location", "config_only"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["site_location"] != "cli" {
		t.Fatalf("stdout = %q, want CLI external-dir fact", stdout.String())
	}
	if got["config_only"] != nil {
		t.Fatalf("stdout = %q, want configured external-dir overridden", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_configTTLsUseFreshCachedFact(t *testing.T) {
	dir := t.TempDir()
	factPath := filepath.Join(dir, "site.txt")
	if err := os.WriteFile(factPath, []byte("site_role=web\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "facts : {\n  ttls : [ { \"site_role\" : \"1 hour\" } ],\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	oldDefaultCachePath := engine.DefaultCachePath
	engine.DefaultCachePath = func() string { return cacheDir }
	t.Cleanup(func() { engine.DefaultCachePath = oldDefaultCachePath })

	var stdout, stderr bytes.Buffer
	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--external-dir", dir, "site_role"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "web\n"; got != want {
		t.Fatalf("first stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("first stderr = %q, want empty", stderr.String())
	}
	if err := os.WriteFile(factPath, []byte("site_role=db\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--external-dir", dir, "site_role"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "web\n"; got != want {
		t.Fatalf("second stdout = %q, want cached %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("second stderr = %q, want empty", stderr.String())
	}
}

func TestRun_strictLogsMissingFactErrorWhenQueriedFactIsMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--strict", "--json", "os.name", "missing_fact"})
	status, ok := err.(ExitStatus)
	if !ok {
		t.Fatalf("Run() err = %T %[1]v, want ExitStatus", err)
	}
	if status.Code() != 1 {
		t.Fatalf("Run() status = %d, want 1", status.Code())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["os.name"] == nil {
		t.Fatalf("stdout = %q, want resolved os.name", stdout.String())
	}
	if value, ok := got["missing_fact"]; !ok || value != nil {
		t.Fatalf("stdout = %q, want missing_fact null", stdout.String())
	}
	if got, want := stderr.String(), "ERROR Facts - fact \"missing_fact\" does not exist.\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRun_queryNoJSONUsesLegacyOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--no-json", "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_timingPrintsResolutionDuration(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--timing", "facterversion"}); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout = %q, want timing line and fact value", stdout.String())
	}
	timingLine := regexp.MustCompile(`^fact 'facterversion', took: \([0-9]+\.[0-9]{3}\) seconds$`)
	if !timingLine.MatchString(lines[0]) {
		t.Fatalf("timing line = %q, want Ruby-compatible timing message", lines[0])
	}
	if lines[1] != engine.Version {
		t.Fatalf("fact output = %q, want %q", lines[1], engine.Version)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_rejectsRemovedCustomFactOptions(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		option string
	}{
		{name: "custom dir", args: []string{"--custom-dir", t.TempDir(), "facterversion"}, option: "--custom-dir"},
		{name: "no ruby", args: []string{"--no-ruby", "facterversion"}, option: "--no-ruby"},
		{name: "no custom facts", args: []string{"--no-custom-facts", "facterversion"}, option: "--no-custom-facts"},
		{name: "trace", args: []string{"--trace", "facterversion"}, option: "--trace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run(&stdout, &stderr, tt.args)
			if err == nil {
				t.Fatalf("Run(%v) err = nil, want unknown option error", tt.args)
			}
			if got, want := err.Error(), "unrecognised option '"+tt.option+"'"; got != want {
				t.Fatalf("Run(%v) err = %q, want %q", tt.args, got, want)
			}
			assertUsageOutput(t, stdout.String())
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRun_rejectsRemovedPuppetOptions(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		option string
	}{
		{name: "puppet", args: []string{"--puppet", "facterversion"}, option: "--puppet"},
		{name: "puppet short", args: []string{"-p", "facterversion"}, option: "-p"},
		{name: "no puppet", args: []string{"--no-puppet", "facterversion"}, option: "--no-puppet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run(&stdout, &stderr, tt.args)
			if err == nil {
				t.Fatalf("Run(%v) err = nil, want unknown option error", tt.args)
			}
			if got, want := err.Error(), "unrecognised option '"+tt.option+"'"; got != want {
				t.Fatalf("Run(%v) err = %q, want %q", tt.args, got, want)
			}
			assertUsageOutput(t, stdout.String())
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRun_queryYAML(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--yaml", "os.name"}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); !strings.HasPrefix(got, "os.name: ") || strings.TrimSpace(strings.TrimPrefix(got, "os.name: ")) == "" {
		t.Fatalf("stdout = %q, want non-empty os.name YAML", got)
	}
}

func TestRun_queryHOCON(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--hocon", "os.name"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got == "" {
		t.Fatalf("stdout = %q, want non-empty os.name HOCON", stdout.String())
	}
}

func TestRun_queryLegacy(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRun_acceptsLogLevelCompatibilityFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--log-level", "none", "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_acceptsLogLevelPlaceholderCompatibilityValue(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--log-level", "log_level", "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_logLevelDebugOutputsDebugLogs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "debug", args: []string{"--log-level", "debug", "facterversion"}},
		{name: "trace", args: []string{"--log-level=trace", "facterversion"}},
		{name: "debug with trace", args: []string{"--debug", "--log-level=trace", "facterversion"}},
		{name: "short", args: []string{"-l", "debug", "facterversion"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if err := Run(&stdout, &stderr, tt.args); err != nil {
				t.Fatal(err)
			}
			if got, want := stdout.String(), engine.Version+"\n"; got != want {
				t.Fatalf("stdout = %q, want %q", got, want)
			}
			if !strings.Contains(stderr.String(), "DEBUG Facts - resolving facts") {
				t.Fatalf("stderr = %q, want DEBUG output", stderr.String())
			}
		})
	}
}

func TestRun_debugColorOutputsColorizedDebugLogs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows terminals do not reliably emit ANSI color sequences")
	}
	t.Setenv("TERM", "xterm-256color")
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--debug", "--color", "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if !strings.Contains(stderr.String(), "DEBUG Facts - resolving facts") {
		t.Fatalf("stderr = %q, want DEBUG output", stderr.String())
	}
	if !strings.Contains(stderr.String(), "\x1b[") {
		t.Fatalf("stderr = %q, want ANSI color sequence", stderr.String())
	}
}

func TestRun_verboseOutputsInfoLogs(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--verbose", "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if !strings.Contains(stderr.String(), "INFO Facts - executed with command line: --verbose facterversion") {
		t.Fatalf("stderr = %q, want executed command line INFO output", stderr.String())
	}
	if !strings.Contains(stderr.String(), "INFO Facts - resolving facts") {
		t.Fatalf("stderr = %q, want INFO output", stderr.String())
	}
}

func TestRun_verboseColorOutputsGreenInfoLogs(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--verbose", "--color", "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if !strings.Contains(stderr.String(), "\x1b[32mINFO Facts - resolving facts\x1b[0m") {
		t.Fatalf("stderr = %q, want green INFO output", stderr.String())
	}
}

func TestRun_acceptsNoOpCompatibilityFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "color", args: []string{"--color", "facterversion"}},
		{name: "no color", args: []string{"--no-color", "facterversion"}},
		{name: "no block", args: []string{"--no-block", "facterversion"}},
		{name: "no cache", args: []string{"--no-cache", "facterversion"}},
		{name: "sequential", args: []string{"--sequential", "facterversion"}},
		{name: "http debug", args: []string{"--http-debug", "facterversion"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if err := Run(&stdout, &stderr, tt.args); err != nil {
				t.Fatal(err)
			}
			if got, want := stdout.String(), engine.Version+"\n"; got != want {
				t.Fatalf("stdout = %q, want %q", got, want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRun_queryStructuredRootFactJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--json", "os"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["os"] == nil {
		t.Fatalf("stdout = %q, want os object", stdout.String())
	}
	for _, key := range []string{"architecture", "family", "name"} {
		if got["os"][key] == nil {
			t.Fatalf("os fact = %#v, want key %q", got["os"], key)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

// colorTreeExternalDir provides a deterministic structured fact tree for the
// depth-coloring contract tests.
func colorTreeExternalDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	content := "colortree:\n  inner:\n    leaf: value\nsimple: plain\n"
	if err := os.WriteFile(filepath.Join(dir, "colortree.yaml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRun_colorColorsDefaultFormatKeysByDepth(t *testing.T) {
	dir := colorTreeExternalDir(t)
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--color", "--external-dir", dir, "colortree", "simple"}); err != nil {
		t.Fatal(err)
	}
	want := "\x1b[36mcolortree\x1b[0m => {\n" +
		"  \x1b[33minner\x1b[0m => {\n" +
		"    \x1b[32mleaf\x1b[0m => \"value\"\n" +
		"  }\n" +
		"}\n" +
		"\x1b[36msimple\x1b[0m => plain\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_defaultFormatHasNoANSIWithoutColor(t *testing.T) {
	dir := colorTreeExternalDir(t)

	for _, args := range [][]string{
		{"--external-dir", dir, "colortree", "simple"},
		{"--no-color", "--external-dir", dir, "colortree", "simple"},
	} {
		var stdout, stderr bytes.Buffer
		if err := Run(&stdout, &stderr, args); err != nil {
			t.Fatalf("Run(%v) err = %v", args, err)
		}
		if strings.Contains(stdout.String(), "\x1b[") {
			t.Fatalf("Run(%v) stdout = %q, want no ANSI escape sequences", args, stdout.String())
		}
	}
}

func TestRun_machineFormatsAreByteIdenticalWithAndWithoutColor(t *testing.T) {
	dir := colorTreeExternalDir(t)

	for _, format := range []string{"--json", "--yaml", "--hocon"} {
		t.Run(format, func(t *testing.T) {
			var plain, colored bytes.Buffer
			var stderr bytes.Buffer
			if err := Run(&plain, &stderr, []string{format, "--external-dir", dir, "colortree", "simple"}); err != nil {
				t.Fatal(err)
			}
			if err := Run(&colored, &stderr, []string{format, "--color", "--external-dir", dir, "colortree", "simple"}); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(plain.Bytes(), colored.Bytes()) {
				t.Fatalf("%s output with --color = %q, want byte-identical to %q", format, colored.String(), plain.String())
			}
			if strings.Contains(colored.String(), "\x1b[") {
				t.Fatalf("%s output = %q, want no ANSI escape sequences", format, colored.String())
			}
		})
	}
}

func TestRun_queryExternalTxtFact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.txt")
	if err := os.WriteFile(path, []byte("site_location=lab\nowner=platform=team\nskip_me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "owner"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "platform=team\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func defaultExternalFactDirForTest(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "facts.d")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := defaultExternalFactDirs
	defaultExternalFactDirs = func() []string { return []string{dir} }
	t.Cleanup(func() { defaultExternalFactDirs = old })
	return dir
}

func TestRun_queryDefaultExternalFactDirectory(t *testing.T) {
	dir := defaultExternalFactDirForTest(t)
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("site_location=default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"site_location"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "default\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_externalDirOverridesDefaultExternalFactDirectory(t *testing.T) {
	defaultDir := defaultExternalFactDirForTest(t)
	if err := os.WriteFile(filepath.Join(defaultDir, "site.txt"), []byte("site_location=default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cliDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cliDir, "site.txt"), []byte("site_location=cli\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", cliDir, "site_location"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "cli\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_externalFactOverridesCoreFactInFullJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.txt")
	if err := os.WriteFile(path, []byte("os.name=ExternalOS\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "--json"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	osFact, ok := got["os"].(map[string]any)
	if !ok {
		t.Fatalf("os = %#v, want structured fact", got["os"])
	}
	if osFact["name"] != "ExternalOS" {
		t.Fatalf("os.name = %#v, want external fact override", osFact["name"])
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_queryExternalEnvironmentFact(t *testing.T) {
	t.Setenv("FACTER_site_location", "lab")
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"site_location"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "lab\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_queryExecutableExternalFact(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exercises POSIX execute-bit and shebang semantics; covered on the POSIX platform gates")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "dynamic_fact")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'dynamic_owner=platform\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "dynamic_owner"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "platform\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_queryExternalYAMLArrayIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.yaml")
	if err := os.WriteFile(path, []byte("arr_ext_fact:\n  - ex1\n  - ex2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "arr_ext_fact.0"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "ex1\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_warnsAndSkipsExecutableExternalFactsDuringRecursiveResolution(t *testing.T) {
	t.Setenv("FACTER_EXTERNAL_FACTS_RUNNING", "1")
	dir := t.TempDir()
	executablePath := filepath.Join(dir, "dynamic_fact")
	if err := os.WriteFile(executablePath, []byte("#!/bin/sh\nprintf 'dynamic=true\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	staticPath := filepath.Join(dir, "site.txt")
	if err := os.WriteFile(staticPath, []byte("static=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "--json"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["dynamic"] != nil {
		t.Fatalf("stdout = %q, want executable external fact skipped", stdout.String())
	}
	if got["static"] != "true" {
		t.Fatalf("stdout = %q, want static external fact", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Recursion detected") {
		t.Fatalf("stderr = %q, want recursion warning", stderr.String())
	}
}

func TestRun_rejectsNoExternalFactsWithExternalDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.txt")
	if err := os.WriteFile(path, []byte("owner=platform\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--external-dir", dir, "--no-external-facts", "owner"})
	if err == nil {
		t.Fatal("Run() err = nil, want conflicting option error")
	}
	if !strings.Contains(err.Error(), "--no-external-facts and --external-dir options conflict") {
		t.Fatalf("Run() err = %q, want external-dir conflict", err)
	}
	assertUsageOutput(t, stdout.String())
}

func TestRun_rejectsNoExternalFactsWithExternalDirEquals(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--external-dir=" + dir, "--no-external-facts", "owner"})
	if err == nil {
		t.Fatal("Run() err = nil, want conflicting option error")
	}
	if !strings.Contains(err.Error(), "--no-external-facts and --external-dir options conflict") {
		t.Fatalf("Run() err = %q, want external-dir conflict", err)
	}
	assertUsageOutput(t, stdout.String())
}

func TestRun_rejectsConflictingOutputFormats(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--json", "--yaml", "os.name"})
	if err == nil {
		t.Fatal("Run() err = nil, want conflicting option error")
	}
	if !strings.Contains(err.Error(), "--json and --yaml options conflict") {
		t.Fatalf("Run() err = %q, want output format conflict", err)
	}
	assertUsageOutput(t, stdout.String())
}

func TestRun_rejectsConflictingHOCONOutputToggle(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--hocon", "--no-hocon", "os.name"})
	if err == nil {
		t.Fatal("Run() err = nil, want conflicting option error")
	}
	if !strings.Contains(err.Error(), "--hocon and --no-hocon options conflict") {
		t.Fatalf("Run() err = %q, want hocon conflict", err)
	}
	assertUsageOutput(t, stdout.String())
}

func TestRun_noQueryPrintsKnownCoreFacts(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"facterversion => " + engine.Version, "os => {", "name => \"" + runtimeOSName() + "\""} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func runtimeOSName() string {
	switch runtime.GOOS {
	case "darwin":
		return "Darwin"
	case "linux":
		collection := engine.Collection(engine.CoreFacts(engine.NewSession()))
		osFact, _ := collection["os"].(map[string]any)
		if name, ok := osFact["name"].(string); ok && name != "" {
			return name
		}
		return "Linux"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}

func TestRun_rejectsRemovedShowLegacyOptions(t *testing.T) {
	for _, option := range []string{"--show-legacy", "--no-show-legacy"} {
		t.Run(option, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run(&stdout, &stderr, []string{option})
			if err == nil {
				t.Fatalf("Run(%s) err = nil, want unknown option error", option)
			}
			if got, want := err.Error(), "unrecognised option '"+option+"'"; got != want {
				t.Fatalf("Run(%s) err = %q, want %q", option, got, want)
			}
			assertUsageOutput(t, stdout.String())
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRun_configShowLegacyKeyIsInert(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := `global : {
  show-legacy : true,
}
`
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--json"}); err != nil {
		t.Fatalf("Run() err = %v, want retired show-legacy key ignored", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	for _, key := range []string{"uptime", "uptime_days", "uptime_hours", "uptime_seconds", "operatingsystem"} {
		if _, ok := got[key]; ok {
			t.Fatalf("stdout = %q, want no legacy fact %q despite show-legacy config key", stdout.String(), key)
		}
	}
	if _, ok := got["facterversion"]; !ok {
		t.Fatalf("stdout = %q, want core facts", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_configBlocklistLegacyIsInertInDefaultOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "facter.conf")
	conf := "facts : {\n  blocklist : [ \"legacy\" ],\n}\n"
	if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var withBlocklist, stderr bytes.Buffer

	if err := Run(&withBlocklist, &stderr, []string{"-c", path, "--json"}); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(withBlocklist.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", withBlocklist.String(), err)
	}
	for _, key := range []string{"facterversion", "os", "path", "processors", "system_uptime"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("stdout = %q, want core fact %q despite inert legacy blocklist", withBlocklist.String(), key)
		}
	}
}

func TestRun_configAllowsNoExternalFactsWithEmptyExternalDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "facter.conf")
	conf := "global : {\n  external-dir : \"\"\n  no-external-facts : true,\n}\n"
	if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", path, "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(stdout.String()), engine.Version; got != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_configForceDotResolutionAllowsPartialDottedExternalFactQuery(t *testing.T) {
	externalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalDir, "site.txt"), []byte("a.b.c=external\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "global : {\n  external-dir : [ \"" + externalDir + "\" ],\n  force-dot-resolution : true,\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--json", "a.b"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["a.b"]["c"] != "external" {
		t.Fatalf("stdout = %q, want partial dotted external fact", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_forceDotResolutionAllowsPartialDottedExternalFactQuery(t *testing.T) {
	externalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalDir, "site.txt"), []byte("a.b.c=external\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", externalDir, "--force-dot-resolution", "--json", "a.b"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["a.b"]["c"] != "external" {
		t.Fatalf("stdout = %q, want partial dotted external fact", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_forceDotResolutionMergesDottedExternalFactWithoutQuery(t *testing.T) {
	externalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalDir, "site.txt"), []byte("os.name=external_os\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", externalDir, "--force-dot-resolution", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	osFact, ok := got["os"].(map[string]any)
	if !ok {
		t.Fatalf("stdout = %q, want structured os fact", stdout.String())
	}
	if osFact["name"] != "external_os" {
		t.Fatalf("os.name = %#v, want dotted external fact merged", osFact["name"])
	}
	if osFact["family"] == nil {
		t.Fatalf("os = %#v, want core os fields preserved", osFact)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_configBlocklistGroupSuppressesGroupFacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "facter.conf")
	conf := "facts : {\n  blocklist : [ \"networking\" ],\n}\n"
	if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", path, "--json"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	for _, key := range []string{"networking", "fqdn", "hostname", "ipaddress"} {
		if _, ok := got[key]; ok {
			t.Fatalf("stdout = %q, want %q blocked", stdout.String(), key)
		}
	}
	for _, key := range []string{"facterversion", "os", "processors"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("stdout = %q, want core fact %q", stdout.String(), key)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_noBlockIgnoresConfiguredBlocklist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "facter.conf")
	conf := "facts : {\n  blocklist : [ \"networking\" ],\n}\n"
	if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", path, "--no-block", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	networking, ok := got["networking"].(map[string]any)
	if !ok {
		t.Fatalf("stdout = %q, want networking present with --no-block", stdout.String())
	}
	for _, key := range []string{"hostname", "fqdn"} {
		if _, ok := networking[key]; !ok {
			t.Fatalf("stdout = %q, want networking.%s present with --no-block", stdout.String(), key)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_concatenatedShortJSONAndDebugFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"-jd", "os.name"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["os.name"] == "" {
		t.Fatalf("stdout = %q, want non-empty os.name", stdout.String())
	}
	if !strings.Contains(stderr.String(), "DEBUG") {
		t.Fatalf("stderr = %q, want DEBUG output", stderr.String())
	}
}

func TestRun_concatenatedShortTimingAndDebugFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"-td", "os.name"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "fact 'os.name', took: ") {
		t.Fatalf("stdout = %q, want timing output", stdout.String())
	}
	if !strings.Contains(stderr.String(), "DEBUG") {
		t.Fatalf("stderr = %q, want DEBUG output", stderr.String())
	}
}

func TestRun_concatenatedShortJSONDebugAndTimingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"-jdt", "os.name"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "\"os.name\"") {
		t.Fatalf("stdout = %q, want JSON os.name", stdout.String())
	}
	if !strings.Contains(stdout.String(), "fact 'os.name', took: ") {
		t.Fatalf("stdout = %q, want timing output", stdout.String())
	}
	if !strings.Contains(stderr.String(), "DEBUG") {
		t.Fatalf("stderr = %q, want DEBUG output", stderr.String())
	}
}

func TestRun_concatenatedShortFlagsRejectUnknownOption(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"-jdtz"})
	if err == nil {
		t.Fatal("Run() err = nil, want unknown option")
	}
	if !strings.Contains(err.Error(), "unrecognised option '-z'") {
		t.Fatalf("Run() err = %q, want unknown -z option", err)
	}
	assertUsageOutput(t, stdout.String())
}

func TestRun_helpPrintsUsage(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if err := Run(&stdout, &stderr, []string{arg}); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"Usage", "facts [options] [query]", "--list-block-groups"} {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
				}
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func assertUsageOutput(t *testing.T, got string) {
	t.Helper()
	for _, want := range []string{"Usage", "facts [options] [query]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
}

func TestRun_listBlockGroupsPrintsFactGroups(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--list-block-groups"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"networking", "- hostname", "operating system", "- os"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_listCacheGroupsPrintsFactGroups(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--list-cache-groups"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"networking", "- hostname", "processor", "- processors"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_listCacheGroupsIncludesConfiguredFactGroups(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	content := `fact-groups : {
  cached-custom-facts : [ "site_role", "site_location" ],
}`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--list-cache-groups"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"cached-custom-facts", "- site_role", "- site_location"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_listCacheGroupsAcceptsShortConfigEquals(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	content := `fact-groups : {
  cached-custom-facts : [ "site_role" ],
}`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"-c=" + configPath, "--list-cache-groups"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "cached-custom-facts\n- site_role") {
		t.Fatalf("stdout = %q, want configured group from -c= path", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_listCacheGroupsIncludesExternalDirectoryEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), []byte("site_role: web\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ignored.yaml"), []byte("ignored: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "--list-cache-groups"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"site.yaml\n", ".ignored.yaml\n", "nested\n"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want external directory entry group %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_listCacheGroupsConfiguredGroupOverridesDefault(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	content := `fact-groups : {
  memory : [ "custom_memory_fact" ],
}`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--list-cache-groups"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "memory\n- custom_memory_fact") {
		t.Fatalf("stdout = %q, want configured memory group", stdout.String())
	}
	if strings.Contains(stdout.String(), "- memoryfree") {
		t.Fatalf("stdout = %q, want configured memory group to replace default memory group", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func BenchmarkRunJSONSingleFact(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var stdout, stderr bytes.Buffer
		if err := Run(&stdout, &stderr, []string{"--no-cache", "--json", "facterversion"}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunLegacySingleFact(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var stdout, stderr bytes.Buffer
		if err := Run(&stdout, &stderr, []string{"--no-cache", "facterversion"}); err != nil {
			b.Fatal(err)
		}
	}
}
