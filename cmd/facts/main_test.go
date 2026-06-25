package main

import (
	"bytes"
	"context"
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
	"time"

	"github.com/ncode/facts/internal/engine"
)

var commandBuild struct {
	once sync.Once
	dir  string
	bin  string
	err  error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if commandBuild.dir != "" {
		_ = os.RemoveAll(commandBuild.dir)
	}
	os.Exit(code)
}

func TestFactsCommand_version(t *testing.T) {
	bin := buildFactsCommand(t)

	stdout, stderr := runFactsCommand(t, bin, "--version")
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestMainFunctionVersion(t *testing.T) {
	oldArgs, oldStdout, oldStderr := os.Args, os.Stdout, os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Args, os.Stdout, os.Stderr = oldArgs, oldStdout, oldStderr
		_ = stdoutR.Close()
		_ = stderrR.Close()
	})
	os.Args, os.Stdout, os.Stderr = []string{"facts", "--version"}, stdoutW, stderrW

	main()
	_ = stdoutW.Close()
	_ = stderrW.Close()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if _, err := stdout.ReadFrom(stdoutR); err != nil {
		t.Fatal(err)
	}
	if _, err := stderr.ReadFrom(stderrR); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), engine.Version+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunMainReportsOptionErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := runMain(&stdout, &stderr, []string{"-z"}); code != 1 {
		t.Fatalf("runMain() code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "facts [options] [query]") {
		t.Fatalf("stdout = %q, want usage text", stdout.String())
	}
	if got, want := stderr.String(), "ERROR Facts::OptionsValidator - unrecognised option '-z'\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunMainReportsGenericErrors(t *testing.T) {
	writeErr := errors.New("stdout closed")
	var stderr bytes.Buffer

	if code := runMain(errorWriter{err: writeErr}, &stderr, []string{"--version"}); code != 1 {
		t.Fatalf("runMain() code = %d, want 1", code)
	}
	if got := stderr.String(); got == "" || strings.Contains(got, "Facts::OptionsValidator") {
		t.Fatalf("stderr = %q, want generic app error", got)
	}
	if got := stderr.String(); !strings.Contains(got, writeErr.Error()) {
		t.Fatalf("stderr = %q, want %q", got, writeErr)
	}
}

func TestFactsCommand_noQueryPrintsStructuredFacts(t *testing.T) {
	bin := buildFactsCommand(t)

	stdout, stderr := runFactsCommand(t, bin)
	for _, want := range []string{"memory => {", "networking => {", "os => {", "path =>", "processors => {"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFactsCommand_noQueryJSONPrintsValidJSON(t *testing.T) {
	bin := buildFactsCommand(t)

	stdout, stderr := runFactsCommand(t, bin, "-j")
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	for _, want := range []string{"memory", "networking", "os", "path", "processors"} {
		if got[want] == nil {
			t.Fatalf("stdout = %q, want %q root fact", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFactsCommand_concatenatedJSONAndDebugFlags(t *testing.T) {
	bin := buildFactsCommand(t)

	stdout, stderr := runFactsCommand(t, bin, "-jd")
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout.String(), err)
	}
	if !strings.Contains(stderr.String(), "DEBUG Facts -") {
		t.Fatalf("stderr = %q, want DEBUG log", stderr.String())
	}
}

func TestFactsCommand_concatenatedTimingAndDebugFlags(t *testing.T) {
	bin := buildFactsCommand(t)

	stdout, stderr := runFactsCommand(t, bin, "-td", "facterversion")
	if !strings.Contains(stdout.String(), "fact 'facterversion', took: ") {
		t.Fatalf("stdout = %q, want timing output", stdout.String())
	}
	if !strings.Contains(stderr.String(), "DEBUG Facts -") {
		t.Fatalf("stderr = %q, want DEBUG log", stderr.String())
	}
}

func TestFactsCommand_queryPrintsFactValue(t *testing.T) {
	bin := buildFactsCommand(t)

	stdout, stderr := runFactsCommand(t, bin, "networking.hostname")
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatalf("stdout = %q, want hostname", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFactsCommand_externalFactRecursionGuard(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exercises POSIX execute-bit and shebang semantics; covered on the POSIX platform gates")
	}
	bin := buildFactsCommand(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "recursive_fact")
	content := "#!/bin/sh\n\"" + bin + "\" --external-dir \"" + dir + "\" os >/dev/null\nprintf 'recursive_fact=ok\\n'\n"
	if err := os.WriteFile(script, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := runFactsCommand(t, bin, "--external-dir", dir, "recursive_fact")
	if got, want := stdout.String(), "ok\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if !strings.Contains(stderr.String(), "Recursion detected") {
		t.Fatalf("stderr = %q, want recursion warning", stderr.String())
	}
}

func TestFactsCommand_strictMissingFactLogsMissingFactErrorLikeRubyCLI(t *testing.T) {
	bin := buildFactsCommand(t)

	stdout, stderr, status := runFactsCommandStatus(t, bin, "--strict", "--json", "os.name", "missing_fact")
	if status != 1 {
		t.Fatalf("exit status = %d, want 1", status)
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

func TestFactsCommand_invalidConcatenatedShortFlagReportsOptionsValidatorError(t *testing.T) {
	bin := buildFactsCommand(t)

	stdout, stderr, status := runFactsCommandStatus(t, bin, "-jdtz")
	if status == 0 {
		t.Fatal("exit status = 0, want non-zero")
	}
	for _, want := range []string{"Usage", "facts [options] [query]"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if got, want := stderr.String(), "ERROR Facts::OptionsValidator - unrecognised option '-z'\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func buildFactsCommand(t *testing.T) string {
	t.Helper()
	commandBuild.once.Do(func() {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			commandBuild.err = errors.New("runtime.Caller failed")
			return
		}
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
		commandBuild.dir, commandBuild.err = os.MkdirTemp("", "facts-command-test-")
		if commandBuild.err != nil {
			return
		}
		name := "facts"
		if runtime.GOOS == "windows" {
			name = "facts.exe"
		}
		commandBuild.bin = filepath.Join(commandBuild.dir, name)
		build := exec.Command("go", "build", "-buildvcs=false", "-o", commandBuild.bin, "./cmd/facts")
		build.Dir = repoRoot
		build.Env = os.Environ()
		if out, err := build.CombinedOutput(); err != nil {
			commandBuild.err = fmt.Errorf("go build: %w\n%s", err, out)
		}
	})
	if commandBuild.err != nil {
		t.Fatal(commandBuild.err)
	}
	return commandBuild.bin
}

func runFactsCommand(t *testing.T, bin string, args ...string) (*bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout, stderr, status := runFactsCommandStatus(t, bin, args...)
	if status != 0 {
		t.Fatalf("facts %s: exit status %d\nstderr: %s", strings.Join(args, " "), status, stderr.String())
	}
	return stdout, stderr
}

func runFactsCommandStatus(t *testing.T, bin string, args ...string) (*bytes.Buffer, *bytes.Buffer, int) {
	t.Helper()
	// Cold Windows runners need several seconds for the first WMI and
	// PowerShell-CIM queries; 5s killed the binary mid-resolution.
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, bin, args...)
	defaultBase := t.TempDir()
	cmd.Env = append(os.Environ(),
		"HOME="+defaultBase,
		"ProgramData="+defaultBase,
		"APPDATA="+defaultBase,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &stdout, &stderr, exitErr.ExitCode()
		}
		t.Fatalf("facts %s: %v\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return &stdout, &stderr, 0
}
