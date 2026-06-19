//go:build darwin || freebsd

package engine

import "testing"

func TestStatfsBlockBytesClampsOverflow(t *testing.T) {
	got := statfsBlockBytes(^uint64(0), 4096)
	if got != maxInt {
		t.Fatalf("statfsBlockBytes() = %d, want maxInt", got)
	}
}
