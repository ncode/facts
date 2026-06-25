package cli

import (
	"slices"
	"testing"
)

func TestOptionsReturnDefensiveCopies(t *testing.T) {
	options := Options()
	if len(options) == 0 {
		t.Fatal("Options() returned no options")
	}
	originals := make([]Option, len(options))
	for i := range options {
		originals[i] = options[i]
		originals[i].Aliases = append([]string(nil), options[i].Aliases...)
		originals[i].Conflicts = append([]string(nil), options[i].Conflicts...)
	}
	t.Cleanup(func() {
		current := Options()
		for i := range originals {
			if i >= len(current) {
				continue
			}
			for j, alias := range originals[i].Aliases {
				if j < len(current[i].Aliases) {
					current[i].Aliases[j] = alias
				}
			}
			for j, conflict := range originals[i].Conflicts {
				if j < len(current[i].Conflicts) {
					current[i].Conflicts[j] = conflict
				}
			}
			options[i] = originals[i]
			options[i].Aliases = append([]string(nil), originals[i].Aliases...)
			options[i].Conflicts = append([]string(nil), originals[i].Conflicts...)
		}
	})
	for i := range options {
		options[i].Canonical = "--mutated-canonical"
		if len(options[i].Aliases) > 0 {
			options[i].Aliases[0] = "--mutated-alias"
		}
		if len(options[i].Conflicts) > 0 {
			options[i].Conflicts[0] = "--mutated-conflict"
		}
	}

	fresh := Options()
	if len(fresh) != len(originals) {
		t.Fatalf("len(Options()) = %d, want %d", len(fresh), len(originals))
	}
	for i := range fresh {
		if fresh[i].Canonical != originals[i].Canonical {
			t.Fatalf("Options()[%d].Canonical = %q, want %q", i, fresh[i].Canonical, originals[i].Canonical)
		}
		if !slices.Equal(fresh[i].Aliases, originals[i].Aliases) {
			t.Fatalf("Options()[%d].Aliases = %#v, want %#v", i, fresh[i].Aliases, originals[i].Aliases)
		}
		if !slices.Equal(fresh[i].Conflicts, originals[i].Conflicts) {
			t.Fatalf("Options()[%d].Conflicts = %#v, want %#v", i, fresh[i].Conflicts, originals[i].Conflicts)
		}
	}
}

func TestDocumentedOptionsReturnDefensiveCopies(t *testing.T) {
	options := DocumentedOptions()
	if len(options) == 0 {
		t.Fatal("DocumentedOptions() returned no options")
	}
	originals := make([]Option, len(options))
	for i := range options {
		originals[i] = options[i]
		originals[i].Aliases = slices.Clone(options[i].Aliases)
		originals[i].Conflicts = slices.Clone(options[i].Conflicts)
	}
	t.Cleanup(func() {
		current := DocumentedOptions()
		for i := range originals {
			if i >= len(current) {
				continue
			}
			for j, alias := range originals[i].Aliases {
				if j < len(current[i].Aliases) {
					current[i].Aliases[j] = alias
				}
			}
			for j, conflict := range originals[i].Conflicts {
				if j < len(current[i].Conflicts) {
					current[i].Conflicts[j] = conflict
				}
			}
			options[i] = originals[i]
			options[i].Aliases = slices.Clone(originals[i].Aliases)
			options[i].Conflicts = slices.Clone(originals[i].Conflicts)
		}
	})
	for i := range options {
		options[i].Canonical = "--mutated-canonical"
		if len(options[i].Aliases) > 0 {
			options[i].Aliases[0] = "--mutated-alias"
		}
		if len(options[i].Conflicts) > 0 {
			options[i].Conflicts[0] = "--mutated-conflict"
		}
	}

	fresh := DocumentedOptions()
	if len(fresh) != len(originals) {
		t.Fatalf("len(DocumentedOptions()) = %d, want %d", len(fresh), len(originals))
	}
	for i := range fresh {
		if fresh[i].Canonical != originals[i].Canonical {
			t.Fatalf("DocumentedOptions()[%d].Canonical = %q, want %q", i, fresh[i].Canonical, originals[i].Canonical)
		}
		if !slices.Equal(fresh[i].Aliases, originals[i].Aliases) {
			t.Fatalf("DocumentedOptions()[%d].Aliases = %#v, want %#v", i, fresh[i].Aliases, originals[i].Aliases)
		}
		if !slices.Equal(fresh[i].Conflicts, originals[i].Conflicts) {
			t.Fatalf("DocumentedOptions()[%d].Conflicts = %#v, want %#v", i, fresh[i].Conflicts, originals[i].Conflicts)
		}
	}
}

func TestOptionsExposeVisibleAndFixedHiddenContract(t *testing.T) {
	expectedHidden := map[string]bool{
		"--no-hocon": true,
		"--no-json":  true,
		"--no-yaml":  true,
	}
	seenHidden := map[string]bool{}

	for _, option := range DocumentedOptions() {
		if option.Hidden {
			t.Fatalf("DocumentedOptions() included hidden option %#v", option)
		}
	}
	documented := map[string]bool{}
	for _, option := range DocumentedOptions() {
		documented[option.Canonical] = true
	}

	for _, option := range Options() {
		if option.Hidden {
			if !expectedHidden[option.Canonical] {
				t.Fatalf("Options() marked unexpected option %q as hidden", option.Canonical)
			}
			seenHidden[option.Canonical] = true
			if !KnownOption(option.Canonical) {
				t.Fatalf("KnownOption(%q) = false, want hidden option still accepted", option.Canonical)
			}
			if documented[option.Canonical] {
				t.Fatalf("DocumentedOptions() included hidden option %q", option.Canonical)
			}
			continue
		}
		if !documented[option.Canonical] {
			t.Fatalf("DocumentedOptions() omitted visible option %q", option.Canonical)
		}
	}
	for canonical := range expectedHidden {
		if !seenHidden[canonical] {
			t.Fatalf("Options() did not mark expected hidden option %q as hidden", canonical)
		}
	}
}

func TestOptions_describeAcceptedOptionMetadata(t *testing.T) {
	tests := []struct {
		name       string
		arg        string
		canonical  string
		arity      ValueArity
		repeatable bool
		taskFlag   bool
		hidden     bool
		conflicts  []string
	}{
		{
			name:       "external dir is repeatable valued option",
			arg:        "--external-dir",
			canonical:  "--external-dir",
			arity:      RequiredValue,
			repeatable: true,
			conflicts:  []string{"--no-external-facts"},
		},
		{
			name:      "short config alias canonicalizes to config",
			arg:       "-c",
			canonical: "--config",
			arity:     RequiredValue,
		},
		{
			name:      "short log-level alias canonicalizes to log-level",
			arg:       "-l=debug",
			canonical: "--log-level",
			arity:     RequiredValue,
		},
		{
			name:      "list block groups is a task flag",
			arg:       "--list-block-groups",
			canonical: "--list-block-groups",
			taskFlag:  true,
		},
		{
			name:      "force dot resolution is documented",
			arg:       "--force-dot-resolution",
			canonical: "--force-dot-resolution",
		},
		{
			name:      "inverse json compatibility option is hidden",
			arg:       "--no-json",
			canonical: "--no-json",
			hidden:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option, ok := LookupOption(tt.arg)
			if !ok {
				t.Fatalf("LookupOption(%q) ok = false, want true", tt.arg)
			}
			if option.Canonical != tt.canonical {
				t.Fatalf("LookupOption(%q).Canonical = %q, want %q", tt.arg, option.Canonical, tt.canonical)
			}
			if option.Arity != tt.arity {
				t.Fatalf("LookupOption(%q).Arity = %v, want %v", tt.arg, option.Arity, tt.arity)
			}
			if option.Repeatable != tt.repeatable {
				t.Fatalf("LookupOption(%q).Repeatable = %v, want %v", tt.arg, option.Repeatable, tt.repeatable)
			}
			if option.TaskFlag != tt.taskFlag {
				t.Fatalf("LookupOption(%q).TaskFlag = %v, want %v", tt.arg, option.TaskFlag, tt.taskFlag)
			}
			if option.Hidden != tt.hidden {
				t.Fatalf("LookupOption(%q).Hidden = %v, want %v", tt.arg, option.Hidden, tt.hidden)
			}
			for _, conflict := range tt.conflicts {
				if !hasOption(option.Conflicts, conflict) {
					t.Fatalf("LookupOption(%q).Conflicts = %v, want %q", tt.arg, option.Conflicts, conflict)
				}
			}
			if !option.Hidden && option.Documentation.Help == "" {
				t.Fatalf("LookupOption(%q).Documentation.Help is empty for non-hidden option", tt.arg)
			}
			if !option.Hidden && option.Documentation.Man == "" {
				t.Fatalf("LookupOption(%q).Documentation.Man is empty for non-hidden option", tt.arg)
			}
		})
	}
}

func TestOptionValueHelpersUseSharedMetadata(t *testing.T) {
	separateValue := []string{"--external-dir", "--config", "-c", "--log-level", "-l"}
	for _, arg := range separateValue {
		t.Run(arg, func(t *testing.T) {
			if !OptionTakesSeparateValue(arg) {
				t.Fatalf("OptionTakesSeparateValue(%q) = false, want true", arg)
			}
		})
	}

	inlineValue := []string{"--external-dir=/facts", "--config=/facts.conf", "-c=/facts.conf", "-l=debug"}
	for _, arg := range inlineValue {
		t.Run(arg, func(t *testing.T) {
			if OptionTakesSeparateValue(arg) {
				t.Fatalf("OptionTakesSeparateValue(%q) = true, want false", arg)
			}
		})
	}

	if !ShortOptionTakesAttachedValue('c') {
		t.Fatal("ShortOptionTakesAttachedValue('c') = false, want true")
	}
	if !ShortOptionTakesAttachedValue('l') {
		t.Fatal("ShortOptionTakesAttachedValue('l') = false, want true")
	}
	if ShortOptionTakesAttachedValue('j') {
		t.Fatal("ShortOptionTakesAttachedValue('j') = true, want false")
	}
}

func TestLookupOptionRejectsInlineValueForNoValueOption(t *testing.T) {
	if _, ok := LookupOption("--json=false"); ok {
		t.Fatal("LookupOption(\"--json=false\") ok = true, want false")
	}
}

func TestOptionLookupHelpersHandleUnknownAndRawNames(t *testing.T) {
	if got := CanonicalOption("--missing"); got != "--missing" {
		t.Fatalf("CanonicalOption(--missing) = %q, want original", got)
	}
	if KnownOption("--missing") {
		t.Fatal("KnownOption(--missing) = true, want false")
	}
	if got := rawOptionName("--config=/tmp/facts.conf"); got != "--config" {
		t.Fatalf("rawOptionName() = %q, want --config", got)
	}
	if got := rawOptionName("--json"); got != "--json" {
		t.Fatalf("rawOptionName() = %q, want --json", got)
	}
}

func hasOption(options []string, want string) bool {
	for _, option := range options {
		if option == want {
			return true
		}
	}
	return false
}
