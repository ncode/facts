package cli

import (
	"slices"
	"strings"
	"testing"
)

func TestPrepareArguments_reordersPriorityFlags(t *testing.T) {
	got := PrepareArguments([]string{"--debug", "--list-cache-groups", "--list-block-groups"})
	want := []string{"--list-cache-groups", "--list-block-groups", "--debug"}
	if !slices.Equal(got, want) {
		t.Fatalf("PrepareArguments() = %v, want %v", got, want)
	}
}

func TestPrepareArguments_reordersTaskNamesAfterOptions(t *testing.T) {
	got := PrepareArguments([]string{"--config", "/tmp/facter.conf", "list_cache_groups"})
	want := []string{"list_cache_groups", "--config", "/tmp/facter.conf"}
	if !slices.Equal(got, want) {
		t.Fatalf("PrepareArguments() = %v, want %v", got, want)
	}
}

func TestPrepareArguments_reordersShortVersionFlag(t *testing.T) {
	got := PrepareArguments([]string{"os.name", "-v"})
	want := []string{"-v", "os.name"}
	if !slices.Equal(got, want) {
		t.Fatalf("PrepareArguments() = %v, want %v", got, want)
	}
}

func TestPrepareArguments_preservesShortOptionsWithEquals(t *testing.T) {
	got := PrepareArguments([]string{"-l=debug", "os.name"})
	want := []string{"query", "-l=debug", "os.name"}
	if !slices.Equal(got, want) {
		t.Fatalf("PrepareArguments() = %v, want %v", got, want)
	}
}

func TestPrepareArguments_expandsShortOptionsWithAttachedValues(t *testing.T) {
	got := PrepareArguments([]string{"-c/tmp/facter.conf", "-ldebug", "os.name"})
	want := []string{"query", "-c", "/tmp/facter.conf", "-l", "debug", "os.name"}
	if !slices.Equal(got, want) {
		t.Fatalf("PrepareArguments() = %v, want %v", got, want)
	}
}

func TestPrepareArguments_addsDefaultQueryTaskWithEqualsOptions(t *testing.T) {
	got := PrepareArguments([]string{"--config=/tmp/facter.conf", "--external-dir=/tmp/facts", "os.name"})
	want := []string{"query", "--config=/tmp/facter.conf", "--external-dir=/tmp/facts", "os.name"}
	if !slices.Equal(got, want) {
		t.Fatalf("PrepareArguments() = %v, want %v", got, want)
	}
}

func TestPrepareArguments_addsDefaultQueryTask(t *testing.T) {
	got := PrepareArguments([]string{"fact1", "fact2"})
	want := []string{"query", "fact1", "fact2"}
	if !slices.Equal(got, want) {
		t.Fatalf("PrepareArguments() = %v, want %v", got, want)
	}
}

func TestValidateOptions_rejectsRepeatedNonRepeatableFlag(t *testing.T) {
	err := ValidateOptions([]string{"query", "--json", "--json", "os.name"})
	if err == nil {
		t.Fatal("ValidateOptions() err = nil, want duplicate option error")
	}
	if !strings.Contains(err.Error(), "option --json cannot be specified more than once") {
		t.Fatalf("ValidateOptions() err = %q, want duplicate --json error", err)
	}
}

func TestValidateOptions_rejectsRepeatedShortLongAlias(t *testing.T) {
	err := ValidateOptions([]string{"query", "--puppet", "-p", "os.name"})
	if err == nil {
		t.Fatal("ValidateOptions() err = nil, want duplicate option error")
	}
	if !strings.Contains(err.Error(), "option --puppet cannot be specified more than once") {
		t.Fatalf("ValidateOptions() err = %q, want duplicate --puppet error", err)
	}
}

func TestValidateOptions_allowsRepeatedExternalDir(t *testing.T) {
	err := ValidateOptions([]string{"query", "--external-dir", "/one", "--external-dir", "/two", "site"})
	if err != nil {
		t.Fatalf("ValidateOptions() err = %v, want nil", err)
	}
}

func TestValidateOptions_rejectsMissingRequiredOptionValue(t *testing.T) {
	err := ValidateOptions([]string{"query", "--external-dir", "--no-external-facts", "site"})
	if err == nil {
		t.Fatal("ValidateOptions() err = nil, want missing option value error")
	}
	if !strings.Contains(err.Error(), "--external-dir requires a value") {
		t.Fatalf("ValidateOptions() err = %q, want missing --external-dir value error", err)
	}
}

func TestValidateOptions_rejectsUnknownConcatenatedShortFlag(t *testing.T) {
	args := PrepareArguments([]string{"-pjdtz"})
	err := ValidateOptions(args)
	if err == nil {
		t.Fatal("ValidateOptions() err = nil, want unknown option error")
	}
	if !strings.Contains(err.Error(), "unrecognised option '-z'") {
		t.Fatalf("ValidateOptions() err = %q, want unknown -z option", err)
	}
}

func TestValidateOptions_rejectsConflictingLogOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "debug and verbose", args: []string{"query", "--debug", "--verbose", "os.name"}},
		{name: "debug and log level info", args: []string{"query", "--debug", "--log-level", "info", "os.name"}},
		{name: "verbose and log level debug", args: []string{"query", "--verbose", "--log-level", "debug", "os.name"}},
		{name: "equals log level and verbose", args: []string{"query", "--log-level=debug", "--verbose", "os.name"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOptions(tt.args)
			if err == nil {
				t.Fatal("ValidateOptions() err = nil, want log option conflict")
			}
			if !strings.Contains(err.Error(), "debug, verbose, and log-level options conflict") {
				t.Fatalf("ValidateOptions() err = %q, want log option conflict", err)
			}
		})
	}
}

func TestValidateOptions_rejectsConflictingColorOptions(t *testing.T) {
	err := ValidateOptions([]string{"query", "--color", "--no-color", "os.name"})
	if err == nil {
		t.Fatal("ValidateOptions() err = nil, want color option conflict")
	}
	if !strings.Contains(err.Error(), "--color and --no-color options conflict") {
		t.Fatalf("ValidateOptions() err = %q, want color option conflict", err)
	}
}

func TestValidateOptions_allowsPuppetWithoutExternalFacts(t *testing.T) {
	if err := ValidateOptions([]string{"query", "--puppet", "--no-external-facts", "os.name"}); err != nil {
		t.Fatalf("ValidateOptions() err = %v, want nil", err)
	}
}

func TestValidateOptions_allowsEquivalentLogOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "debug log level", args: []string{"query", "--debug", "--log-level", "debug", "os.name"}},
		{name: "verbose info level", args: []string{"query", "--verbose", "--log-level", "info", "os.name"}},
		{name: "short debug log level with equals", args: []string{"query", "--debug", "-l=debug", "os.name"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateOptions(tt.args); err != nil {
				t.Fatalf("ValidateOptions() err = %v, want nil", err)
			}
		})
	}
}

func TestValidateOptions_rejectsUnsupportedLogLevel(t *testing.T) {
	err := ValidateOptions([]string{"query", "--log-level", "loud", "os.name"})
	if err == nil {
		t.Fatal("ValidateOptions() err = nil, want unsupported log level error")
	}
	if !strings.Contains(err.Error(), "unsupported log level loud") {
		t.Fatalf("ValidateOptions() err = %q, want unsupported log level error", err)
	}
}
