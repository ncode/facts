package cli

import "testing"

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

func hasOption(options []string, want string) bool {
	for _, option := range options {
		if option == want {
			return true
		}
	}
	return false
}
