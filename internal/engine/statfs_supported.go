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

	blockSize := int64(stat.Bsize)
	sizeBytes := int64(stat.Blocks) * blockSize
	availableBytes := int64(stat.Bavail) * blockSize
	usedBytes := sizeBytes - int64(stat.Bfree)*blockSize
	return mountStat{SizeBytes: int(sizeBytes), AvailableBytes: int(availableBytes), UsedBytes: int(usedBytes)}, true
}
