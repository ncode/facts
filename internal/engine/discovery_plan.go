package engine

import "slices"

type discoveryPlan struct {
	externalDirs       []string
	noExternalFacts    bool
	disabledFacts      map[string]bool
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
		if len(plan.externalDirs) == 0 {
			plan.externalDirs = slices.Clone(config.ExternalDirs)
		}
		plan.noExternalFacts = plan.noExternalFacts || config.NoExternalFacts
		plan.cacheGroups = cloneFactGroups(config.FactGroups)
		if e.cfg.DisabledFacts == nil {
			plan.disabledFacts = DisabledFactsForFiltering(config.Disabled, config.FactGroups)
		}
		if plan.useCache {
			plan.cacheTTLs = slices.Clone(config.TTLs)
		}
		if e.cfg.CLICompat && config.ForceDotResolution {
			plan.includeTypedDotted = true
		}
	}
	if plan.disabledFacts == nil {
		plan.disabledFacts = map[string]bool{}
	}
	if !plan.noExternalFacts && len(plan.externalDirs) == 0 && e.cfg.SystemDefaults {
		plan.externalDirs = e.defaultExternalDirs()
	}
	return plan, failures
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
