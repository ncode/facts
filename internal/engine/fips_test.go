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

	for _, goos := range []string{"darwin", "freebsd", "openbsd", "netbsd", "solaris", "aix"} {
		host := &fakeHostOS{platform: goos, emptyRunDefault: true}
		s := NewSession()
		s.host = host
		if got := fipsEnabledFacts(s, "/proc/sys/crypto/fips_enabled"); got != nil {
			t.Fatalf("fipsEnabledFacts(%s) = %#v, want nil", goos, got)
		}
		if len(host.runCalls) != 0 || len(host.readFileCalls) != 0 {
			t.Fatalf("fipsEnabledFacts(%s) probed the host (runs=%v reads=%v), want no probe", goos, host.runCalls, host.readFileCalls)
		}
	}
}

func TestFIPSEnabledFacts_resolveOnLinuxAndWindows(t *testing.T) {
	t.Parallel()

	const path = "/proc/sys/crypto/fips_enabled"
	linuxHost := &fakeHostOS{platform: "linux", files: map[string][]byte{path: []byte("1\n")}}
	linux := NewSession()
	linux.host = linuxHost
	want := []ResolvedFact{{Name: "fips_enabled", Value: true}}
	if got := fipsEnabledFacts(linux, path); !reflect.DeepEqual(got, want) {
		t.Fatalf("fipsEnabledFacts(linux) = %#v, want %#v", got, want)
	}

	windowsHost := &fakeHostOS{
		platform: "windows",
		runOutputs: map[string]string{
			fakeRunKey("reg", "query", `HKLM\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`, "/v", "Enabled"): strings.Join([]string{
				`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
				"    Enabled    REG_DWORD    0x0",
			}, "\n"),
		},
	}
	windows := NewSession()
	windows.host = windowsHost
	want = []ResolvedFact{{Name: "fips_enabled", Value: false}}
	if got := fipsEnabledFacts(windows, ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("fipsEnabledFacts(windows) = %#v, want %#v", got, want)
	}
}

func TestCurrentFIPSEnabledReadsWindowsRegistry(t *testing.T) {
	t.Parallel()

	host := &fakeHostOS{
		platform:        "windows",
		emptyRunDefault: true,
		runOutputs: map[string]string{
			fakeRunKey("reg", "query", `HKLM\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`, "/v", "Enabled"): strings.Join([]string{
				`HKEY_LOCAL_MACHINE\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`,
				"    Enabled    REG_DWORD    0xff",
			}, "\n"),
		},
	}
	s := NewSession()
	s.host = host

	if !currentFIPSEnabled(s, "") {
		t.Fatal("currentFIPSEnabled(windows) = false, want true")
	}
	want := []fakeHostRunCall{{name: "reg", args: []string{"query", `HKLM\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy`, "/v", "Enabled"}}}
	if !reflect.DeepEqual(host.runCalls, want) {
		t.Fatalf("commands = %#v, want reg query", host.runCalls)
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
