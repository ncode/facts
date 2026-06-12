package cli

import (
	"errors"
	"fmt"
	"strings"
)

// OptionError identifies command-line option validation failures.
type OptionError struct {
	Err error
}

func (oe *OptionError) Error() string {
	if oe == nil || oe.Err == nil {
		return ""
	}
	return oe.Err.Error()
}

func (oe *OptionError) Unwrap() error {
	if oe == nil {
		return nil
	}
	return oe.Err
}

// IsOptionError reports whether err came from option validation.
func IsOptionError(err error) bool {
	_, ok := errors.AsType[*OptionError](err)
	return ok
}

// ValidateOptions reports invalid option combinations before command
// execution, matching Ruby Facter's validator.
func ValidateOptions(args []string) error {
	if err := validateOptions(args); err != nil {
		return &OptionError{Err: err}
	}
	return nil
}

func validateOptions(args []string) error {
	seen := make(map[string]bool, len(args))
	seenRaw := make(map[string]bool, len(args))
	counts := make(map[string]int, len(args))
	logLevel := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			seenRaw[rawOption(arg)] = true
			option := canonicalOption(arg)
			if !knownOption(option) {
				return fmt.Errorf("unrecognised option '%s'", arg)
			}
			seen[option] = true
			counts[option]++
			if option == "--log-level" {
				if _, value, ok := strings.Cut(arg, "="); ok {
					logLevel = value
				} else if i+1 < len(args) {
					logLevel = args[i+1]
				}
			}
		}
		if optionTakesSeparateValue(arg) {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return fmt.Errorf("%s requires a value", canonicalOption(arg))
			}
			i++
		}
	}

	for option, count := range counts {
		if count > 1 && !repeatableOption(option) {
			return fmt.Errorf("option %s cannot be specified more than once.", option)
		}
	}

	invalid := []optionConflict{
		{option: "--color", conflicts: []string{"--no-color"}},
		{option: "--json", conflicts: []string{"--no-json", "-y", "--yaml", "--hocon"}},
		{option: "--yaml", conflicts: []string{"--no-yaml", "-j", "--hocon"}},
		{option: "--hocon", conflicts: []string{"--no-hocon"}},
		{option: "-j", conflicts: []string{"--no-json", "--hocon"}},
		{option: "-y", conflicts: []string{"--no-yaml", "-j", "--hocon"}},
		{option: "--puppet", conflicts: []string{"--no-puppet"}},
		{option: "-p", conflicts: []string{"--no-puppet"}},
		{option: "--no-external-facts", conflicts: []string{"--external-dir"}},
	}
	for _, invalid := range invalid {
		if !seenRaw[invalid.option] {
			continue
		}
		for _, conflict := range invalid.conflicts {
			if seenRaw[conflict] {
				return fmt.Errorf("%s and %s options conflict: please specify only one.", invalid.option, conflict)
			}
		}
	}
	if logOptionsConflict(seen, logLevel) {
		return fmt.Errorf("debug, verbose, and log-level options conflict: please specify only one.")
	}
	if seen["--log-level"] && !SupportedLogLevel(logLevel) {
		return fmt.Errorf("unsupported log level %s", logLevel)
	}
	return nil
}

type optionConflict struct {
	option    string
	conflicts []string
}

func rawOption(arg string) string {
	name, _, ok := strings.Cut(arg, "=")
	if ok {
		return name
	}
	return arg
}

func canonicalOption(arg string) string {
	if !strings.HasPrefix(arg, "--") {
		return shortOptionAlias(arg)
	}
	name, _, ok := strings.Cut(arg, "=")
	if !ok {
		return arg
	}
	return name
}

func shortOptionAlias(arg string) string {
	name, _, _ := strings.Cut(arg, "=")
	switch arg {
	case "-j":
		return "--json"
	case "-y":
		return "--yaml"
	case "-p":
		return "--puppet"
	case "-h":
		return "--help"
	case "-v":
		return "--version"
	case "-d":
		return "--debug"
	case "-t":
		return "--timing"
	case "-c":
		return "--config"
	case "-l":
		return "--log-level"
	default:
		switch name {
		case "-c":
			return "--config"
		case "-l":
			return "--log-level"
		}
		return arg
	}
}

func knownOption(arg string) bool {
	switch arg {
	case "--color", "--config", "--debug", "--external-dir",
		"--force-dot-resolution", "--help", "--hocon", "--http-debug", "--json", "--list-block-groups",
		"--list-cache-groups", "--log-level", "--no-block", "--no-cache",
		"--no-color", "--no-external-facts", "--no-hocon",
		"--no-json", "--no-puppet", "--no-yaml", "--man", "--puppet",
		"--sequential", "--strict", "--timing",
		"--verbose", "--version", "--yaml":
		return true
	default:
		return false
	}
}

func repeatableOption(arg string) bool {
	switch arg {
	case "--external-dir":
		return true
	default:
		return false
	}
}

func optionTakesSeparateValue(arg string) bool {
	if _, _, ok := strings.Cut(arg, "="); ok {
		return false
	}
	switch arg {
	case "--external-dir", "--config", "-c", "--log-level", "-l":
		return true
	default:
		return false
	}
}

func logOptionsConflict(seen map[string]bool, logLevel string) bool {
	debug := seen["--debug"]
	verbose := seen["--verbose"]
	logLevelSet := seen["--log-level"]
	if debug && verbose {
		return true
	}
	if debug && logLevelSet && logLevel != "debug" && logLevel != "trace" {
		return true
	}
	if verbose && logLevelSet && logLevel != "info" {
		return true
	}
	return false
}

// SupportedLogLevel reports whether level is accepted by Ruby-compatible log-level validation.
func SupportedLogLevel(level string) bool {
	switch level {
	case "none", "trace", "debug", "info", "warn", "error", "fatal", "log_level":
		return true
	default:
		return false
	}
}
