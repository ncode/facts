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
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			if IsTask(arg) {
				continue
			}
			break
		}
		if strings.HasPrefix(arg, "-") {
			seenRaw[rawOption(arg)] = true
			option, ok := LookupOption(arg)
			if !ok {
				return fmt.Errorf("unrecognised option '%s'", arg)
			}
			seen[option.Canonical] = true
			counts[option.Canonical]++
			if option.Canonical == "--log-level" {
				if _, value, ok := strings.Cut(arg, "="); ok {
					logLevel = value
				} else if i+1 < len(args) {
					logLevel = args[i+1]
				}
			}
		}
		if OptionTakesSeparateValue(arg) {
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", CanonicalOption(arg))
			}
			i++
		}
	}

	for option, count := range counts {
		metadata, _ := LookupOption(option)
		if count > 1 && !metadata.Repeatable {
			return fmt.Errorf("option %s cannot be specified more than once.", option)
		}
	}

	for _, invalid := range optionConflictRules() {
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

func rawOption(arg string) string {
	name, _, ok := strings.Cut(arg, "=")
	if ok {
		return name
	}
	return arg
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
