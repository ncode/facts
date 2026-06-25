package engine

import "testing"

func TestStatfsBlockBytesRejectsZeroInputs(t *testing.T) {
	t.Parallel()

	if got := statfsBlockBytes(0, 4096); got != 0 {
		t.Fatalf("statfsBlockBytes(zero blocks) = %d, want 0", got)
	}
	if got := statfsBlockBytes(10, 0); got != 0 {
		t.Fatalf("statfsBlockBytes(zero block size) = %d, want 0", got)
	}
}

func TestStatfsBlockBytesClampsOverflowOnAllPlatforms(t *testing.T) {
	t.Parallel()

	if got := statfsBlockBytes(^uint64(0), 4096); got != maxInt {
		t.Fatalf("statfsBlockBytes(overflow) = %d, want maxInt", got)
	}
}

func TestStatfsNativeBlockBytesRejectsNegativeBlocks(t *testing.T) {
	t.Parallel()

	if got := statfsNativeBlockBytes(int64(-1), 4096); got != 0 {
		t.Fatalf("statfsNativeBlockBytes(negative blocks) = %d, want 0", got)
	}
	if got := statfsNativeBlockBytes(int64(7), 512); got != 3584 {
		t.Fatalf("statfsNativeBlockBytes() = %d, want 3584", got)
	}
}

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

func TestStatfsNativeUsedBytesRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		blocks    int64
		free      int64
		blockSize uint64
	}{
		{name: "zero blocks", blocks: 0, free: 0, blockSize: 4096},
		{name: "negative blocks", blocks: -1, free: 0, blockSize: 4096},
		{name: "negative free blocks", blocks: 10, free: -1, blockSize: 4096},
		{name: "free blocks exceed total", blocks: 10, free: 12, blockSize: 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := statfsNativeUsedBytes(tt.blocks, tt.free, tt.blockSize); got != 0 {
				t.Fatalf("statfsNativeUsedBytes(%d, %d, %d) = %d, want 0", tt.blocks, tt.free, tt.blockSize, got)
			}
		})
	}
}

func TestStatfsNativeUsedBytesSubtractsFreeBlocks(t *testing.T) {
	if got := statfsNativeUsedBytes(int64(10), int64(3), uint64(4096)); got != 28_672 {
		t.Fatalf("statfsNativeUsedBytes() = %d, want 28672", got)
	}
}
