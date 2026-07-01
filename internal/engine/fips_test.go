package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestCoreFacts_fipsEnabledOnlyOnLinuxAndWindows(t *testing.T) {
	collection := Collection(CoreFacts(testSession, nil))

	value, ok := collection["fips_enabled"]
	switch runtime.GOOS {
	case "linux", "windows":
		if _, isBool := value.(bool); !ok || !isBool {
			t.Fatalf("fips_enabled = %#v, want bool", value)
		}
	default:
		if ok {
			t.Fatalf("fips_enabled = %#v, want fact omitted on %s", value, runtime.GOOS)
		}
	}
}

func TestFIPSEnabledFacts_omittedOutsideLinuxAndWindows(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		t.Fatalf("command = %s %#v, want no probe on a platform without the fact", name, args)
		return ""
	}
	for _, goos := range []string{"darwin", "freebsd", "openbsd", "netbsd", "solaris", "aix"} {
		if got := fipsEnabledFacts(goos, "/proc/sys/crypto/fips_enabled", run, os.ReadFile); got != nil {
			t.Fatalf("fipsEnabledFacts(%s) = %#v, want nil", goos, got)
		}
	}
}

func TestFIPSEnabledFacts_resolveOnLinuxAndWindows(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "fips_enabled")
	if err := os.WriteFile(path, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "fips_enabled", Value: true}}
	if got := fipsEnabledFacts("linux", path, nil, os.ReadFile); !reflect.DeepEqual(got, want) {
		t.Fatalf("fipsEnabledFacts(linux) = %#v, want %#v", got, want)
	}

	run := func(name string, args ...string) string {
		return strings.Join([]string{
			`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
			"    Enabled    REG_DWORD    0x0",
		}, "\n")
	}
	want = []ResolvedFact{{Name: "fips_enabled", Value: false}}
	if got := fipsEnabledFacts("windows", "", run, os.ReadFile); !reflect.DeepEqual(got, want) {
		t.Fatalf("fipsEnabledFacts(windows) = %#v, want %#v", got, want)
	}
}

func TestCurrentFIPSEnabledReadsWindowsRegistry(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		if name != "reg" || !reflect.DeepEqual(args, []string{"query", `HKLM\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`, "/v", "Enabled"}) {
			t.Fatalf("command = %s %#v", name, args)
		}
		return strings.Join([]string{
			`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
			"    Enabled    REG_DWORD    0xff",
		}, "\n")
	}

	if !currentFIPSEnabled("windows", "", run, os.ReadFile) {
		t.Fatal("currentFIPSEnabled(windows) = false, want true")
	}
}

func TestParseWindowsFIPSEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name: "enabled decimal",
			input: strings.Join([]string{
				`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
				"    Enabled    REG_DWORD    0x1",
			}, "\n"),
			want: true,
		},
		{
			name: "enabled non-one",
			input: strings.Join([]string{
				`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
				"    Enabled    REG_DWORD    0xff",
			}, "\n"),
			want: true,
		},
		{
			name: "disabled zero",
			input: strings.Join([]string{
				`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
				"    Enabled    REG_DWORD    0x0",
			}, "\n"),
			want: false,
		},
		{
			name:  "missing",
			input: `HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseWindowsFIPSEnabled(tt.input); got != tt.want {
				t.Fatalf("parseWindowsFIPSEnabled() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestFIPSEnabledReadsProcFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "enabled", content: "1\n", want: true},
		{name: "disabled", content: "0\n", want: false},
		{name: "unexpected", content: "enabled\n", want: false},
		{name: "whitespace", content: " 1 \n", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "fips_enabled")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}

			if got := fipsEnabled(path, os.ReadFile); got != tt.want {
				t.Fatalf("fipsEnabled() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestFIPSEnabledMissingFileIsFalse(t *testing.T) {
	t.Parallel()

	if got := fipsEnabled(filepath.Join(t.TempDir(), "missing"), os.ReadFile); got {
		t.Fatalf("fipsEnabled() = %t, want false", got)
	}
}
