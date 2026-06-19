//go:build windows

package engine

import "golang.org/x/sys/windows"

func coreWindowsRoot() string {
	root, err := windows.GetSystemWindowsDirectory()
	if err != nil || root == "" {
		return `C:\Windows`
	}
	return root
}
