package app

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/ncode/facts/internal/cli"
	"github.com/ncode/facts/internal/engine"
)

type parsedOptions struct {
	JSON               bool
	NoJSON             bool
	YAML               bool
	NoYAML             bool
	HOCON              bool
	NoHOCON            bool
	Debug              bool
	Color              bool
	NoColor            bool
	Timing             bool
	Strict             bool
	ForceDotResolution bool
	NoBlock            bool
	NoCache            bool
	Verbose            bool
	Sequential         bool
	HTTPDebug          bool
	NoExternalFacts    bool

	ConfigPath     string
	LogLevel       string
	ExternalDirs   []string
	DisableEntries []string
}

type optionBinder func(*flag.FlagSet, string)

func parseOptions(name string, output io.Writer, args []string) (*flag.FlagSet, *parsedOptions, error) {
	flags, parsed, err := newParsedOptionFlagSet(name, output)
	if err != nil {
		return nil, nil, err
	}
	if err := flags.Parse(args); err != nil {
		return nil, nil, err
	}
	return flags, parsed, nil
}

func parseListOptions(args []string) *parsedOptions {
	parsed := &parsedOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		option, ok := cli.LookupOption(arg)
		if !ok || option.TaskFlag {
			continue
		}
		value, hasInlineValue := inlineOptionValue(arg)
		switch option.Canonical {
		case "--config":
			if hasInlineValue {
				parsed.ConfigPath = value
			} else if i+1 < len(args) {
				parsed.ConfigPath = args[i+1]
				i++
			}
		case "--external-dir":
			if hasInlineValue {
				parsed.ExternalDirs = append(parsed.ExternalDirs, value)
			} else if i+1 < len(args) {
				parsed.ExternalDirs = append(parsed.ExternalDirs, args[i+1])
				i++
			}
		default:
			if option.Arity == cli.RequiredValue && !hasInlineValue && i+1 < len(args) {
				i++
			}
		}
	}
	return parsed
}

func newParsedOptionFlagSet(name string, output io.Writer) (*flag.FlagSet, *parsedOptions, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(output)
	parsed := &parsedOptions{}
	bindings := parsed.optionBindings()
	for _, option := range cli.Options() {
		if option.TaskFlag {
			continue
		}
		bind, ok := bindings[option.Canonical]
		if !ok {
			return nil, nil, fmt.Errorf("missing parser binding for %s", option.Canonical)
		}
		bind(flags, flagName(option.Canonical))
		for _, alias := range option.Aliases {
			bind(flags, flagName(alias))
		}
	}
	return flags, parsed, nil
}

func (p *parsedOptions) optionBindings() map[string]optionBinder {
	bindBool := func(dst *bool, usage string) optionBinder {
		return func(flags *flag.FlagSet, name string) {
			flags.BoolVar(dst, name, false, usage)
		}
	}
	bindString := func(dst *string, usage string) optionBinder {
		return func(flags *flag.FlagSet, name string) {
			flags.StringVar(dst, name, "", usage)
		}
	}
	return map[string]optionBinder{
		"--color":                bindBool(&p.Color, "colorize diagnostic output"),
		"--no-color":             bindBool(&p.NoColor, "disable colorized diagnostic output"),
		"--config":               bindString(&p.ConfigPath, "load configuration from file"),
		"--debug":                bindBool(&p.Debug, "write debug logs"),
		"--disable":              p.bindDisable,
		"--external-dir":         p.bindExternalDir,
		"--force-dot-resolution": bindBool(&p.ForceDotResolution, "merge dotted facts into structured facts"),
		"--hocon":                bindBool(&p.HOCON, "render facts as HOCON"),
		"--json":                 bindBool(&p.JSON, "render facts as JSON"),
		"--log-level":            bindString(&p.LogLevel, "accepted for Facter compatibility"),
		"--no-block":             bindBool(&p.NoBlock, "disable fact blocking"),
		"--no-cache":             bindBool(&p.NoCache, "disable loading and refreshing facts from the cache"),
		"--no-external-facts":    bindBool(&p.NoExternalFacts, "accepted for Facter compatibility"),
		"--no-hocon":             bindBool(&p.NoHOCON, "do not render facts as HOCON"),
		"--no-json":              bindBool(&p.NoJSON, "do not render facts as JSON"),
		"--no-yaml":              bindBool(&p.NoYAML, "do not render facts as YAML"),
		"--verbose":              bindBool(&p.Verbose, "write info logs"),
		"--yaml":                 bindBool(&p.YAML, "render facts as YAML"),
		"--strict":               bindBool(&p.Strict, "return an error when queried facts are missing"),
		"--timing":               bindBool(&p.Timing, "write fact timing"),
		"--sequential":           bindBool(&p.Sequential, "accepted for Facter compatibility"),
		"--http-debug":           bindBool(&p.HTTPDebug, "accepted for Facter compatibility"),
	}
}

func (p *parsedOptions) bindExternalDir(flags *flag.FlagSet, name string) {
	flags.Func(name, "load external facts from directory", func(value string) error {
		p.ExternalDirs = append(p.ExternalDirs, value)
		return nil
	})
}

func (p *parsedOptions) bindDisable(flags *flag.FlagSet, name string) {
	flags.Func(name, "disable facts or fact groups (comma-separated, repeatable)", func(value string) error {
		p.DisableEntries = append(p.DisableEntries, engine.SplitDisableList(value)...)
		return nil
	})
}

func flagName(option string) string {
	return strings.TrimLeft(option, "-")
}

func inlineOptionValue(arg string) (string, bool) {
	_, value, ok := strings.Cut(arg, "=")
	return value, ok
}
