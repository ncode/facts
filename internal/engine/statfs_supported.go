//go:build linux || darwin || freebsd

package engine

import "syscall"

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
