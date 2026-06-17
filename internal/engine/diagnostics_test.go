package engine

import (
	"context"
	"log/slog"
	"sync"
)

// discardLog returns a logger that drops every record, for test call sites that
// do not assert on diagnostics.
func discardLog() *slog.Logger { return slog.New(slog.DiscardHandler) }

// captureLogger returns a *slog.Logger that appends emitted messages to the
// given per-level slices (pass nil to ignore a level). It replaces the deleted
// Set{Debug,Warning,Error}Handler capture pattern in tests: route diagnostics
// to a logger and assert on the captured slices.
func captureLogger(debugMsgs, warnMsgs, errorMsgs *[]string) *slog.Logger {
	return slog.New(&captureHandler{debug: debugMsgs, warn: warnMsgs, err: errorMsgs})
}

type captureHandler struct {
	mu               sync.Mutex
	debug, warn, err *[]string
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case r.Level >= slog.LevelError:
		if h.err != nil {
			*h.err = append(*h.err, r.Message)
		}
	case r.Level >= slog.LevelWarn:
		if h.warn != nil {
			*h.warn = append(*h.warn, r.Message)
		}
	case r.Level >= slog.LevelInfo:
		// info is unused by engine diagnostics; ignore.
	default:
		if h.debug != nil {
			*h.debug = append(*h.debug, r.Message)
		}
	}
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }
