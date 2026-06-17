// Package acceptance builds the real cmd/facts binary and exercises it
// end-to-end on the live host: release-gate fact set, output formats, and
// exit codes. It runs on all four platform CI gates (Linux, macOS, Windows
// natively via `go test ./...`; FreeBSD via the VM job's package list).
package acceptance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var binaryBuild struct {
	once sync.Once
	bin  string
	dir  string
	err  error
}

func buildFactsBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("acceptance tests build and run the real binary; skipped with -short")
	}
	binaryBuild.once.Do(func() {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			binaryBuild.err = errors.New("runtime.Caller failed")
			return
		}
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
		binaryBuild.dir, binaryBuild.err = os.MkdirTemp("", "facts-acceptance-")
		if binaryBuild.err != nil {
			return
		}
		name := "facts"
		if runtime.GOOS == "windows" {
			name = "facts.exe"
		}
		binaryBuild.bin = filepath.Join(binaryBuild.dir, name)
		build := exec.Command("go", "build", "-buildvcs=false", "-o", binaryBuild.bin, "./cmd/facts")
		build.Dir = repoRoot
		build.Env = os.Environ()
		if out, err := build.CombinedOutput(); err != nil {
			binaryBuild.err = fmt.Errorf("go build ./cmd/facts: %w\n%s", err, out)
		}
	})
	if binaryBuild.err != nil {
		t.Fatal(binaryBuild.err)
	}
	return binaryBuild.bin
}

func runFacts(t *testing.T, args ...string) (stdout, stderr *bytes.Buffer, exitCode int) {
	t.Helper()
	bin := buildFactsBinary(t)
	cmd := exec.Command(bin, args...)
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return stdout, stderr, 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return stdout, stderr, exitErr.ExitCode()
	}
	t.Fatalf("facts %s: %v", strings.Join(args, " "), err)
	return nil, nil, 0
}

// releaseGateFactSet is the cross-platform core of the per-platform release
// gates (tools/*release-gate*, and the Linux/macOS CI smokes): structured
// facts and scalars that must resolve on every supported platform. Legacy
// aliases are removed entirely (openspec change remove-legacy-facts); the
// gates assert structured names only.
var releaseGateFactSet = []string{
	"os.name",
	"os.family",
	"os.release",
	"os.architecture",
	"os.hardware",
	"kernel.name",
	"kernel.release.full",
	"kernel.release.major",
	"kernel.version.full",
	"virtual",
	"is_virtual",
	"networking",
	"memory",
	"memory.system.total",
	"processors",
	"processors.count",
	"system_uptime",
	"timezone",
	"path",
	"facterversion",
}

func expectedKernel() string {
	switch runtime.GOOS {
	case "linux":
		return "Linux"
	case "darwin":
		return "Darwin"
	case "windows":
		return "windows"
	case "freebsd":
		return "FreeBSD"
	default:
		return ""
	}
}

func TestAcceptance_releaseGateFactSetJSON(t *testing.T) {
	args := append([]string{"--json"}, releaseGateFactSet...)
	stdout, stderr, code := runFacts(t, args...)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", code, stderr.String())
	}
	var facts map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &facts); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	for _, name := range releaseGateFactSet {
		value, ok := facts[name]
		if !ok {
			t.Errorf("missing fact %q", name)
			continue
		}
		if value == nil || value == "" {
			t.Errorf("fact %q is empty", name)
		}
	}
	if want := expectedKernel(); want != "" {
		if got := facts["kernel.name"]; got != want {
			t.Errorf("kernel.name = %v, want %v", facts["kernel.name"], want)
		}
	}
	if _, ok := facts["is_virtual"].(bool); !ok {
		t.Errorf("is_virtual = %v (%T), want boolean", facts["is_virtual"], facts["is_virtual"])
	}
	for _, name := range []string{"networking", "memory", "processors", "system_uptime"} {
		if _, ok := facts[name].(map[string]any); !ok {
			t.Errorf("%s = %T, want non-empty map", name, facts[name])
		}
	}
	if t.Failed() {
		t.Logf("facts stdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}

func TestAcceptance_defaultOutputPrintsStructuredFacts(t *testing.T) {
	stdout, stderr, code := runFacts(t)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr: %s", code, stderr.String())
	}
	for _, want := range []string{"memory => {", "networking => {", "os => {", "processors => {"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("default output missing %q", want)
		}
	}
}

func TestAcceptance_singleQueryPrintsScalar(t *testing.T) {
	stdout, _, code := runFacts(t, "kernel.name")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	got := strings.TrimSpace(stdout.String())
	if want := expectedKernel(); want != "" && got != want {
		t.Fatalf("kernel.name = %q, want %q", got, want)
	}
}

func TestAcceptance_dottedQueryResolvesStructuredLeaf(t *testing.T) {
	stdout, _, code := runFacts(t, "os.name")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatal("os.name resolved to empty output")
	}
}

func TestAcceptance_yamlSingleQuery(t *testing.T) {
	stdout, _, code := runFacts(t, "--yaml", "kernel")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	out := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(out, "---") && !strings.Contains(out, "kernel:") {
		t.Fatalf("yaml output = %q, want YAML document with kernel key", out)
	}
}

func TestAcceptance_showLegacyIsAnUnknownOption(t *testing.T) {
	for _, option := range []string{"--show-legacy", "--no-show-legacy"} {
		_, stderr, code := runFacts(t, option)
		if code == 0 {
			t.Errorf("exit(%s) = 0, want usage error for removed option", option)
		}
		if !strings.Contains(stderr.String(), "unrecognised option '"+option+"'") {
			t.Errorf("stderr(%s) = %q, want unrecognised option error", option, stderr.String())
		}
	}
}

func TestAcceptance_legacyAliasesAbsentFromDefaultOutput(t *testing.T) {
	stdout, _, code := runFacts(t, "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var facts map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &facts); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	for _, name := range []string{
		"operatingsystem", "osfamily", "architecture", "hardwaremodel",
		"processorcount", "memorysize", "hostname", "fqdn", "ipaddress",
		"sshfp_rsa", "uptime",
	} {
		if value, ok := facts[name]; ok {
			t.Errorf("legacy alias %q = %v, want absent from default output", name, value)
		}
	}

	stdout, _, code = runFacts(t, "operatingsystem")
	if code != 0 {
		t.Fatalf("exit(operatingsystem) = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Fatalf("facts operatingsystem = %q, want empty output", got)
	}
}

func TestAcceptance_missingFactExitCodes(t *testing.T) {
	stdout, _, code := runFacts(t, "definitely_not_a_fact")
	if code != 0 {
		t.Fatalf("non-strict missing fact exit = %d, want 0", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Fatalf("non-strict missing fact output = %q, want empty", got)
	}

	_, stderr, code := runFacts(t, "--strict", "definitely_not_a_fact")
	if code != 1 {
		t.Fatalf("strict missing fact exit = %d, want 1\nstderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "definitely_not_a_fact") {
		t.Fatalf("strict stderr = %q, want missing fact named", stderr.String())
	}
}
