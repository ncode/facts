package facts

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ncode/facts/internal/engine"
)

// ErrFactNotFound reports a query that no fact resolved. A fact that
// legitimately resolved to nil is found, not missing: Snapshot.Value returns
// (nil, nil) for it.
var ErrFactNotFound = engine.ErrFactNotFound

// Engine is an isolated, immutable unit of fact-discovery configuration.
// Construct one with New; nothing mutates after construction, and Engines are
// safe for concurrent use. A new Engine is hermetic: it discovers core facts
// only, until options opt it into registered facts, external facts, a config
// file, a cache, or full system-following behavior.
type Engine struct {
	inner *engine.Engine
}

// Option configures an Engine under construction.
type Option func(*engine.EngineConfig) error

// New returns an immutable Engine built from opts. With no options the
// Engine is hermetic: it resolves core facts only — it does not read config
// files, scan fact directories, execute external-fact scripts, read
// FACTS_*/FACTER_* environment variables, or touch the persistent cache.
func New(opts ...Option) (*Engine, error) {
	var cfg engine.EngineConfig
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	inner, err := engine.NewEngine(cfg)
	if err != nil {
		return nil, err
	}
	return &Engine{inner: inner}, nil
}

// WithConfigFile opts into reading the given config file (facter.conf
// semantics), honoring its fact directories, toggles, blocklists, and cache
// TTLs with the same semantics as the facts CLI.
func WithConfigFile(path string) Option {
	return func(cfg *engine.EngineConfig) error {
		if path == "" {
			return errors.New("config file path is empty")
		}
		cfg.ConfigFile = path
		return nil
	}
}

// WithExternalDirs opts into loading external facts from exactly the given
// directories, with input-contract semantics identical to the CLI's.
func WithExternalDirs(dirs ...string) Option {
	return func(cfg *engine.EngineConfig) error {
		cfg.ExternalDirs = append(cfg.ExternalDirs, dirs...)
		return nil
	}
}

// WithCache opts into the persistent fact cache with facter.conf TTL
// semantics.
func WithCache() Option {
	return func(cfg *engine.EngineConfig) error {
		cfg.UseCache = true
		return nil
	}
}

// WithFact registers a fact programmatically, fixed at construction. The
// resolver runs once per Discover; a nil value with a nil error registers the
// fact as resolved-to-nil, and an error counts as a partial discovery
// failure. A registered fact overrides a core fact of the same name and is
// overridden by external facts.
func WithFact(name string, resolve func(ctx context.Context) (any, error)) Option {
	return func(cfg *engine.EngineConfig) error {
		cfg.Facts = append(cfg.Facts, engine.ProgrammaticFact{Name: name, Resolve: resolve})
		return nil
	}
}

// WithLogger routes engine diagnostics to logger with contract-pinned message
// text, mapped severities, and once-only conditions deduplicated per Engine.
// Without it, diagnostics are discarded.
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *engine.EngineConfig) error {
		cfg.Logger = logger
		return nil
	}
}

// WithSystemDefaults configures full CLI-equivalent system-following
// behavior: the default config file (the facts-native facts.conf first, the
// facter-compatible facter.conf second), the default external fact
// directories (facts-native locations ahead of the facter-compatible ones),
// and FACTS_*/FACTER_* environment facts (FACTS_* wins name collisions).
func WithSystemDefaults() Option {
	return func(cfg *engine.EngineConfig) error {
		cfg.SystemDefaults = true
		return nil
	}
}

// Discover runs the configured resolvers and returns an immutable Snapshot of
// the canonical fact tree. Queries, when given, follow the CLI's dot-notation
// semantics.
//
// Discovery failures are partial: the returned Snapshot holds every fact that
// resolved, and err joins each per-source failure. When ctx ends discovery
// early, err satisfies errors.Is(err, ctx.Err()). Not-applicable facts are
// absent from the Snapshot and contribute nothing to err.
func (e *Engine) Discover(ctx context.Context, queries ...string) (*Snapshot, error) {
	if e == nil || e.inner == nil {
		return nil, errors.New("facts: uninitialized Engine")
	}
	inner, err := e.inner.Discover(ctx, queries...)
	return &Snapshot{inner: inner}, err
}
