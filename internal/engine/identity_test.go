package engine

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestIdentityFactFromInfoWindowsOmitsPOSIXFields(t *testing.T) {
	t.Parallel()

	privileged := true
	got := identityFactFromInfo("windows", identityInfo{
		User:       `MG93C9IN9WKOITF\Administrator`,
		UID:        "S-1-5-21-uid",
		GID:        "S-1-5-21-gid",
		Group:      "Administrators",
		Privileged: &privileged,
	})

	want := map[string]any{
		"user":       `MG93C9IN9WKOITF\Administrator`,
		"privileged": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identityFactFromInfo(windows) = %#v, want %#v", got, want)
	}
}

func TestCurrentWindowsIdentityInfoUsesWhoamiCommands(t *testing.T) {
	t.Parallel()

	run := func(name string, args ...string) string {
		switch {
		case name == "whoami" && len(args) == 0:
			return `MG93C9IN9WKOITF\Administrator`
		case name == "whoami" && reflect.DeepEqual(args, []string{"/groups"}):
			return strings.Join([]string{
				`Group Name                                 Type             SID          Attributes`,
				`========================================== ================ ============ ===============================================`,
				`BUILTIN\Administrators                    Alias            S-1-5-32-544 Mandatory group, Enabled by default, Enabled group`,
			}, "\n")
		default:
			t.Fatalf("run = %s %v, want whoami or whoami /groups", name, args)
			return ""
		}
	}

	got := currentWindowsIdentityInfo(run, discardLog())
	if got.User != `MG93C9IN9WKOITF\Administrator` {
		t.Fatalf("User = %q, want administrator", got.User)
	}
	if got.Privileged == nil || !*got.Privileged {
		t.Fatalf("Privileged = %#v, want true", got.Privileged)
	}
}

func TestCurrentWindowsIdentityInfoLogsFailureWhenUserCannotResolveLikeRubyResolver(t *testing.T) {
	debugMessages := []string{}
	logger := captureLogger(&debugMessages, nil, nil)

	got := currentWindowsIdentityInfo(func(name string, args ...string) string {
		if name != "whoami" || len(args) != 0 {
			t.Fatalf("run = %s %v, want only whoami", name, args)
		}
		return ""
	}, logger)

	if got.User != "" {
		t.Fatalf("User = %q, want empty", got.User)
	}
	if got.Privileged != nil {
		t.Fatalf("Privileged = %#v, want nil", got.Privileged)
	}
	want := []string{"failure resolving identity facts: "}
	if !reflect.DeepEqual(debugMessages, want) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, want)
	}
}

func TestParseWindowsAdministratorGroupsDetectsDenyOnlyAdmin(t *testing.T) {
	t.Parallel()

	output := strings.Join([]string{
		`Group Name                                 Type             SID          Attributes`,
		`========================================== ================ ============ ===============================================`,
		`BUILTIN\Administrators                    Alias            S-1-5-32-544 Group used for deny only`,
	}, "\n")

	got, ok := parseWindowsAdministratorGroups(output)
	if !ok {
		t.Fatal("parseWindowsAdministratorGroups() ok = false, want true")
	}
	if got {
		t.Fatal("parseWindowsAdministratorGroups() = true, want false")
	}
}

func TestIdentityFactFromInfoPOSIXReturnsNumericUIDAndGID(t *testing.T) {
	t.Parallel()

	privileged := false
	got := identityFactFromInfo("linux", identityInfo{
		User:       "test1.test2",
		UID:        "501",
		GID:        "20",
		Group:      "staff",
		Privileged: &privileged,
	})

	want := map[string]any{
		"user":       "test1.test2",
		"uid":        501,
		"gid":        20,
		"group":      "staff",
		"privileged": false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identityFactFromInfo(posix) = %#v, want %#v", got, want)
	}
}

func TestCoreFacts_includeMacOSReleaseKernelHardwareAndIdentity(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skipf("macOS host fact integration runs only on darwin, not %s", runtime.GOOS)
	}
	collection := Collection(CoreFacts(NewSession()))
	osFact, ok := collection["os"].(map[string]any)
	if !ok {
		t.Fatalf("os = %#v, want map", collection["os"])
	}
	for _, tt := range []struct {
		key  string
		want string
	}{
		{key: "name", want: "Darwin"},
		{key: "family", want: "Darwin"},
	} {
		if got := osFact[tt.key]; got != tt.want {
			t.Fatalf("os.%s = %#v, want %q", tt.key, got, tt.want)
		}
	}
	for _, key := range []string{"architecture", "hardware"} {
		if got, ok := osFact[key].(string); !ok || got == "" {
			t.Fatalf("os.%s = %#v, want non-empty string", key, osFact[key])
		}
	}
	release, ok := osFact["release"].(map[string]any)
	if !ok || release["full"] == "" || release["major"] == "" {
		t.Fatalf("os.release = %#v, want full and major values", osFact["release"])
	}
	macosx, ok := osFact["macosx"].(map[string]any)
	if !ok {
		t.Fatalf("os.macosx = %#v, want map", osFact["macosx"])
	}
	for _, key := range []string{"product", "build"} {
		if got, ok := macosx[key].(string); !ok || got == "" {
			t.Fatalf("os.macosx.%s = %#v, want non-empty string", key, macosx[key])
		}
	}
	version, ok := macosx["version"].(map[string]any)
	if !ok || version["full"] == "" || version["major"] == "" {
		t.Fatalf("os.macosx.version = %#v, want full and major values", macosx["version"])
	}

	dmi, ok := collection["dmi"].(map[string]any)
	if !ok {
		t.Fatalf("dmi = %#v, want map", collection["dmi"])
	}
	product, ok := dmi["product"].(map[string]any)
	if !ok || product["name"] == "" {
		t.Fatalf("dmi.product = %#v, want product.name", dmi["product"])
	}
	identity, ok := collection["identity"].(map[string]any)
	if !ok {
		t.Fatalf("identity = %#v, want map", collection["identity"])
	}
	for _, key := range []string{"user", "uid", "gid", "group", "privileged"} {
		if _, ok := identity[key]; !ok {
			t.Fatalf("identity = %#v, want key %q", identity, key)
		}
	}
	if got, ok := collection["facterversion"].(string); !ok || got == "" {
		t.Fatalf("facterversion = %#v, want non-empty string", collection["facterversion"])
	}
	for _, legacy := range []string{"kernelrelease", "kernelversion", "kernelmajversion"} {
		if _, ok := collection[legacy]; ok {
			t.Fatalf("%s = %#v, want no flat kernel fact", legacy, collection[legacy])
		}
	}
	kernel, kernelOK := collection["kernel"].(map[string]any)
	if !kernelOK {
		t.Fatalf("kernel = %#v, want structured map", collection["kernel"])
	}
	if got, ok := kernel["name"].(string); !ok || got == "" {
		t.Fatalf("kernel.name = %#v, want non-empty string", kernel["name"])
	}
	release, releaseOK := kernel["release"].(map[string]any)
	if !releaseOK {
		t.Fatalf("kernel.release = %#v, want map", kernel["release"])
	}
	for _, key := range []string{"full", "major", "minor"} {
		if got, ok := release[key].(string); !ok || got == "" {
			t.Fatalf("kernel.release.%s = %#v, want non-empty string", key, release[key])
		}
	}
	version, versionOK := kernel["version"].(map[string]any)
	if !versionOK {
		t.Fatalf("kernel.version = %#v, want map", kernel["version"])
	}
	if got, ok := version["full"].(string); !ok || got == "" {
		t.Fatalf("kernel.version.full = %#v, want non-empty string", version["full"])
	}
	for _, key := range []string{"operatingsystem", "osfamily", "operatingsystemrelease", "architecture"} {
		if got, ok := collection[key]; ok {
			t.Fatalf("%s = %#v, want no legacy alias fact", key, got)
		}
	}
}
