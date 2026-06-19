package engine

import "testing"

func TestStatfsUsedBlockBytesClampsAfterSubtractingFreeBlocks(t *testing.T) {
	got := statfsUsedBlockBytes(^uint64(0), ^uint64(0)-1, 4096)
	if got != 4096 {
		t.Fatalf("statfsUsedBlockBytes() = %d, want 4096", got)
	}
}

func TestStatfsUsedBlockBytesClampsOverflow(t *testing.T) {
	got := statfsUsedBlockBytes(^uint64(0), 0, 4096)
	if got != maxInt {
		t.Fatalf("statfsUsedBlockBytes() = %d, want maxInt", got)
	}
}
