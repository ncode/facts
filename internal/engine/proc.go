package engine

import (
	"fmt"
	"os"
	"strings"
)

type procEnvironReader func(path string, defaultValue []string) ([]string, bool)

func linuxProcGetenvForPID(pid int, field string, readLines procEnvironReader) (string, bool) {
	prefix := field + "="
	lines, _ := readLines(fmt.Sprintf("/proc/%d/environ", pid), nil)
	for _, line := range lines {
		value, ok := strings.CutPrefix(line, prefix)
		if ok {
			return value, true
		}
	}
	return "", false
}

func linuxProcEnvironLines(path string, defaultValue []string) ([]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultValue, false
	}
	if len(data) == 0 {
		return []string{}, true
	}
	lines := strings.Split(string(data), "\x00")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, true
}
