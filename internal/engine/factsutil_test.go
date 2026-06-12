package engine

import (
	"reflect"
	"regexp"
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

func TestReleaseHashFromMatchData_matchesRubyFactsUtils(t *testing.T) {
	releasePattern := regexp.MustCompile(`^RELEASE=(\d+.\d+.*)`)
	majorPattern := regexp.MustCompile(`^RELEASE=(\d+)`)

	tests := []struct {
		name    string
		match   []string
		want    map[string]any
		wantNil bool
	}{
		{name: "major minor", match: releasePattern.FindStringSubmatch("RELEASE=4.3"), want: map[string]any{"full": "4.3", "major": "4", "minor": "3"}},
		{name: "major only", match: majorPattern.FindStringSubmatch("RELEASE=4"), want: map[string]any{"full": "4", "major": "4"}},
		{name: "nil", match: nil, wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := releaseHashFromMatchData(tt.match)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("releaseHashFromMatchData() = %#v, want nil", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("releaseHashFromMatchData() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTryToBool_matchesRubyUtils(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  any
	}{
		{name: "true", input: "true", want: true},
		{name: "false", input: "false", want: false},
		{name: "unchanged", input: "something else", want: "something else"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tryToBool(tt.input); got != tt.want {
				t.Fatalf("tryToBool(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTryToInt_matchesRubyUtils(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  any
	}{
		{name: "int", input: 7, want: 7},
		{name: "string int", input: "7", want: 7},
		{name: "positive string int", input: "+7", want: 7},
		{name: "negative string int", input: "-7", want: -7},
		{name: "float", input: 7.10, want: 7},
		{name: "non numeric string", input: "string", want: "string"},
		{name: "partial string int", input: "7string", want: "7string"},
		{name: "true", input: true, want: true},
		{name: "false", input: false, want: false},
		{name: "string float", input: "7.10", want: "7.10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tryToInt(tt.input); got != tt.want {
				t.Fatalf("tryToInt(%#v) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
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
