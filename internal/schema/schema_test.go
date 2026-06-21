package schema

import (
	"reflect"
	"strings"
	"testing"
)

func TestMatchesPath(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		entry   Entry
		path    string
		want    bool
	}{
		{
			name:    "exact path",
			pattern: "kernel.name",
			entry:   Entry{Type: "string"},
			path:    "kernel.name",
			want:    true,
		},
		{
			name:    "wildcard matches one segment",
			pattern: "networking.interfaces.*.mtu",
			entry:   Entry{Type: "integer"},
			path:    "networking.interfaces.en0.mtu",
			want:    true,
		},
		{
			name:    "wildcard does not match multiple segments",
			pattern: "networking.interfaces.*.mtu",
			entry:   Entry{Type: "integer"},
			path:    "networking.interfaces.en0.alias.mtu",
			want:    false,
		},
		{
			name:    "documented dynamic child",
			pattern: "disks.*.serial_number",
			entry:   Entry{Type: "string"},
			path:    "disks.sda.serial_number",
			want:    true,
		},
		{
			name:    "dynamic map is not open",
			pattern: "disks.*",
			entry:   Entry{Type: "map"},
			path:    "disks.sda.serial",
			want:    false,
		},
		{
			name:    "array entry covers flattened array items",
			pattern: "path",
			entry:   Entry{Type: "array"},
			path:    "path.*",
			want:    true,
		},
		{
			name:    "explicit open subtree",
			pattern: "system_profiler",
			entry:   Entry{Type: "map", OpenSubtree: true},
			path:    "system_profiler.hardware.model_name",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesPath(tt.pattern, tt.entry, tt.path); got != tt.want {
				t.Fatalf("MatchesPath(%q, %#v, %q) = %v, want %v", tt.pattern, tt.entry, tt.path, got, tt.want)
			}
		})
	}
}

func TestSchemaUndocumentedPathsRejectsUnknownDynamicChildren(t *testing.T) {
	s := Schema{
		"disks.*": {
			Type:        "map",
			Description: "disk",
			Platforms:   []string{"linux"},
		},
		"disks.*.serial_number": {
			Type:        "string",
			Description: "serial number",
			Platforms:   []string{"linux"},
		},
	}

	got := s.UndocumentedPaths([]string{
		"disks.sda.serial",
		"disks.sda.serial_number",
	}, "linux")
	want := []string{"disks.sda.serial"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UndocumentedPaths() = %#v, want %#v", got, want)
	}
}

func TestFlattenTree(t *testing.T) {
	tree := map[string]any{
		"filesystems": []string{"apfs", "devfs"},
		"kernel": map[string]any{
			"name":    "Darwin",
			"release": map[string]any{},
		},
	}

	got := FlattenTree(tree)
	want := []string{"filesystems.*", "kernel.name", "kernel.release"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FlattenTree() = %#v, want %#v", got, want)
	}
}

func TestSchemaUndocumentedPathsAcceptsOpenSubtree(t *testing.T) {
	s := Schema{
		"system_profiler": {
			Type:        "map",
			Description: "system profiler",
			Platforms:   []string{"darwin"},
			OpenSubtree: true,
		},
	}

	got := s.UndocumentedPaths([]string{"system_profiler.hardware.model_name"}, "darwin")
	if len(got) != 0 {
		t.Fatalf("UndocumentedPaths() = %#v, want none", got)
	}
}

func TestValidateRejectsUnknownPlatform(t *testing.T) {
	s := Schema{
		"kernel.name": {
			Type:        "string",
			Description: "kernel name",
			Platforms:   []string{"linux", "solaris"},
		},
	}

	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), `invalid platform "solaris"`) {
		t.Fatalf("Validate() error = %v, want invalid platform", err)
	}
}

func TestValidateRejectsScalarOpenSubtree(t *testing.T) {
	s := Schema{
		"kernel.name": {
			Type:        "string",
			Description: "kernel name",
			Platforms:   []string{"linux"},
			OpenSubtree: true,
		},
	}

	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "open_subtree requires type map or array") {
		t.Fatalf("Validate() error = %v, want open_subtree type error", err)
	}
}
