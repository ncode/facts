//go:build linux

package engine

import "testing"

// TestLinuxMountStatUsesFrsize asserts the Linux statfs block math multiplies
// block counts by f_frsize (not f_bsize), and falls back to f_bsize only when
// f_frsize is zero. (task 1.1)
func TestLinuxMountStatUsesFrsize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                          string
		blocks, bavail, bfree         uint64
		bsize, frsize                 int64
		wantSize, wantAvail, wantUsed int
	}{
		{
			// virtiofs-style: f_bsize is 256x f_frsize. Using f_bsize would
			// inflate every total 256x. df reports the f_frsize totals.
			name:   "frsize differs from bsize",
			blocks: 60_000_000, bavail: 45_000_000, bfree: 45_000_000,
			bsize: 1_048_576, frsize: 4_096,
			wantSize:  60_000_000 * 4_096,
			wantAvail: 45_000_000 * 4_096,
			wantUsed:  (60_000_000 - 45_000_000) * 4_096,
		},
		{
			// Normal filesystem where frsize == bsize.
			name:   "frsize equals bsize",
			blocks: 1_000, bavail: 250, bfree: 300,
			bsize: 4_096, frsize: 4_096,
			wantSize:  1_000 * 4_096,
			wantAvail: 250 * 4_096,
			wantUsed:  (1_000 - 300) * 4_096,
		},
		{
			// Exotic filesystem reporting f_frsize == 0 must fall back to
			// f_bsize, matching coreutils df.
			name:   "frsize zero falls back to bsize",
			blocks: 2_000, bavail: 500, bfree: 600,
			bsize: 512, frsize: 0,
			wantSize:  2_000 * 512,
			wantAvail: 500 * 512,
			wantUsed:  (2_000 - 600) * 512,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := linuxMountStat(tt.blocks, tt.bavail, tt.bfree, tt.bsize, tt.frsize)
			if got.SizeBytes != tt.wantSize {
				t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, tt.wantSize)
			}
			if got.AvailableBytes != tt.wantAvail {
				t.Errorf("AvailableBytes = %d, want %d", got.AvailableBytes, tt.wantAvail)
			}
			if got.UsedBytes != tt.wantUsed {
				t.Errorf("UsedBytes = %d, want %d", got.UsedBytes, tt.wantUsed)
			}
		})
	}
}

// TestLinuxMountStatCapacityForFullReadOnlyMount asserts a full read-only mount
// (f_bavail == 0) produces available_bytes 0, so filesystemCapacity reports
// 100%. (task 1.2)
func TestLinuxMountStatCapacityForFullReadOnlyMount(t *testing.T) {
	t.Parallel()

	// Squashfs-style read-only image: every block used, nothing available.
	got := linuxMountStat(10_000, 0, 0, 131_072, 131_072)
	if got.AvailableBytes != 0 {
		t.Fatalf("AvailableBytes = %d, want 0", got.AvailableBytes)
	}
	if cap := filesystemCapacity(got.UsedBytes, got.AvailableBytes); cap != "100%" {
		t.Fatalf("capacity = %q, want 100%%", cap)
	}
}
