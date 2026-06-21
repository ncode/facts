package cli

import (
	"errors"
	"testing"
)

func TestValidateOptions_rejectsInvalidPairs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "json conflicts with yaml",
			args: []string{"--json", "--yaml"},
			want: "--json and --yaml options conflict: please specify only one.",
		},
		{
			name: "json conflicts with short yaml alias",
			args: []string{"--json", "-y"},
			want: "--json and -y options conflict: please specify only one.",
		},
		{
			name: "yaml conflicts with short json alias",
			args: []string{"--yaml", "-j"},
			want: "--yaml and -j options conflict: please specify only one.",
		},
		{
			name: "short json alias conflicts with hocon",
			args: []string{"-j", "--hocon"},
			want: "-j and --hocon options conflict: please specify only one.",
		},
		{
			name: "external dir conflicts with no external facts",
			args: []string{"--external-dir", "/facts", "--no-external-facts"},
			want: "--no-external-facts and --external-dir options conflict: please specify only one.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOptions(tt.args)
			if err == nil {
				t.Fatal("ValidateOptions() err = nil, want error")
			}
			if _, ok := errors.AsType[*OptionError](err); !ok {
				t.Fatalf("ValidateOptions() err type = %T, want *OptionError", err)
			}
			if got := err.Error(); got != tt.want {
				t.Fatalf("ValidateOptions() err = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateOptions_rejectsDuplicateCanonicalOptions(t *testing.T) {
	err := ValidateOptions([]string{"--json", "-j"})
	if err == nil {
		t.Fatal("ValidateOptions() err = nil, want duplicate option error")
	}
	if got, want := err.Error(), "option --json cannot be specified more than once."; got != want {
		t.Fatalf("ValidateOptions() err = %q, want %q", got, want)
	}
}

func TestValidateOptions_acceptsRepeatableDirectoryOptions(t *testing.T) {
	err := ValidateOptions([]string{"--external-dir", "/first", "--external-dir", "/second"})
	if err != nil {
		t.Fatalf("ValidateOptions() err = %v, want nil", err)
	}
}

func TestValidateOptions_rejectsRemovedCustomFactOptions(t *testing.T) {
	for _, option := range []string{"--custom-dir", "--no-ruby", "--no-custom-facts", "--trace"} {
		t.Run(option, func(t *testing.T) {
			err := ValidateOptions([]string{option, "/facts"})
			if err == nil {
				t.Fatal("ValidateOptions() err = nil, want unknown option error")
			}
			if got, want := err.Error(), "unrecognised option '"+option+"'"; got != want {
				t.Fatalf("ValidateOptions() err = %q, want %q", got, want)
			}
		})
	}
}

func TestValidateOptions_rejectsUnknownConcatenatedShortOption(t *testing.T) {
	args := PrepareArguments([]string{"-jdtz"})
	err := ValidateOptions(args)
	if err == nil {
		t.Fatal("ValidateOptions() err = nil, want unknown option error")
	}
	if got, want := err.Error(), "unrecognised option '-z'"; got != want {
		t.Fatalf("ValidateOptions() err = %q, want %q", got, want)
	}
}

func TestValidateOptions_rejectsInlineValueForNoValueOption(t *testing.T) {
	err := ValidateOptions([]string{"--json=false", "os.name"})
	if err == nil {
		t.Fatal("ValidateOptions() err = nil, want unknown option error")
	}
	if got, want := err.Error(), "unrecognised option '--json=false'"; got != want {
		t.Fatalf("ValidateOptions() err = %q, want %q", got, want)
	}
}

func TestValidateOptions_validatesLogLevelCombinations(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "debug allows debug log level", args: []string{"--debug", "--log-level", "debug"}},
		{name: "debug allows trace log level like Ruby normalization", args: []string{"--debug", "--log-level", "trace"}},
		{name: "verbose allows info log level", args: []string{"--verbose", "--log-level=info"}},
		{name: "accepts placeholder log level", args: []string{"--log-level", "log_level"}},
		{name: "debug rejects info log level", args: []string{"--debug", "--log-level", "info"}, wantErr: "debug, verbose, and log-level options conflict: please specify only one."},
		{name: "unsupported level", args: []string{"--log-level", "loud"}, wantErr: "unsupported log level loud"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOptions(tt.args)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateOptions() err = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateOptions() err = nil, want error")
			}
			if got := err.Error(); got != tt.wantErr {
				t.Fatalf("ValidateOptions() err = %q, want %q", got, tt.wantErr)
			}
		})
	}
}
