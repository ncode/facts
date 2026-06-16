package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// ProgrammaticFact is a fact registered on an Engine at construction.
// A nil Resolve registers a fact that resolves to nil. The resolved value is
// normalized like every registered fact value (times to RFC 3339, string-keyed
// maps to map[string]any); values containing null bytes are rejected with a
// warning and resolve to nil.
type ProgrammaticFact struct {
	Name    string
	Resolve func(ctx context.Context) (any, error)
}

// EngineConfig is the configuration an Engine is frozen from. The zero value
// is the hermetic default: core facts only — no config file, no fact
// directories, no script execution, no environment facts, no persistent
// cache.
type EngineConfig struct {
	// ConfigFile opts into reading the given facter.conf, honoring its
	// global.external-dir, no-external-facts toggle, fact blocklists,
	// and cache TTLs with CLI-identical semantics.
	ConfigFile string
	// ExternalDirs opts into loading external facts from exactly these
	// directories (no environment variables, no default directories).
	ExternalDirs []string
	// UseCache opts into the persistent fact cache with facter.conf TTL
	// semantics.
	UseCache bool
	// SystemDefaults selects full CLI-equivalent system-following behavior:
	// the default config file (facts.conf first, facter.conf second),
	// default external fact directories (facts-native locations first), and
	// FACTS_*/FACTER_* environment facts (FACTS_* wins name collisions).
	SystemDefaults bool
	// Logger receives engine diagnostics; nil discards them.
	Logger *slog.Logger
	// Facts are registered facts, fixed at construction.
	Facts []ProgrammaticFact

	// The remaining fields are CLI-only knobs set by internal/app; the public
	// facts package exposes no options for them.

	// CLICompat selects the CLI's loader and error semantics: FACTS_*/FACTER_*
	// environment facts whenever external facts are on, silent skips for
	// failing external executables, and fail-fast on the first hard source
	// error.
	CLICompat bool
	// NoExternalFacts skips external-fact loading (--no-external-facts).
	NoExternalFacts bool
	// Puppet appends Puppet facts to the core set (--puppet).
	Puppet bool
	// BlockedFacts overrides the config-derived blocklist when non-nil.
	BlockedFacts map[string]bool
}

// Engine is an immutable unit of fact-discovery configuration. All
// registration happens at construction; resolution happens only through
// Discover, and concurrent use is safe because nothing mutates after
// NewEngine except the once-only diagnostic dedup set, which has its own
// lock.
type Engine struct {
	cfg    EngineConfig
	logger *slog.Logger

	onceMu   sync.Mutex
	onceSeen map[string]bool
}

// NewEngine validates and freezes cfg into an Engine.
func NewEngine(cfg EngineConfig) (*Engine, error) {
	cfg.ExternalDirs = slices.Clone(cfg.ExternalDirs)
	cfg.Facts = slices.Clone(cfg.Facts)
	for i, fact := range cfg.Facts {
		if fact.Name == "" {
			return nil, fmt.Errorf("fact %d: name is empty", i)
		}
		if strings.ContainsRune(fact.Name, '\x00') {
			return nil, fmt.Errorf("fact %q: name contains a null byte", fact.Name)
		}
		cfg.Facts[i].Name = strings.ToLower(fact.Name)
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Engine{cfg: cfg, logger: logger, onceSeen: map[string]bool{}}, nil
}

// warnOnce emits message at warn level the first time it is seen on this
// Engine, within and across discoveries.
func (e *Engine) warnOnce(message string) {
	e.onceMu.Lock()
	seen := e.onceSeen[message]
	e.onceSeen[message] = true
	e.onceMu.Unlock()
	if !seen {
		e.logger.Warn(message)
	}
}

// Discover runs the configured resolvers and returns an immutable Snapshot of
// the canonical tree. Query matching follows the CLI's dot-notation
// semantics.
//
// The Snapshot is valid even when err != nil: discovery failures are partial,
// and err is the join of every per-source failure (including ctx.Err() when
// the context ends discovery early). Not-applicable facts are absent, never
// errors.
func (e *Engine) Discover(ctx context.Context, queries ...string) (*Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	s := NewSessionContext(ctx)
	s.logger = e.logger

	var failures []error
	var facts []ResolvedFact

	externalDirs := slices.Clone(e.cfg.ExternalDirs)
	blocked := e.cfg.BlockedFacts
	if blocked == nil {
		blocked = map[string]bool{}
	}
	var ttls []FactTTL
	var cacheGroups []FactGroup
	noExternalFacts := e.cfg.NoExternalFacts

	if e.cfg.ConfigFile != "" || e.cfg.SystemDefaults {
		configFile := e.cfg.ConfigFile
		config, err := ParseConfig(configFile)
		if err != nil {
			failures = append(failures, err)
		} else {
			if len(externalDirs) == 0 {
				externalDirs = config.ExternalDirs
			}
			noExternalFacts = noExternalFacts || config.NoExternalFacts
			cacheGroups = config.FactGroups
			if e.cfg.BlockedFacts == nil {
				blocked = BlocklistedFactsForFiltering(config.Blocklist, config.FactGroups)
			}
			if e.cfg.UseCache {
				ttls = config.TTLs
			}
		}
	}

	finish := func() (*Snapshot, error) {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
		}
		return newSnapshot(facts), errors.Join(failures...)
	}

	var externalFacts []ResolvedFact
	if !noExternalFacts {
		dirs := externalDirs
		if e.cfg.SystemDefaults && !e.cfg.CLICompat && len(dirs) == 0 {
			dirs = CurrentDefaultExternalFactDirs()
		}
		if len(dirs) > 0 && ExternalFactResolutionRunning() {
			e.warnOnce("Recursion detected while resolving external facts; executable external facts will be skipped")
		}
		loader := externalFactLoader{
			s:       s,
			dirs:    dirs,
			blocked: blocked,
		}
		if e.cfg.CLICompat {
			loader.mode = externalFactLoaderCLI
			loader.includeEnv = true
			loaded, err := loader.load()
			if err != nil {
				return newSnapshot(nil), err
			}
			externalFacts = loaded
		} else {
			loader.mode = externalFactLoaderLibrary
			loader.includeEnv = e.cfg.SystemDefaults
			loaded, err := loader.load()
			if err != nil {
				failures = append(failures, err)
			}
			externalFacts = loaded
		}
	}
	if ctx.Err() != nil {
		facts = externalFacts
		return finish()
	}

	var registeredFacts []ResolvedFact
	for _, fact := range e.cfg.Facts {
		if ctx.Err() != nil {
			break
		}
		var value any
		if fact.Resolve != nil {
			resolved, err := fact.Resolve(ctx)
			if err != nil {
				failures = append(failures, fmt.Errorf("fact %s: %w", fact.Name, err))
				continue
			}
			value = NormalizeCustomValue(resolved)
			if CustomValueContainsNullByte(value) {
				s.warn("custom fact value contains a null byte reference")
				value = nil
			}
		}
		registeredFacts = append(registeredFacts, ResolvedFact{Name: fact.Name, Value: value, Type: "custom"})
	}
	if ctx.Err() != nil {
		facts = append(registeredFacts, externalFacts...)
		return finish()
	}

	facts = CoreFacts(s)
	facts = append(facts, registeredFacts...)
	facts = append(facts, externalFacts...)
	facts = FilterBlockedFacts(facts, blocked)

	if e.cfg.UseCache && ctx.Err() == nil {
		cache := NewFactCache(DefaultCachePath(), ttls, cacheGroups)
		remaining, cached := cache.ResolveFacts(facts)
		if err := cache.CacheFacts(remaining); err != nil {
			failures = append(failures, err)
		}
		facts = append(remaining, cached...)
	}

	return finish()
}
