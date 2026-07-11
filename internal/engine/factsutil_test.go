package engine

import (
	"reflect"
	"testing"
)

func TestDiscoverFamily_matchesRubyFactsUtils(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "fedora", id: "rhel fedora", want: "RedHat"},
		{name: "centos", id: "rhel fedora centos", want: "RedHat"},
		{name: "amazon", id: "amazon", want: "RedHat"},
		{name: "amazon linux id", id: "amzn", want: "RedHat"},
		{name: "oracle linux", id: "oraclelinux", want: "RedHat"},
		{name: "oracle linux id", id: "ol", want: "RedHat"},
		{name: "meego", id: "meego", want: "RedHat"},
		{name: "rocky", id: "rocky", want: "RedHat"},
		{name: "almalinux", id: "almalinux", want: "RedHat"},
		{name: "short id requires exact match", id: "solus", want: "solus"},
		{name: "azure linux", id: "azurelinux", want: "RedHat"},
		{name: "psbm", id: "PSBM", want: "RedHat"},
		{name: "virtuozzo", id: "VirtuozzoLinux", want: "RedHat"},
		{name: "sled", id: "SLED", want: "Suse"},
		{name: "kde", id: "KDE", want: "Debian"},
		{name: "huawei", id: "HuaweiOS", want: "Debian"},
		{name: "linux mint", id: "linuxmint", want: "Debian"},
		{name: "devuan", id: "devuan", want: "Debian"},
		{name: "gentoo", id: "gentoo", want: "Gentoo"},
		{name: "manjaro", id: "Manjaro", want: "Archlinux"},
		{name: "mandriva", id: "Mandriva", want: "Mandrake"},
		{name: "mageia", id: "Mageia", want: "Mandrake"},
		{name: "unknown falls back", id: "OpenWrt", want: "OpenWrt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := discoverFamily(tt.id); got != tt.want {
				t.Fatalf("discoverFamily(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestUsesRedHatReleaseDistroExcludesOracleAndAmazonAliases(t *testing.T) {
	for _, id := range []string{"ol", "oel", "oraclelinux", "amzn", "amazon"} {
		t.Run(id, func(t *testing.T) {
			if usesRedHatReleaseDistro(id) {
				t.Fatalf("usesRedHatReleaseDistro(%q) = true, want false", id)
			}
		})
	}
	if !usesRedHatReleaseDistro("rhel") {
		t.Fatal("usesRedHatReleaseDistro(rhel) = false, want true")
	}
}

func TestReleaseHashFromString_matchesRubyFactsUtils(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		includePatch bool
		want         map[string]any
	}{
		{name: "major minor", version: "6.2", want: map[string]any{"full": "6.2", "major": "6", "minor": "2"}},
		{name: "patch omitted", version: "6.2.1", want: map[string]any{"full": "6.2.1", "major": "6", "minor": "2"}},
		{name: "patch included", version: "6.2.1", includePatch: true, want: map[string]any{"full": "6.2.1", "major": "6", "minor": "2", "patch": "1"}},
		{name: "major only", version: "6", want: map[string]any{"full": "6", "major": "6"}},
		{name: "no patch", version: "6.2", includePatch: true, want: map[string]any{"full": "6.2", "major": "6", "minor": "2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := releaseHashFromString(tt.version, tt.includePatch)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("releaseHashFromString() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestReleaseHashFromString_emptyVersionReturnsNil(t *testing.T) {
	if got := releaseHashFromString("", false); got != nil {
		t.Fatalf("releaseHashFromString() = %#v, want nil", got)
	}
}

func TestFirstNonEmptyReturnsFirstValueInOrder(t *testing.T) {
	if got := firstNonEmpty("first", "second"); got != "first" {
		t.Fatalf("firstNonEmpty() = %q, want first", got)
	}
	if got := firstNonEmpty("", "first", "second"); got != "first" {
		t.Fatalf("firstNonEmpty() = %q, want first", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("firstNonEmpty(empty) = %q, want empty", got)
	}
}

func TestDeepStringifyKeys_matchesRubyUtils(t *testing.T) {
	input := map[any]any{
		"this": map[any]any{
			"is": []any{
				map[any]any{1: "test"},
			},
		},
	}

	want := map[string]any{
		"this": map[string]any{
			"is": []any{
				map[string]any{"1": "test"},
			},
		},
	}

	if got := deepStringifyKeys(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("deepStringifyKeys() = %#v, want %#v", got, want)
	}
}

func TestDeepStringifyKeysHandlesStringMapsSlicesAndScalars(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"nested": map[any]any{
			1: []any{
				map[string]any{"leaf": "value"},
				"scalar",
			},
		},
	}
	want := map[string]any{
		"nested": map[string]any{
			"1": []any{
				map[string]any{"leaf": "value"},
				"scalar",
			},
		},
	}

	if got := deepStringifyKeys(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("deepStringifyKeys() = %#v, want %#v", got, want)
	}
	if got := deepStringifyKeys("scalar"); got != "scalar" {
		t.Fatalf("deepStringifyKeys(scalar) = %#v, want scalar", got)
	}
}
