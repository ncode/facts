//go:build linux

package engine

import "syscall"

// statMountpoint resolves filesystem size, available, and used bytes for a
// mount on Linux. Linux exposes both the I/O block size (f_bsize) and the
// fundamental fragment size (f_frsize); the statfs block counts (f_blocks,
// f_bavail, f_bfree) are expressed in f_frsize units, so block totals MUST be
// multiplied by f_frsize to match df/Facter. On filesystems where the two
// differ (for example virtiofs, which reports f_bsize 256x larger than
// f_frsize) using f_bsize inflates sizes massively. When f_frsize is zero we
// fall back to f_bsize, matching coreutils df.
func statMountpoint(path string) (mountStat, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return mountStat{}, false
	}
	return linuxMountStat(int64(stat.Blocks), int64(stat.Bavail), int64(stat.Bfree), int64(stat.Bsize), int64(stat.Frsize)), true
}

// linuxMountStat converts statfs block counts to byte totals using the
// fundamental fragment size (frsize), falling back to the I/O block size (bsize)
// only when frsize is zero, matching coreutils df. The block counts (blocks,
// bavail, bfree) are expressed in frsize units, so using bsize on filesystems
// where bsize != frsize (for example virtiofs, bsize 256x frsize) inflates
// every byte total. Kept pure (no syscall) so the block math is unit-testable.
func linuxMountStat(blocks, bavail, bfree, bsize, frsize int64) mountStat {
	blockSize := frsize
	if blockSize == 0 {
		blockSize = bsize
	}
	sizeBytes := blocks * blockSize
	availableBytes := bavail * blockSize
	usedBytes := sizeBytes - bfree*blockSize
	return mountStat{SizeBytes: int(sizeBytes), AvailableBytes: int(availableBytes), UsedBytes: int(usedBytes)}
}
