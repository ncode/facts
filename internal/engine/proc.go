package engine

import (
	"fmt"
	"strings"
)

type procEnvironReader func(path string, defaultValue []string) ([]string, bool)

func linuxProcGetenvForPID(pid int, field string, readLines procEnvironReader) (string, bool) {
	prefix := field + "="
	lines, _ := readLines(fmt.Sprintf("/proc/%d/environ", pid), nil)
	for _, line := range lines {
		for part := range strings.SplitSeq(line, "\x00") {
			value, ok := strings.CutPrefix(part, prefix)
			if ok {
				return value, true
			}
		}
	}
	return "", false
}
