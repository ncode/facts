//go:build darwin || freebsd

package engine

import "syscall"

// statMountpoint resolves filesystem size, available, and used bytes for a
// mount on darwin/freebsd. Their syscall.Statfs_t has no f_frsize field; f_bsize
// is already the fundamental block size there, so block counts are multiplied by
// f_bsize. The Linux implementation lives in statfs_linux.go and uses f_frsize.
func statMountpoint(path string) (mountStat, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return mountStat{}, false
	}

	blockSize := uint64(stat.Bsize)
	sizeBytes := statfsNativeBlockBytes(stat.Blocks, blockSize)
	availableBytes := statfsNativeBlockBytes(stat.Bavail, blockSize)
	freeBytes := statfsNativeBlockBytes(stat.Bfree, blockSize)
	return mountStat{SizeBytes: sizeBytes, AvailableBytes: availableBytes, UsedBytes: statfsUsedBytes(sizeBytes, freeBytes)}, true
}
