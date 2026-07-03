package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_disableOptionDropsNamedFacts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), []byte("alpha: 1\nbeta: 2\ngamma: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "--disable", "alpha,beta", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if _, ok := got["alpha"]; ok {
		t.Fatalf("stdout = %q, want alpha disabled by --disable", stdout.String())
	}
	if _, ok := got["beta"]; ok {
		t.Fatalf("stdout = %q, want beta disabled by --disable", stdout.String())
	}
	if got["gamma"] == nil {
		t.Fatalf("stdout = %q, want gamma resolved", stdout.String())
	}
}

func TestRun_disableOptionRepeatableAcrossFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), []byte("alpha: 1\nbeta: 2\ngamma: 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "--disable", "alpha", "--disable", "beta", "--json"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if _, ok := got["alpha"]; ok {
		t.Fatalf("stdout = %q, want alpha disabled by first --disable", stdout.String())
	}
	if _, ok := got["beta"]; ok {
		t.Fatalf("stdout = %q, want beta disabled by repeated --disable", stdout.String())
	}
	if got["gamma"] == nil {
		t.Fatalf("stdout = %q, want gamma resolved", stdout.String())
	}
}

func TestRun_noBlockClearsDisableOption(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), []byte("alpha: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--external-dir", dir, "--disable", "alpha", "--no-block", "--json", "alpha"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if got["alpha"] == nil {
		t.Fatalf("stdout = %q, want --no-block to clear --disable and resolve alpha", stdout.String())
	}
}

// The version fast path must fall through to full discovery when facterversion
// is disabled by an ambient source, so a disabled version query behaves like
// any other disabled fact. Pinned in the bare legacy format (the only format
// whose disabled-single-query stdout is literally empty) before the fast path
// consumes the engine disabled-union, so the reroute cannot alter fall-through.
func TestRun_facterversionDisabledByEnvFallsThrough(t *testing.T) {
	t.Setenv("FACTS_DISABLE", "facterversion")
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"facterversion"}); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty (disabled facterversion falls through)", stdout.String())
	}
	if got, want := stderr.String(), "WARN Facts - fact \"facterversion\" is disabled by FACTS_DISABLE\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRun_facterversionDisabledByFlagFallsThrough(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if err := Run(&stdout, &stderr, []string{"--disable", "facterversion", "facterversion"}); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty (disabled facterversion falls through)", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want no diagnostic for --disable", stderr.String())
	}
}
