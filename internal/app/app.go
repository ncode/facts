package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/ncode/facts/internal/cli"
	"github.com/ncode/facts/internal/engine"
)

var defaultExternalFactDirs = engine.CurrentDefaultExternalFactDirs

// Run executes the facts command with the provided arguments.
func Run(stdout, stderr io.Writer, args []string) error {
	args = cli.PrepareArguments(args)
	if err := cli.ValidateOptions(args); err != nil {
		return optionError(stdout, err)
	}
	if len(args) == 0 {
		args = []string{"query"}
	}

	switch args[0] {
	case "help", "-h", "--help":
		_, err := fmt.Fprint(stdout, helpText())
		return err
	case "man", "--man":
		_, err := fmt.Fprint(stdout, manText())
		return err
	case "version", "--version", "-v":
		_, err := fmt.Fprintln(stdout, engine.Version)
		return err
	case "list_block_groups", "--list-block-groups":
		groups, err := factGroups(args[1:])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, engine.FormatFactGroups(groups))
		return err
	case "list_cache_groups", "--list-cache-groups":
		groups, err := factGroups(args[1:])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, engine.FormatFactGroups(groups))
		return err
	case "query":
		return runQuery(stdout, stderr, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func factGroups(args []string) ([]engine.FactGroup, error) {
	groups := engine.BuiltinFactGroups()
	configPath := configPathFromArgs(args)
	config, err := engine.ParseConfig(configPath, slog.New(slog.DiscardHandler))
	if err != nil {
		return nil, err
	}
	groups = engine.MergeFactGroups(groups, config.FactGroups)
	externalDirs := externalDirsFromArgs(args)
	if len(externalDirs) == 0 {
		externalDirs = config.ExternalDirs
	}
	externalDirs = effectiveExternalDirs(externalDirs)
	external, err := engine.ExternalFactGroups(externalDirs)
	if err != nil {
		return nil, err
	}
	return engine.MergeFactGroups(groups, external), nil
}

func effectiveExternalDirs(explicit []string) []string {
	if len(explicit) > 0 {
		return explicit
	}
	return defaultExternalFactDirs()
}

func configPathFromArgs(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		option, ok := cli.LookupOption(arg)
		if !ok {
			continue
		}
		if value, hasInlineValue := inlineOptionValue(arg); hasInlineValue {
			if option.Canonical == "--config" {
				return value
			}
			continue
		}
		if option.Canonical == "--config" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if option.Arity == cli.RequiredValue && i+1 < len(args) {
			i++
		}
	}
	return ""
}

func externalDirsFromArgs(args []string) []string {
	dirs := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		option, ok := cli.LookupOption(arg)
		if !ok {
			continue
		}
		if value, hasInlineValue := inlineOptionValue(arg); hasInlineValue {
			if option.Canonical == "--external-dir" {
				dirs = append(dirs, value)
			}
			continue
		}
		if option.Canonical == "--external-dir" {
			if i+1 < len(args) {
				dirs = append(dirs, args[i+1])
				i++
			}
			continue
		}
		if option.Arity == cli.RequiredValue && i+1 < len(args) {
			i++
		}
	}
	return dirs
}

func inlineOptionValue(arg string) (string, bool) {
	_, value, ok := strings.Cut(arg, "=")
	return value, ok
}

func helpText() string {
	var b strings.Builder
	b.WriteString(`Usage
=====

facts [options] [query] [query] [...]

Options
=======
`)
	for _, option := range cli.DocumentedOptions() {
		b.WriteString(option.Documentation.Help)
		b.WriteByte('\n')
	}
	return b.String()
}

func manText() string {
	var b strings.Builder
	b.WriteString(`facts - collect and display facts about the current system
==========================================================

SYNOPSIS
--------
facts [options] [query] [query] [...]

DESCRIPTION
-----------
facts is a command-line tool that gathers basic facts about nodes (systems) such as hardware details, network settings, OS type and version, and more. These facts are made available as variables in your Puppet manifests and can be used to inform conditional expressions in Puppet.

If no queries are given, then all facts will be returned.

Many command line options can also be set via the HOCON config file. This file can also be used to block or cache certain fact groups.

OPTIONS
-------
`)
	for _, option := range cli.DocumentedOptions() {
		b.WriteString(option.Documentation.Man)
		b.WriteByte('\n')
	}
	b.WriteString(`

FILES
-----
/etc/facts/facts.conf

The facts-native config file, consulted first.

/etc/puppetlabs/facter/facter.conf

The facter-compatible config file, read when no facts-native config file exists. Both are parsed with identical semantics.

EXAMPLES
--------
Display all facts:

    facts

Display a single structured fact:

    facts processors

Format facts as JSON:

    facts --json os.name os.release.major processors.isa
`)
	return b.String()
}

func runQuery(stdout, stderr io.Writer, args []string) error {
	flags := flag.NewFlagSet("query", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "render facts as JSON")
	jsonOutputShort := flags.Bool("j", false, "render facts as JSON")
	flags.Bool("no-json", false, "do not render facts as JSON")
	yamlOutput := flags.Bool("yaml", false, "render facts as YAML")
	yamlOutputShort := flags.Bool("y", false, "render facts as YAML")
	flags.Bool("no-yaml", false, "do not render facts as YAML")
	hoconOutput := flags.Bool("hocon", false, "render facts as HOCON")
	flags.Bool("no-hocon", false, "do not render facts as HOCON")
	debug := flags.Bool("debug", false, "write debug logs")
	debugShort := flags.Bool("d", false, "write debug logs")
	color := flags.Bool("color", false, "colorize diagnostic output")
	noColor := flags.Bool("no-color", false, "disable colorized diagnostic output")
	flags.String("log-level", "", "accepted for Facter compatibility")
	flags.String("l", "", "accepted for Facter compatibility")
	timing := flags.Bool("timing", false, "write fact timing")
	timingShort := flags.Bool("t", false, "write fact timing")
	strict := flags.Bool("strict", false, "return an error when queried facts are missing")
	forceDotResolution := flags.Bool("force-dot-resolution", false, "merge dotted facts into structured facts")
	configPath := flags.String("config", "", "load configuration from file")
	configPathShort := flags.String("c", "", "load configuration from file")
	noBlock := flags.Bool("no-block", false, "disable fact blocking")
	noCache := flags.Bool("no-cache", false, "disable loading and refreshing facts from the cache")
	verbose := flags.Bool("verbose", false, "write info logs")
	flags.Bool("sequential", false, "accepted for Facter compatibility")
	flags.Bool("http-debug", false, "accepted for Facter compatibility")
	noExternalFacts := flags.Bool("no-external-facts", false, "accepted for Facter compatibility")
	var externalDirs []string
	flags.Func("external-dir", "load external facts from directory", func(value string) error {
		externalDirs = append(externalDirs, value)
		return nil
	})
	var disableEntries []string
	flags.Func("disable", "disable facts or fact groups (comma-separated, repeatable)", func(value string) error {
		disableEntries = append(disableEntries, engine.SplitDisableList(value)...)
		return nil
	})
	if err := flags.Parse(args); err != nil {
		return err
	}
	colorOutput := resolveColor(*color, *noColor, stdout)
	colorDiagnostics := resolveColor(*color, *noColor, stderr)
	// One diagnostics sink for the whole run: engine diagnostics (config, cache,
	// collection, …) and CLI diagnostics share this handler. debug/verbose are
	// set once resolved below; warn-class (the only level ParseConfig emits) is
	// always enabled, so config diagnostics render identically before that.
	logHandler := &stderrLogHandler{stderr: stderr, color: colorDiagnostics}
	logger := slog.New(logHandler)
	configFile := firstNonEmpty(*configPath, *configPathShort)
	configOptions, configErr := engine.ParseConfig(configFile, logger)
	if configErr != nil {
		return configErr
	}
	cliExternalDirs := externalDirs
	discoveryExternalDirs := externalDirs
	if len(discoveryExternalDirs) == 0 {
		discoveryExternalDirs = configOptions.ExternalDirs
	}
	if configOptions.NoExternalFacts {
		*noExternalFacts = true
	}
	if configOptions.Debug {
		*debug = true
	}
	if configOptions.Verbose {
		*verbose = true
	}
	if *noExternalFacts && hasNonEmpty(discoveryExternalDirs) {
		return optionError(stdout, errors.New("--no-external-facts and --external-dir options conflict: please specify only one"))
	}
	if !*noExternalFacts {
		discoveryExternalDirs = effectiveExternalDirs(discoveryExternalDirs)
	}
	disabledFactsForFastPath := map[string]bool{}
	var disabledFacts map[string]bool
	if *noBlock {
		// --no-block is the master override: an empty non-nil map clears the
		// whole disabled set for this run, including --disable and FACTS_DISABLE.
		disabledFacts = map[string]bool{}
	}
	if !*noBlock {
		// The fast path must honor every disable source so a disabled
		// facterversion query falls through to normal resolution (and
		// disable-beats-query), mirroring the engine's union.
		fastPathEntries := append([]string(nil), configOptions.Disabled...)
		fastPathEntries = append(fastPathEntries, disableEntries...)
		fastPathEntries = append(fastPathEntries, engine.EnvironmentDisabledFacts(os.Environ())...)
		disabledFactsForFastPath = engine.DisabledFactsForFiltering(fastPathEntries, configOptions.FactGroups)
	}
	mergeDottedFacts := configOptions.ForceDotResolution || *forceDotResolution
	logLevel := firstNonEmpty(flags.Lookup("log-level").Value.String(), flags.Lookup("l").Value.String(), configOptions.LogLevel)
	if logLevel != "" && !cli.SupportedLogLevel(logLevel) {
		return optionError(stdout, fmt.Errorf("unsupported log level %s", logLevel))
	}
	debugEnabled := *debug || *debugShort
	verboseEnabled := *verbose
	if resolvedLogOptionsConflict(debugEnabled, verboseEnabled, logLevel) {
		return optionError(stdout, errors.New("debug, verbose, and log-level options conflict: please specify only one."))
	}
	debugDiagnostics := debugEnabled || logLevelEnablesDebug(logLevel)
	logHandler.debug = debugDiagnostics
	logHandler.verbose = verboseEnabled || strings.EqualFold(logLevel, "info")
	if debugDiagnostics {
		writeDebug(stderr, "resolving facts", colorDiagnostics)
	} else if verboseEnabled || strings.EqualFold(logLevel, "info") {
		writeInfo(stderr, "executed with command line: "+strings.Join(args, " "), colorDiagnostics)
		writeInfo(stderr, "resolving facts", colorDiagnostics)
	}
	if canUseVersionQueryFastPath(flags.Args(), discoveryExternalDirs, disabledFactsForFastPath, *noExternalFacts, *timing || *timingShort) {
		return writeVersionQuery(stdout, *jsonOutput || *jsonOutputShort, *yamlOutput || *yamlOutputShort, *hoconOutput)
	}
	resolutionStart := time.Now()

	eng, err := engine.NewEngine(engine.EngineConfig{
		CLICompat:              true,
		SystemDefaults:         true,
		ConfigFile:             configFile,
		ConfigLoaded:           true,
		Config:                 configOptions,
		ExternalDirs:           cliExternalDirs,
		UseCache:               !*noCache,
		NoExternalFacts:        *noExternalFacts,
		DisabledFacts:          disabledFacts,
		ExtraDisabled:          disableEntries,
		DefaultExternalDirsSet: true,
		DefaultExternalDirs:    defaultExternalFactDirs(),
		IncludeTypedDotted:     mergeDottedFacts,
		Logger:                 logger,
	})
	if err != nil {
		return err
	}
	snapshot, err := eng.Discover(context.Background(), flags.Args()...)
	if err != nil {
		return err
	}
	facts := snapshot.Facts()
	resolutionDuration := time.Since(resolutionStart).Seconds()
	out, err := engine.BuildFormatter(engine.FormatOptions{
		JSON:               *jsonOutput || *jsonOutputShort,
		YAML:               *yamlOutput || *yamlOutputShort,
		HOCON:              *hoconOutput,
		IncludeTypedDotted: mergeDottedFacts,
		Colorize:           colorOutput,
	}).Format(facts)
	if err != nil {
		return err
	}
	if *timing || *timingShort {
		for _, fact := range facts {
			name := fact.UserQuery
			if name == "" {
				name = fact.Name
			}
			if _, err := fmt.Fprintf(stdout, "fact '%s', took: (%.3f) seconds\n", name, resolutionDuration); err != nil {
				return err
			}
		}
	}
	if out != "" {
		if strings.HasSuffix(out, "\n") {
			_, err = fmt.Fprint(stdout, out)
		} else {
			_, err = fmt.Fprintln(stdout, out)
		}
		if err != nil {
			return err
		}
	}
	if *strict {
		missing := engine.NewProjection(facts, mergeDottedFacts).MissingQueries(facts)
		if len(missing) > 0 {
			for _, name := range missing {
				writeError(stderr, fmt.Sprintf("fact %q does not exist.", name), colorDiagnostics)
			}
			return ExitStatus(1)
		}
	}
	return nil
}

// ExitStatus reports a command status that should not be printed as an error.
type ExitStatus int

func (s ExitStatus) Error() string {
	return fmt.Sprintf("exit status %d", int(s))
}

func (s ExitStatus) Code() int {
	return int(s)
}

func optionError(stdout io.Writer, err error) error {
	if _, writeErr := fmt.Fprint(stdout, helpText()); writeErr != nil {
		return writeErr
	}
	return err
}

// resolveColor decides whether output written to w is colorized: --no-color
// always disables, --color always enables, and otherwise color is on exactly
// when w is a terminal, so piped and redirected output stays clean.
func resolveColor(force, disable bool, w io.Writer) bool {
	if disable {
		return false
	}
	if force {
		return true
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func writeDebug(w io.Writer, message string, color bool) {
	if color {
		fmt.Fprintf(w, "\x1b[36mDEBUG Facts - %s\x1b[0m\n", message)
		return
	}
	fmt.Fprintf(w, "DEBUG Facts - %s\n", message)
}

func writeInfo(w io.Writer, message string, color bool) {
	if color {
		fmt.Fprintf(w, "\x1b[32mINFO Facts - %s\x1b[0m\n", message)
		return
	}
	fmt.Fprintf(w, "INFO Facts - %s\n", message)
}

func writeError(w io.Writer, message string, color bool) {
	if color {
		fmt.Fprintf(w, "\x1b[31mERROR Facts - %s\x1b[0m\n", message)
		return
	}
	fmt.Fprintf(w, "ERROR Facts - %s\n", message)
}

func writeWarn(w io.Writer, message string, color bool) {
	if color {
		fmt.Fprintf(w, "\x1b[33mWARN Facts - %s\x1b[0m\n", message)
		return
	}
	fmt.Fprintf(w, "WARN Facts - %s\n", message)
}

func resolvedLogOptionsConflict(debug, verbose bool, logLevel string) bool {
	if debug && verbose {
		return true
	}
	if logLevel == "" || strings.EqualFold(logLevel, "log_level") {
		return false
	}
	if debug && (strings.EqualFold(logLevel, "debug") || strings.EqualFold(logLevel, "trace")) {
		return false
	}
	if verbose && strings.EqualFold(logLevel, "info") {
		return false
	}
	return (debug || verbose) && logLevel != ""
}

func canUseVersionQueryFastPath(queries, externalDirs []string, disabledFacts map[string]bool, noExternalFacts, timing bool) bool {
	if len(queries) != 1 || queries[0] != "facterversion" || timing || disabledFacts["facterversion"] {
		return false
	}
	if !noExternalFacts && len(externalDirs) > 0 {
		return false
	}
	return true
}

func writeVersionQuery(stdout io.Writer, jsonOutput, yamlOutput, hoconOutput bool) error {
	facts := []engine.ResolvedFact{{Name: "facterversion", Value: engine.Version, UserQuery: "facterversion"}}
	var (
		out string
		err error
	)
	if jsonOutput {
		out, err = engine.FormatJSON(facts)
	} else if yamlOutput {
		out = engine.FormatYAML(facts)
	} else if hoconOutput {
		out = engine.FormatHOCON(facts)
	} else {
		out = engine.FormatLegacy(facts)
	}
	if err != nil {
		return err
	}
	if strings.HasSuffix(out, "\n") {
		_, err = fmt.Fprint(stdout, out)
		return err
	}
	_, err = fmt.Fprintln(stdout, out)
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func hasNonEmpty(values []string) bool {
	for _, value := range values {
		if value != "" {
			return true
		}
	}
	return false
}

func logLevelEnablesDebug(level string) bool {
	switch strings.ToLower(level) {
	case "debug", "trace":
		return true
	default:
		return false
	}
}
