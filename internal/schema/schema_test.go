package schema

import (
	"reflect"
	"strings"
	"testing"

	targets "github.com/ncode/facts/internal/platform"
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

func TestFlattenTreeEscapesDottedKeys(t *testing.T) {
	tree := map[string]any{
		"networking": map[string]any{
			"interfaces": map[string]any{
				"eth0.100": map[string]any{
					"mtu": 1500,
				},
			},
		},
	}

	paths := FlattenTree(tree)
	want := []string{`networking.interfaces.eth0\.100.mtu`}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("FlattenTree() = %#v, want %#v", paths, want)
	}

	s := Schema{
		"networking.interfaces.*.mtu": {
			Type:        "integer",
			Description: "interface MTU",
			Platforms:   []string{"linux"},
		},
	}
	if got := s.UndocumentedPaths(paths, "linux"); len(got) != 0 {
		t.Fatalf("UndocumentedPaths() = %#v, want none", got)
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

func TestSchemaMissingEntriesSkipsAbsentWildcardCollection(t *testing.T) {
	s := Schema{
		"mountpoints.*.size": {
			Type:        "string",
			Description: "mount size",
			Platforms:   []string{"linux"},
		},
	}

	got := s.MissingEntries([]string{"kernel.name"}, "linux")
	if len(got) != 0 {
		t.Fatalf("MissingEntries() = %#v, want none", got)
	}
}

func TestSchemaMissingEntriesRequiresWildcardCollectionRoot(t *testing.T) {
	s := Schema{
		"mountpoints.*": {
			Type:        "map",
			Description: "mountpoint",
			Platforms:   []string{"linux"},
		},
	}

	got := s.MissingEntries([]string{"kernel.name"}, "linux")
	want := []string{"mountpoints.*"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingEntries() = %#v, want %#v", got, want)
	}
}

func TestSchemaMissingEntriesRequiresWildcardChildWhenCollectionExists(t *testing.T) {
	s := Schema{
		"mountpoints.*.size": {
			Type:        "string",
			Description: "mount size",
			Platforms:   []string{"linux"},
		},
	}

	got := s.MissingEntries([]string{"mountpoints.root.device"}, "linux")
	want := []string{"mountpoints.*.size"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingEntries() = %#v, want %#v", got, want)
	}
}

func TestSchemaMissingEntriesRequiresWildcardChildForEachCollectionMember(t *testing.T) {
	s := Schema{
		"mountpoints.*.size": {
			Type:        "string",
			Description: "mount size",
			Platforms:   []string{"linux"},
		},
	}

	got := s.MissingEntries([]string{"mountpoints.root.size", "mountpoints.data.device"}, "linux")
	want := []string{"mountpoints.*.size"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingEntries() = %#v, want %#v", got, want)
	}
}

func TestSchemaMissingEntriesSkipsAbsentNestedWildcardCollection(t *testing.T) {
	s := Schema{
		"a.*.b.*.c": {
			Type:        "string",
			Description: "nested child",
			Platforms:   []string{"linux"},
		},
	}

	got := s.MissingEntries([]string{"a.one.name"}, "linux")
	if len(got) != 0 {
		t.Fatalf("MissingEntries() = %#v, want none", got)
	}
}

func TestSchemaMissingEntriesRequiresNestedWildcardChildWhenCollectionExists(t *testing.T) {
	s := Schema{
		"a.*.b.*.c": {
			Type:        "string",
			Description: "nested child",
			Platforms:   []string{"linux"},
		},
	}

	got := s.MissingEntries([]string{"a.one.b.two.name"}, "linux")
	want := []string{"a.*.b.*.c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingEntries() = %#v, want %#v", got, want)
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

func TestParseRejectsMultipleYAMLDocuments(t *testing.T) {
	data := []byte(`
kernel.name:
  type: string
  description: Kernel name.
  platforms: [linux]
---
kernel.release:
  type: string
  description: Kernel release.
  platforms: [linux]
`)

	_, err := Parse(data)
	if err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("Parse() error = %v, want multiple YAML documents", err)
	}
}

func TestPlatformsUseTargetProfileVocabulary(t *testing.T) {
	got := make([]string, 0, len(Platforms()))
	for _, platform := range Platforms() {
		got = append(got, platform.ID)
	}

	want := make([]string, 0, len(targets.SchemaVisibleProfiles()))
	for _, profile := range targets.SchemaVisibleProfiles() {
		want = append(want, profile.ID)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Platforms() IDs = %#v, want target profile schema IDs %#v", got, want)
	}
}
