package engine

import (
	"errors"
	"reflect"
	"testing"
)

func TestConstructOSHierarchy_matchesSupportedFixture(t *testing.T) {
	hierarchy := []any{
		map[string]any{"Linux": []any{
			map[string]any{"Debian": []any{"Elementary", "Ubuntu", "Raspbian"}},
			map[string]any{"El": []any{"Fedora", "Amzn", "Centos"}},
			map[string]any{"Sles": []any{"Opensuse"}},
		}},
		"Macosx",
		"Windows",
	}

	tests := []struct {
		name     string
		searched string
		want     []string
	}{
		{name: "ubuntu", searched: "ubuntu", want: []string{"Linux", "Debian", "Ubuntu"}},
		{name: "debian", searched: "debian", want: []string{"Linux", "Debian"}},
		{name: "linux", searched: "linux", want: []string{"Linux"}},
		{name: "unknown", searched: "my_os", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConstructOSHierarchy(hierarchy, tt.searched)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ConstructOSHierarchy(%q) = %#v, want %#v", tt.searched, got, tt.want)
			}
		})
	}
}

func TestConstructOSHierarchy_returnsEmptyForNilSearchedOS(t *testing.T) {
	if got := ConstructOSHierarchy([]any{"Linux"}, ""); len(got) != 0 {
		t.Fatalf("ConstructOSHierarchy(empty) = %#v, want empty", got)
	}
}

func TestConstructOSHierarchy_fallsBackWhenHierarchyMissing(t *testing.T) {
	got := ConstructOSHierarchy(nil, "myos")
	want := []string{"Myos"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConstructOSHierarchy(nil, myos) = %#v, want %#v", got, want)
	}
}

func TestDetectOSHierarchyFallsBackToLinuxWhenDistroAndFamilyAreUnknownLikeRubyDetector(t *testing.T) {
	hierarchy := []any{
		map[string]any{"Linux": []any{
			map[string]any{"Debian": []any{"Ubuntu"}},
		}},
		"Windows",
	}
	debugMessages := []string{}
	SetDebugHandler(func(message string) {
		debugMessages = append(debugMessages, message)
	})
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := DetectOSHierarchy(hierarchy, "my_linux_distro", "")
	want := []string{"Linux"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DetectOSHierarchy() = %#v, want %#v", got, want)
	}

	wantMessages := []string{
		"Could not detect hierarchy using os identifier: my_linux_distro , trying with family",
		"Could not detect hierarchy using family , falling back to Linux",
	}
	if !reflect.DeepEqual(debugMessages, wantMessages) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, wantMessages)
	}
}

func TestDetectOSHierarchyUsesFirstKnownFamilyLikeRubyDetector(t *testing.T) {
	hierarchy := []any{
		map[string]any{"Linux": []any{
			map[string]any{"El": []any{"Centos", "Fedora"}},
		}},
	}
	debugMessages := []string{}
	SetDebugHandler(func(message string) {
		debugMessages = append(debugMessages, message)
	})
	t.Cleanup(func() { SetDebugHandler(nil) })

	got := DetectOSHierarchy(hierarchy, "my_linux_distro", "Rhel centos fedora")
	want := []string{"Linux", "El", "Centos"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DetectOSHierarchy() = %#v, want %#v", got, want)
	}

	wantMessages := []string{
		"Could not detect hierarchy using os identifier: my_linux_distro , trying with family",
	}
	if !reflect.DeepEqual(debugMessages, wantMessages) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, wantMessages)
	}
}

func TestDetectOSIdentifier_matchesRubyHostOSMapping(t *testing.T) {
	tests := []struct {
		name    string
		hostOS  string
		distro  string
		want    string
		wantErr error
	}{
		{name: "macos", hostOS: "darwin", want: "macosx"},
		{name: "windows mingw", hostOS: "mingw", want: "windows"},
		{name: "windows mswin", hostOS: "mswin", want: "windows"},
		{name: "linux distro", hostOS: "linux", distro: "redhat", want: "redhat"},
		{name: "linux fallback", hostOS: "linux", want: "linux"},
		{name: "unknown", hostOS: "my_custom_os", wantErr: ErrUnknownOS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectOSIdentifier(tt.hostOS, tt.distro)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("DetectOSIdentifier() err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil && err.Error() != `unknown os: "my_custom_os"` {
				t.Fatalf("DetectOSIdentifier() err = %q, want Ruby unknown OS message", err)
			}
			if got != tt.want {
				t.Fatalf("DetectOSIdentifier(%q, %q) = %q, want %q", tt.hostOS, tt.distro, got, tt.want)
			}
		})
	}
}
