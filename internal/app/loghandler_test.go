package app

import (
	"bytes"
	"log/slog"
	"testing"
)

// stderrLogHandler must keep error-class engine diagnostics off stderr: the
// output contract has never rendered them (no error handler was ever
// installed), and the single-diagnostics-seam change routes canonical-tree
// collision diagnostics through this handler at error severity — so the drop
// is load-bearing. Warn and debug lines stay byte-identical.
func TestStderrLogHandlerDropsErrorClassKeepsWarnDebug(t *testing.T) {
	t.Run("error-class produces no stderr line", func(t *testing.T) {
		var stderr bytes.Buffer
		logger := slog.New(&stderrLogHandler{stderr: &stderr, debug: true, verbose: true})
		logger.Error("Custom fact `mygroup.fact1` cannot be added to collection.")
		if stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty (error-class dropped)", stderr.String())
		}
	})
	t.Run("warn-class renders WARN line", func(t *testing.T) {
		var stderr bytes.Buffer
		logger := slog.New(&stderrLogHandler{stderr: &stderr})
		logger.Warn("heads up")
		if got, want := stderr.String(), "WARN Facts - heads up\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}
	})
	t.Run("debug-class renders DEBUG line when enabled", func(t *testing.T) {
		var stderr bytes.Buffer
		logger := slog.New(&stderrLogHandler{stderr: &stderr, debug: true})
		logger.Debug("trace it")
		if got, want := stderr.String(), "DEBUG Facts - trace it\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}
	})
}
