package engine

import (
	"slices"
)

type discoveryPlan struct {
	externalDirs       []string
	noExternalFacts    bool
	disabledFacts      map[string]bool
	ambientDisabled    map[string]string
	useCache           bool
	cacheTTLs          []FactTTL
	cacheGroups        []FactGroup
	loaderMode         externalFactLoaderMode
	includeEnv         bool
	queries            []string
	includeTypedDotted bool
}

func (e *Engine) planDiscovery(s *Session, queries []string) (discoveryPlan, []error) {
	plan := discoveryPlan{
		externalDirs:       slices.Clone(e.cfg.ExternalDirs),
		noExternalFacts:    e.cfg.NoExternalFacts,
		disabledFacts:      cloneBoolMap(e.cfg.DisabledFacts),
		useCache:           e.cfg.UseCache,
		loaderMode:         externalFactLoaderLibrary,
		includeEnv:         e.cfg.SystemDefaults,
		queries:            slices.Clone(queries),
		includeTypedDotted: e.cfg.IncludeTypedDotted,
	}
	if e.cfg.CLICompat {
		plan.loaderMode = externalFactLoaderCLI
		plan.includeEnv = true
	}

	var failures []error
	config, ok, err := e.configForDiscovery(s)
	if err != nil {
		failures = append(failures, err)
	} else if ok {
		plan.noExternalFacts = plan.noExternalFacts || config.NoExternalFacts
		plan.cacheGroups = cloneFactGroups(config.FactGroups)
		if plan.useCache {
			plan.cacheTTLs = slices.Clone(config.TTLs)
		}
		if e.cfg.CLICompat && config.ForceDotResolution {
			plan.includeTypedDotted = true
		}
	}
	// A non-nil DisabledFacts override is used verbatim (this is how --no-block
	// clears everything and how a library consumer forces an explicit set);
	// otherwise the disabled set is the union of config, FACTS_DISABLE, and the
	// --disable CLI entries, each expanded through the configured fact groups.
	if e.cfg.DisabledFacts == nil {
		plan.disabledFacts, plan.ambientDisabled = e.unionDisabledFacts(s, config, plan.includeEnv)
	}
	if plan.disabledFacts == nil {
		plan.disabledFacts = map[string]bool{}
	}
	var defaultExternalDirs []string
	systemDefaults := e.cfg.SystemDefaults && !plan.noExternalFacts
	if systemDefaults && len(plan.externalDirs) == 0 && len(config.ExternalDirs) == 0 {
		defaultExternalDirs = e.defaultExternalDirs()
	}
	configForDirs := config
	configForDirs.NoExternalFacts = false
	plan.externalDirs = DiscoveryExternalDirs(configForDirs, plan.externalDirs, false, systemDefaults, defaultExternalDirs)
	return plan, failures
}

// DiscoveryExternalDirs returns the external fact directories discovery would
// load for the same inputs: explicit directories first, then config
// directories, then process defaults when system defaults are enabled.
func DiscoveryExternalDirs(config Config, explicit []string, noExternalFacts, systemDefaults bool, defaults []string) []string {
	if noExternalFacts || config.NoExternalFacts {
		return nil
	}
	if len(explicit) > 0 {
		return slices.Clone(explicit)
	}
	if len(config.ExternalDirs) > 0 {
		return slices.Clone(config.ExternalDirs)
	}
	if systemDefaults {
		return slices.Clone(defaults)
	}
	return nil
}

// disabledSourceEnv and disabledSourceConfig label the ambient (non-CLI)
// sources of a disabled fact for the explicit-query diagnostic.
const (
	disabledSourceEnv    = "FACTS_DISABLE"
	disabledSourceConfig = "the configuration file"
)

// unionDisabledFacts builds the disabled set as the union of the config
// `disable`/`blocklist` list, the FACTS_DISABLE environment control (only when
// includeEnv is set, consistent with environment facts), and the --disable CLI
// entries, expanding each source through the configured fact groups. It also
// returns the ambient map naming the env/config source of each disabled fact
// that is not also named on the command line, for the explicit-query
// diagnostic.
func (e *Engine) unionDisabledFacts(s *Session, config Config, includeEnv bool) (map[string]bool, map[string]string) {
	groups := config.FactGroups
	var environ []string
	if includeEnv {
		environ = s.host.environ()
	}
	// The disabled set is derived from the exported union so a full discovery
	// and the version fast path can never disagree on whether a fact is disabled.
	disabled := DisabledUnion(config, e.cfg.ExtraDisabled, environ)

	// The ambient map names the env/config source of each disabled fact for the
	// explicit-query diagnostic; it mirrors the union's sources but is engine-only.
	ambient := map[string]string{}
	for name := range DisabledFactsWithGroups(config.Disabled, groups) {
		ambient[name] = disabledSourceConfig
	}
	if includeEnv {
		for name := range DisabledFactsWithGroups(environmentDisabledFacts(s.host.environ()), groups) {
			ambient[name] = disabledSourceEnv
		}
	}
	// --disable entries are "on the same command line", so they join the
	// disabled set but never trigger the ambient diagnostic; a name they cover
	// is dropped from the ambient map even if config/env also disabled it. The
	// drop also covers descendants, mirroring pruneDisabledDescendants: a
	// --disable of `networking` silences an ambient `networking.ip` it subsumes.
	for name := range DisabledFactsWithGroups(e.cfg.ExtraDisabled, groups) {
		delete(ambient, name)
		for k := range ambient {
			if factHierarchyCovers(name, k) {
				delete(ambient, k)
			}
		}
	}
	if len(ambient) == 0 {
		ambient = nil
	}
	return disabled, ambient
}

func (e *Engine) configForDiscovery(s *Session) (Config, bool, error) {
	if e.cfg.ConfigLoaded {
		return cloneConfig(e.cfg.Config), true, nil
	}
	if e.cfg.ConfigFile == "" && !e.cfg.SystemDefaults {
		return Config{}, false, nil
	}
	config, err := ParseConfig(e.cfg.ConfigFile, s.logger)
	return config, true, err
}

func (e *Engine) defaultExternalDirs() []string {
	if e.cfg.DefaultExternalDirsSet {
		return slices.Clone(e.cfg.DefaultExternalDirs)
	}
	return CurrentDefaultExternalFactDirs()
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneConfig(in Config) Config {
	in.Disabled = slices.Clone(in.Disabled)
	in.ExternalDirs = slices.Clone(in.ExternalDirs)
	in.TTLs = slices.Clone(in.TTLs)
	in.FactGroups = cloneFactGroups(in.FactGroups)
	return in
}

func cloneFactGroups(in []FactGroup) []FactGroup {
	if in == nil {
		return nil
	}
	out := make([]FactGroup, len(in))
	for i, group := range in {
		out[i] = FactGroup{Name: group.Name, Facts: slices.Clone(group.Facts)}
	}
	return out
}
