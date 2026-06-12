package app

// Output-contract scenarios migrated from the root package's facter_test.go
// ahead of the Ruby-compat API removal (openspec change
// introduce-facts-library-api, task 1.2). Each test pins current `facter` CLI
// behavior through Run; see
// openspec/changes/introduce-facts-library-api/test-migration-map.md for the
// scenario-by-scenario mapping. Where the deleted Resolve(string) API behaved
// differently from the CLI, the CLI behavior is the surviving contract and the
// divergence is noted on the test.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Legacy aliases are removed entirely: an explicitly queried alias behaves
// exactly like any other missing fact — empty output and exit 0, or a
// missing-fact error under --strict (openspec change remove-legacy-facts).
func TestRun_queriedLegacyAliasIsAnOrdinaryMissingFact(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"operatingsystem"}); err != nil {
		t.Fatalf("Run(operatingsystem) err = %v, want nil", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty for removed legacy alias", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_strictQueriedLegacyAliasIsMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--strict", "operatingsystem"})
	status, ok := err.(ExitStatus)
	if !ok {
		t.Fatalf("Run() err = %T %[1]v, want ExitStatus", err)
	}
	if status.Code() != 1 {
		t.Fatalf("exit status = %d, want 1", status.Code())
	}
	if !strings.Contains(stderr.String(), `fact "operatingsystem" does not exist.`) {
		t.Fatalf("stderr = %q, want missing-fact error for operatingsystem", stderr.String())
	}
}

func TestRun_queriedNilExternalFactRendersJSONNull(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nil.yaml"), []byte("nil_fact: null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "--json", "nil_fact"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if value, ok := got["nil_fact"]; !ok || value != nil {
		t.Fatalf("stdout = %q, want nil_fact null", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

// At the CLI boundary a fact that resolves to nil is reported missing under
// strict mode, matching Ruby's nil-resolution-means-no-value semantics.
func TestRun_strictQueriedNilExternalFactIsMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nil.yaml"), []byte("nil_fact: null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--strict", "--external-dir", dir, "--json", "nil_fact"})
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
	if value, ok := got["nil_fact"]; !ok || value != nil {
		t.Fatalf("stdout = %q, want nil_fact null", stdout.String())
	}
	if got, want := stderr.String(), "ERROR Facts - fact \"nil_fact\" does not exist.\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRun_invalidExternalArrayIndexQueriesPrintNothing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "arr.yaml"), []byte("arr_fact:\n  - x\n  - y\n  - z\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, query := range []string{"arr_fact.3", "arr_fact.abc", "arr_fact.-1"} {
		var stdout, stderr bytes.Buffer
		if err := Run(&stdout, &stderr, []string{"--external-dir", dir, query}); err != nil {
			t.Fatalf("Run(%s) err = %v", query, err)
		}
		if got := stdout.String(); got != "" {
			t.Fatalf("Run(%s) stdout = %q, want empty", query, got)
		}
		if stderr.Len() != 0 {
			t.Fatalf("Run(%s) stderr = %q, want empty", query, stderr.String())
		}
	}
}

func TestRun_partialDottedExternalFactQueryRequiresForceDotResolution(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dotted.txt"), []byte("a.b.c=custom\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "a.b"}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty output with default dot resolution", got)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "--force-dot-resolution", "a.b"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "{\n  c => \"custom\"\n}\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

// The deleted Resolve(string) API permuted options found after positional
// queries; the CLI does not: flag parsing stops at the first query and later
// option-like tokens are treated as fact queries.
func TestRun_trailingOptionTokensAreTreatedAsQueries(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--json", "os.name", "--timing", "kernel"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["os.name"] == nil {
		t.Fatalf("stdout = %q, want resolved os.name", stdout.String())
	}
	if got["kernel"] == nil {
		t.Fatalf("stdout = %q, want resolved kernel", stdout.String())
	}
	if value, ok := got["--timing"]; !ok || value != nil {
		t.Fatalf("stdout = %q, want trailing --timing treated as missing-fact query", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_configExternalDirLoadsExternalFacts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("site_location=lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "global : {\n  external-dir : [ \"" + dir + "\" ],\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "site_location"}); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "lab\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

// The retired custom-fact config keys load like any other unrecognized key:
// no error, no warning, no effect (ADR-0006).
func TestRun_retiredCustomFactConfigKeysAreInert(t *testing.T) {
	dir := t.TempDir()
	content := []byte("Facter.add(:site_role) do\n  setcode do\n    'web'\n  end\nend\n")
	if err := os.WriteFile(filepath.Join(dir, "site.rb"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "global : {\n  custom-dir : [ \"" + dir + "\" ],\n  no-custom-facts : true,\n  no-ruby : true,\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "site_role"}); err != nil {
		t.Fatalf("Run() err = %v, want retired config keys ignored", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want configured custom-dir to have no effect", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_noExternalFactsSkipsEnvironmentExternalFacts(t *testing.T) {
	t.Setenv("FACTER_site_location", "lab")
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--no-external-facts", "site_location"}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want environment external fact skipped", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_configNoExternalFactsSkipsEnvironmentExternalFacts(t *testing.T) {
	t.Setenv("FACTER_site_location", "lab")
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "global : {\n  no-external-facts : true,\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "site_location"}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want environment external fact skipped", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

// FACTERLIB is no longer part of the input contract: a directory of .rb fact
// files it points at is never read (ADR-0006).
func TestRun_facterlibHasNoEffectOnDiscovery(t *testing.T) {
	dir := t.TempDir()
	content := []byte("Facter.add('site_role') do\n  setcode do\n    'web'\n  end\nend\n")
	if err := os.WriteFile(filepath.Join(dir, "site.rb"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FACTERLIB", dir)
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"site_role"}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want FACTERLIB directory unread", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_rejectsUnknownLongOption(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(&stdout, &stderr, []string{"--unknown-option", "os.name"})
	if err == nil {
		t.Fatal("Run() err = nil, want unknown option error")
	}
	if !strings.Contains(err.Error(), "unrecognised option '--unknown-option'") {
		t.Fatalf("Run() err = %q, want unrecognised --unknown-option", err)
	}
	assertUsageOutput(t, stdout.String())
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_warnsAndSkipsRubyExternalFact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fact.rb")
	content := []byte("Facter.add(:rb_fact) do\n  setcode { \"x\" }\nend\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "rb_fact"}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want Ruby external fact skipped", got)
	}
	want := "WARN Facts - Ruby fact files are not supported by the Go port; skipping " + path +
		". Rewrite it as an executable external fact (see docs/CUSTOM_FACT_MIGRATION.md).\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

// With legacy facts removed there is nothing for the retired `legacy`
// blocklist group to block: the config loads without error or warning and
// discovery output is unchanged (openspec change remove-legacy-facts).
func TestRun_configBlocklistLegacyIsInert(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "facter.conf")
	conf := "facts : {\n  blocklist : [ \"legacy\" ],\n}\n"
	if err := os.WriteFile(configPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--config", configPath, "--json", "os.name", "kernel"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["os.name"] == nil || got["kernel"] == nil {
		t.Fatalf("stdout = %q, want os.name and kernel unaffected by inert legacy blocklist", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
