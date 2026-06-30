package cli

import "strings"

// ValueArity describes whether a CLI option consumes a value.
type ValueArity int

const (
	NoValue ValueArity = iota
	RequiredValue
)

// OptionDocumentation holds the help and manual rows for a visible option.
type OptionDocumentation struct {
	Help string
	Man  string
}

// Option describes one accepted command-line option and its aliases.
type Option struct {
	Canonical     string
	Aliases       []string
	Arity         ValueArity
	Repeatable    bool
	TaskFlag      bool
	Hidden        bool
	Conflicts     []string
	Documentation OptionDocumentation
}

var optionDefinitions = []Option{
	{
		Canonical: "--color",
		Conflicts: []string{
			"--no-color",
		},
		Documentation: OptionDocumentation{
			Help: "\t       [--color]                      Force color output (default: enabled when writing to a terminal). In the default output format, fact keys are colored by nesting depth.",
			Man:  "  * --color: Force color output (default: enabled when writing to a terminal). In the default output format, fact keys are colored by nesting depth.",
		},
	},
	{
		Canonical: "--no-color",
		Documentation: OptionDocumentation{
			Help: "\t       [--no-color]                   Disable color output.",
			Man:  "  * --no-color: Disable color output.",
		},
	},
	{
		Canonical: "--config",
		Aliases:   []string{"-c"},
		Arity:     RequiredValue,
		Documentation: OptionDocumentation{
			Help: "\t    -c [--config]                     The location of the config file.",
			Man:  "  * -c, --config: The location of the config file.",
		},
	},
	{
		Canonical: "--debug",
		Aliases:   []string{"-d"},
		Documentation: OptionDocumentation{
			Help: "\t    -d [--debug]                      Enable debug output.",
			Man:  "  * -d, --debug: Enable debug output.",
		},
	},
	{
		Canonical:  "--disable",
		Arity:      RequiredValue,
		Repeatable: true,
		Documentation: OptionDocumentation{
			Help: "\t       [--disable]                    Disable facts or fact groups (comma-separated, repeatable). Standalone resolvers are skipped; others are pruned from output, even when queried.",
			Man:  "  * --disable: Disable facts or fact groups (comma-separated, repeatable). Standalone resolvers are skipped; others are pruned from output, even when queried. Unions with the FACTS_DISABLE environment variable and the config file disable list; --no-block clears the whole set.",
		},
	},
	{
		Canonical:  "--external-dir",
		Arity:      RequiredValue,
		Repeatable: true,
		Conflicts: []string{
			"--no-external-facts",
		},
		Documentation: OptionDocumentation{
			Help: "\t       [--external-dir]               A directory to use for external facts.",
			Man:  "  * --external-dir: A directory to use for external facts.",
		},
	},
	{
		Canonical: "--force-dot-resolution",
		Documentation: OptionDocumentation{
			Help: "\t       [--force-dot-resolution]       Merge dotted facts into structured facts.",
			Man:  "  * --force-dot-resolution: Merge dotted facts into structured facts.",
		},
	},
	{
		Canonical: "--hocon",
		Conflicts: []string{
			"--no-hocon",
		},
		Documentation: OptionDocumentation{
			Help: "\t       [--hocon]                      Output in Hocon format.",
			Man:  "  * --hocon: Output in Hocon format.",
		},
	},
	{
		Canonical: "--json",
		Aliases:   []string{"-j"},
		Conflicts: []string{
			"--no-json",
			"-y",
			"--yaml",
			"--hocon",
		},
		Documentation: OptionDocumentation{
			Help: "\t    -j [--json]                       Output in JSON format.",
			Man:  "  * -j, --json: Output in JSON format.",
		},
	},
	{
		Canonical: "--log-level",
		Aliases:   []string{"-l"},
		Arity:     RequiredValue,
		Documentation: OptionDocumentation{
			Help: "\t    -l [--log-level]                  Set logging level.",
			Man:  "  * -l, --log-level: Set logging level.",
		},
	},
	{
		Canonical: "--no-block",
		Documentation: OptionDocumentation{
			Help: "\t       [--no-block]                   Disable fact blocking.",
			Man:  "  * --no-block: Disable fact blocking.",
		},
	},
	{
		Canonical: "--no-cache",
		Documentation: OptionDocumentation{
			Help: "\t       [--no-cache]                   Disable loading and refreshing facts from the cache.",
			Man:  "  * --no-cache: Disable loading and refreshing facts from the cache.",
		},
	},
	{
		Canonical: "--no-external-facts",
		Conflicts: []string{
			"--external-dir",
		},
		Documentation: OptionDocumentation{
			Help: "\t       [--no-external-facts]          Disable external facts.",
			Man:  "  * --no-external-facts: Disable external facts.",
		},
	},
	{Canonical: "--no-hocon", Hidden: true},
	{Canonical: "--no-json", Hidden: true},
	{Canonical: "--no-yaml", Hidden: true},
	{
		Canonical: "--verbose",
		Documentation: OptionDocumentation{
			Help: "\t       [--verbose]                    Enable verbose output.",
			Man:  "  * --verbose: Enable verbose output.",
		},
	},
	{
		Canonical: "--yaml",
		Aliases:   []string{"-y"},
		Conflicts: []string{
			"--no-yaml",
			"-j",
			"--hocon",
		},
		Documentation: OptionDocumentation{
			Help: "\t    -y [--yaml]                       Output in YAML format.",
			Man:  "  * -y, --yaml: Output in YAML format.",
		},
	},
	{
		Canonical: "--strict",
		Documentation: OptionDocumentation{
			Help: "\t       [--strict]                     Enable more aggressive error reporting.",
			Man:  "  * --strict: Enable more aggressive error reporting.",
		},
	},
	{
		Canonical: "--timing",
		Aliases:   []string{"-t"},
		Documentation: OptionDocumentation{
			Help: "\t    -t [--timing]                     Show how much time it took to resolve each fact.",
			Man:  "  * -t, --timing: Show how much time it took to resolve each fact.",
		},
	},
	{
		Canonical: "--sequential",
		Documentation: OptionDocumentation{
			Help: "\t       [--sequential]                 Resolve facts sequentially.",
			Man:  "  * --sequential: Resolve facts sequentially.",
		},
	},
	{
		Canonical: "--http-debug",
		Documentation: OptionDocumentation{
			Help: "\t       [--http-debug]                 Write HTTP request and responses to stderr.",
			Man:  "  * --http-debug: Write HTTP request and responses to stderr.",
		},
	},
	{
		Canonical: "--help",
		Aliases:   []string{"-h"},
		TaskFlag:  true,
		Documentation: OptionDocumentation{
			Help: "\t    -h [--help]                       Help for all arguments",
			Man:  "  * --help, -h: Help for all arguments.",
		},
	},
	{
		Canonical: "--version",
		Aliases:   []string{"-v"},
		TaskFlag:  true,
		Documentation: OptionDocumentation{
			Help: "\t       --version, -v                  Print the version",
			Man:  "  * --version, -v: Print the version.",
		},
	},
	{
		Canonical: "--man",
		TaskFlag:  true,
		Documentation: OptionDocumentation{
			Help: "\t       --man                          Display manual.",
			Man:  "  * --man: Display manual.",
		},
	},
	{
		Canonical: "--list-block-groups",
		TaskFlag:  true,
		Documentation: OptionDocumentation{
			Help: "\t       --list-block-groups            List block groups",
			Man:  "  * --list-block-groups: List block groups.",
		},
	},
	{
		Canonical: "--list-cache-groups",
		TaskFlag:  true,
		Documentation: OptionDocumentation{
			Help: "\t       --list-cache-groups            List cache groups",
			Man:  "  * --list-cache-groups: List cache groups.",
		},
	},
}

var taskNames = map[string]bool{
	"help":              true,
	"query":             true,
	"version":           true,
	"man":               true,
	"list_block_groups": true,
	"list_cache_groups": true,
}

var optionByName = buildOptionIndex()

func buildOptionIndex() map[string]Option {
	options := make(map[string]Option)
	for _, option := range optionDefinitions {
		options[option.Canonical] = option
		for _, alias := range option.Aliases {
			options[alias] = option
		}
	}
	return options
}

// Options returns all accepted CLI options in documentation order.
func Options() []Option {
	options := make([]Option, 0, len(optionDefinitions))
	for _, option := range optionDefinitions {
		options = append(options, copyOption(option))
	}
	return options
}

// DocumentedOptions returns all accepted CLI options that should appear in docs.
func DocumentedOptions() []Option {
	options := []Option{}
	for _, option := range optionDefinitions {
		if option.Hidden {
			continue
		}
		options = append(options, copyOption(option))
	}
	return options
}

// LookupOption returns metadata for arg, accepting aliases and inline values.
func LookupOption(arg string) (Option, bool) {
	name, _, hasInlineValue := strings.Cut(arg, "=")
	if !hasInlineValue {
		name = arg
	}
	option, ok := optionByName[name]
	if !ok || (hasInlineValue && option.Arity == NoValue) {
		return Option{}, false
	}
	return copyOption(option), true
}

// CanonicalOption returns the canonical option name for arg if it is known.
func CanonicalOption(arg string) string {
	option, ok := LookupOption(arg)
	if !ok {
		return arg
	}
	return option.Canonical
}

// KnownOption reports whether arg names an accepted CLI option.
func KnownOption(arg string) bool {
	_, ok := LookupOption(arg)
	return ok
}

// OptionTakesSeparateValue reports whether arg consumes the next argument.
func OptionTakesSeparateValue(arg string) bool {
	if _, _, ok := strings.Cut(arg, "="); ok {
		return false
	}
	option, ok := LookupOption(arg)
	return ok && option.Arity == RequiredValue
}

// ShortOptionTakesAttachedValue reports whether a short option supports -xVALUE.
func ShortOptionTakesAttachedValue(flag byte) bool {
	option, ok := LookupOption("-" + string(flag))
	return ok && option.Arity == RequiredValue
}

// IsTask reports whether arg is a bare app task name.
func IsTask(arg string) bool {
	return taskNames[arg]
}

// IsTaskFlag reports whether arg is an option that maps to an app task.
func IsTaskFlag(arg string) bool {
	option, ok := LookupOption(arg)
	return ok && option.TaskFlag
}

func rawOptionName(arg string) string {
	name, _, ok := strings.Cut(arg, "=")
	if ok {
		return name
	}
	return arg
}

func copyOption(option Option) Option {
	option.Aliases = append([]string(nil), option.Aliases...)
	option.Conflicts = append([]string(nil), option.Conflicts...)
	return option
}

type optionConflict struct {
	option    string
	conflicts []string
}

func optionConflictRules() []optionConflict {
	return append(optionConflictRulesFor("--color", "--json", "--yaml", "--hocon"),
		optionConflict{option: "-j", conflicts: []string{"--no-json", "--hocon"}},
		optionConflict{option: "-y", conflicts: []string{"--no-yaml", "-j", "--hocon"}},
		optionConflictRuleFor("--no-external-facts"),
	)
}

func optionConflictRulesFor(names ...string) []optionConflict {
	rules := make([]optionConflict, 0, len(names))
	for _, name := range names {
		rules = append(rules, optionConflictRuleFor(name))
	}
	return rules
}

func optionConflictRuleFor(name string) optionConflict {
	option, _ := LookupOption(name)
	return optionConflict{
		option:    name,
		conflicts: option.Conflicts,
	}
}
