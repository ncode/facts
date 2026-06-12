//go:build !linux && !darwin && !freebsd

package engine

func statMountpoint(string) (mountStat, bool) {
	return mountStat{}, false
}
