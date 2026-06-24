package app

import (
	"context"
	"io"
	"log/slog"
	"sync"
)

// stderrLogHandler renders engine diagnostics as Ruby-compatible stderr lines
// ("WARN Facts - ...", "DEBUG Facts - ..."). The stderr contract lives here,
// at the CLI boundary: warn-class diagnostics always print, info and debug
// print only under --verbose/--debug, and error-class engine diagnostics are
// dropped — the CLI has never rendered them (no error handler was ever
// installed) and the output contract pins that silence.
type stderrLogHandler struct {
	stderr  io.Writer
	color   bool
	debug   bool
	verbose bool
	mu      sync.Mutex
}

func (h *stderrLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	switch {
	case level >= slog.LevelError:
		return false
	case level >= slog.LevelWarn:
		return true
	case level >= slog.LevelInfo:
		return h.verbose
	default:
		return h.debug
	}
}

func (h *stderrLogHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch {
	case record.Level >= slog.LevelError:
	case record.Level >= slog.LevelWarn:
		writeWarn(h.stderr, record.Message, h.color)
	case record.Level >= slog.LevelInfo:
		writeInfo(h.stderr, record.Message, h.color)
	default:
		writeDebug(h.stderr, record.Message, h.color)
	}
	return nil
}

// Contract stderr lines carry only message text; attributes and groups have
// no rendering.
func (h *stderrLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *stderrLogHandler) WithGroup(string) slog.Handler      { return h }
