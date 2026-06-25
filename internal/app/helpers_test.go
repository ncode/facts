package app

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestExitStatusReportsCodeAndError(t *testing.T) {
	status := ExitStatus(42)

	if got := status.Code(); got != 42 {
		t.Fatalf("Code() = %d, want 42", got)
	}
	if got := status.Error(); got != "exit status 42" {
		t.Fatalf("Error() = %q, want exit status 42", got)
	}
}

func TestOptionErrorWritesHelpAndReturnsOriginalError(t *testing.T) {
	var stdout bytes.Buffer
	err := errors.New("bad option")

	got := optionError(&stdout, err)
	if got != err {
		t.Fatalf("optionError() = %v, want original error", got)
	}
	if got, want := stdout.String(), helpText(); got != want {
		t.Fatalf("stdout = %q, want help text %q", got, want)
	}
	for _, marker := range []string{"Usage", "--config", "--help"} {
		if !strings.Contains(stdout.String(), marker) {
			t.Fatalf("stdout missing help marker %q: %q", marker, stdout.String())
		}
	}
}

func TestOptionErrorReturnsHelpWriteError(t *testing.T) {
	writeErr := errors.New("stdout closed")
	originalErr := errors.New("bad option")

	got := optionError(errorWriter{err: writeErr}, originalErr)
	if !errors.Is(got, writeErr) {
		t.Fatalf("optionError() = %v, want help write error %v", got, writeErr)
	}
}

func TestResolveColorHonorsFlagsAndWriter(t *testing.T) {
	if !resolveColor(true, false, &bytes.Buffer{}) {
		t.Fatal("resolveColor(force=true) = false, want true")
	}
	if resolveColor(true, true, &bytes.Buffer{}) {
		t.Fatal("resolveColor(disable=true) = true, want false")
	}
	if resolveColor(false, false, &bytes.Buffer{}) {
		t.Fatal("resolveColor(non-file writer) = true, want false")
	}

	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if resolveColor(false, false, file) {
		t.Fatal("resolveColor(regular file) = true, want false")
	}
}

func TestResolvedLogOptionsConflict(t *testing.T) {
	tests := []struct {
		name     string
		debug    bool
		verbose  bool
		logLevel string
		want     bool
	}{
		{name: "debug and verbose conflict", debug: true, verbose: true, want: true},
		{name: "no log level has no conflict", debug: true},
		{name: "placeholder log_level has no conflict", debug: true, logLevel: "log_level"},
		{name: "debug with debug log level is redundant", debug: true, logLevel: "debug"},
		{name: "debug with trace log level is redundant", debug: true, logLevel: "trace"},
		{name: "verbose with info log level is redundant", verbose: true, logLevel: "info"},
		{name: "debug with info log level conflicts", debug: true, logLevel: "info", want: true},
		{name: "verbose with debug log level conflicts", verbose: true, logLevel: "debug", want: true},
		{name: "bare log level has no conflict", logLevel: "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvedLogOptionsConflict(tt.debug, tt.verbose, tt.logLevel)
			if got != tt.want {
				t.Fatalf("resolvedLogOptionsConflict(%v, %v, %q) = %v, want %v", tt.debug, tt.verbose, tt.logLevel, got, tt.want)
			}
		})
	}
}

func TestWriteErrorColorContract(t *testing.T) {
	var stderr bytes.Buffer

	writeError(&stderr, "boom", true)

	if got, want := stderr.String(), "\x1b[31mERROR Facts - boom\x1b[0m\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}

	stderr.Reset()
	writeError(&stderr, "boom", false)

	if got, want := stderr.String(), "ERROR Facts - boom\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
