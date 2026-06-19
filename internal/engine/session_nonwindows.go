//go:build !windows

package engine

func coreWindowsRoot() string {
	return `C:\Windows`
}
