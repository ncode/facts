package engine

import (
	"log/slog"
	"os"
	"sort"
	"strings"
)

func safeRead(path string, defaultValue string, log *slog.Logger) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		debugFileNotAccessible(path, log)
		return defaultValue, false
	}
	return string(data), true
}

func safeReadLines(path string, defaultValue []string, log *slog.Logger) ([]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		debugFileNotAccessible(path, log)
		return defaultValue, false
	}
	if len(data) == 0 {
		return []string{}, true
	}
	lines := strings.SplitAfter(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, true
}

func debugFileNotAccessible(path string, log *slog.Logger) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	log.Debug("File at: " + path + " is not accessible.")
}

func dirChildren(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}
