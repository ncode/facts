package app

import (
	"context"
	"errors"
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
	options := parseListOptions(args)
	groups := engine.BuiltinFactGroups()
	config, err := engine.ParseConfig(options.ConfigPath, slog.New(slog.DiscardHandler))
	if err != nil {
		return nil, err
	}
	groups = engine.MergeFactGroups(groups, config.FactGroups)
	config.NoExternalFacts = false
	configuredExternalDirs := engine.DiscoveryExternalDirs(config, options.ExternalDirs, false, false, nil)
	var defaultExternalDirs []string
	if len(configuredExternalDirs) == 0 {
		defaultExternalDirs = defaultExternalFactDirs()
	}
	externalDirs := engine.DiscoveryExternalDirs(config, options.ExternalDirs, false, true, defaultExternalDirs)
	external, err := engine.ExternalFactGroups(externalDirs)
	if err != nil {
		return nil, err
	}
	return engine.MergeFactGroups(groups, external), nil
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
	flags, options, err := parseOptions("query", stderr, args)
	if err != nil {
		return err
	}
	colorOutput := resolveColor(options.Color, options.NoColor, stdout)
	colorDiagnostics := resolveColor(options.Color, options.NoColor, stderr)
	// One diagnostics sink for the whole run: engine diagnostics (config, cache,
	// collection, …) and CLI diagnostics share this handler. debug/verbose are
	// set once resolved below; warn-class (the only level ParseConfig emits) is
	// always enabled, so config diagnostics render identically before that.
	logHandler := &stderrLogHandler{stderr: stderr, color: colorDiagnostics}
	logger := slog.New(logHandler)
	configFile := options.ConfigPath
	configOptions, configErr := engine.ParseConfig(configFile, logger)
	if configErr != nil {
		return configErr
	}
	cliExternalDirs := options.ExternalDirs
	configForConflict := configOptions
	configForConflict.NoExternalFacts = false
	conflictExternalDirs := engine.DiscoveryExternalDirs(configForConflict, options.ExternalDirs, false, false, nil)
	if configOptions.NoExternalFacts {
		options.NoExternalFacts = true
	}
	if configOptions.Debug {
		options.Debug = true
	}
	if configOptions.Verbose {
		options.Verbose = true
	}
	if options.NoExternalFacts && hasNonEmpty(conflictExternalDirs) {
		return optionError(stdout, errors.New("--no-external-facts and --external-dir options conflict: please specify only one"))
	}
	var defaultExternalDirs []string
	if !options.NoExternalFacts && len(conflictExternalDirs) == 0 {
		defaultExternalDirs = defaultExternalFactDirs()
	}
	discoveryExternalDirs := engine.DiscoveryExternalDirs(configOptions, options.ExternalDirs, options.NoExternalFacts, true, defaultExternalDirs)
	disabledFactsForFastPath := map[string]bool{}
	var disabledFacts map[string]bool
	if options.NoBlock {
		// --no-block is the master override: an empty non-nil map clears the
		// whole disabled set for this run, including --disable and FACTS_DISABLE.
		disabledFacts = map[string]bool{}
	}
	if !options.NoBlock {
		// The fast path honors every disable source so a disabled facterversion
		// query falls through to normal resolution (and disable-beats-query). It
		// derives its set from the same engine union discovery planning uses, so
		// the two can never disagree on whether facterversion is disabled.
		disabledFactsForFastPath = engine.DisabledUnion(configOptions, options.DisableEntries, os.Environ())
	}
	mergeDottedFacts := configOptions.ForceDotResolution || options.ForceDotResolution
	logLevel := firstNonEmpty(options.LogLevel, configOptions.LogLevel)
	if logLevel != "" && !cli.SupportedLogLevel(logLevel) {
		return optionError(stdout, fmt.Errorf("unsupported log level %s", logLevel))
	}
	debugEnabled := options.Debug
	verboseEnabled := options.Verbose
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
	if canUseVersionQueryFastPath(flags.Args(), discoveryExternalDirs, disabledFactsForFastPath, options.NoExternalFacts, options.Timing) {
		return writeVersionQuery(stdout, options.JSON, options.YAML, options.HOCON)
	}
	resolutionStart := time.Now()

	eng, err := engine.NewEngine(engine.EngineConfig{
		CLICompat:              true,
		SystemDefaults:         true,
		ConfigFile:             configFile,
		ConfigLoaded:           true,
		Config:                 configOptions,
		ExternalDirs:           cliExternalDirs,
		UseCache:               !options.NoCache,
		NoExternalFacts:        options.NoExternalFacts,
		DisabledFacts:          disabledFacts,
		ExtraDisabled:          options.DisableEntries,
		DefaultExternalDirsSet: true,
		DefaultExternalDirs:    defaultExternalDirs,
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
	presentation := snapshot.OutputProjection(mergeDottedFacts)
	resolutionDuration := time.Since(resolutionStart).Seconds()
	out, err := engine.BuildFormatter(engine.FormatOptions{
		JSON:     options.JSON,
		YAML:     options.YAML,
		HOCON:    options.HOCON,
		Colorize: colorOutput,
	})(presentation)
	if err != nil {
		return err
	}
	if options.Timing {
		for _, name := range presentation.PresentationNames() {
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
	if options.Strict {
		missing := presentation.MissingQueries()
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
	presentation := engine.NewProjection([]engine.ResolvedFact{{Name: "facterversion", Value: engine.Version, UserQuery: "facterversion"}}, false)
	// The fast path carries only the three format booleans: it deliberately
	// ignores --color and --force-dot-resolution, so it uses an uncolored,
	// non-force-dot Projection and the engine's own formatter precedence.
	out, err := engine.BuildFormatter(engine.FormatOptions{
		JSON:  jsonOutput,
		YAML:  yamlOutput,
		HOCON: hoconOutput,
	})(presentation)
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
