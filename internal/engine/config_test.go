package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestParseConfig_returnsAllConfiguredSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `facts : {
  blocklist : [ "EC2", "networking" ],
  ttls : [
    { "timezone" : "30 days" },
  ],
}
global : {
  external-dir : [ "/opt/facts" ],
  no-external-facts : true,
  force-dot-resolution : true,
  sequential : true,
}
cli : {
  debug : true,
  verbose : true,
  log-level : "info",
}
fact-groups : {
  hardware : [ "dmi" ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Disabled:           []string{"ec2", "networking"},
		ExternalDirs:       []string{"/opt/facts"},
		Debug:              true,
		Verbose:            true,
		LogLevel:           "info",
		NoExternalFacts:    true,
		ForceDotResolution: true,
		Sequential:         true,
		SequentialSet:      true,
		TTLs:               []FactTTL{{Fact: "timezone", TTL: "30 days"}},
		FactGroups:         []FactGroup{{Name: "hardware", Facts: []string{"dmi"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseConfig() = %#v, want %#v", got, want)
	}
}

func TestCurrentDefaultExternalFactDirsUsesCurrentEnvironment(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	programData := filepath.Join(t.TempDir(), "ProgramData")

	got := currentDefaultExternalFactDirs("linux", 501, testConfigEnv(map[string]string{
		"HOME":        home,
		"ProgramData": programData,
	}))
	want := []string{
		home + "/.facts/facts.d",
		home + "/.facter/facts.d",
		home + "/.puppetlabs/opt/facter/facts.d",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentDefaultExternalFactDirs() = %#v, want %#v", got, want)
	}
}

func TestCurrentDefaultExternalFactDirsUsesRootDefaultsForRoot(t *testing.T) {
	got := currentDefaultExternalFactDirs("linux", 0, testConfigEnv(map[string]string{
		"HOME": filepath.Join(t.TempDir(), "home"),
	}))
	want := []string{
		"/etc/facts/facts.d",
		"/etc/puppetlabs/facter/facts.d",
		"/etc/facter/facts.d/",
		"/opt/puppetlabs/facter/facts.d",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentDefaultExternalFactDirs(root) = %#v, want %#v", got, want)
	}
}

func TestCurrentDefaultExternalFactDirsUsesProgramDataOnWindows(t *testing.T) {
	programData := filepath.Join(t.TempDir(), "ProgramData")

	got := currentDefaultExternalFactDirs("windows", -1, testConfigEnv(map[string]string{
		"HOME":        filepath.Join(t.TempDir(), "home"),
		"ProgramData": programData,
	}))
	want := []string{
		programData + "/facts/facts.d",
		programData + "/PuppetLabs/facter/facts.d",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentDefaultExternalFactDirs(windows) = %#v, want %#v", got, want)
	}
}

func TestCurrentDefaultExternalFactDirsMatchesRuntimeInputs(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	programData := filepath.Join(t.TempDir(), "ProgramData")
	t.Setenv("HOME", home)
	t.Setenv("ProgramData", programData)

	got := CurrentDefaultExternalFactDirs()
	var want []string
	switch {
	case runtime.GOOS == "windows":
		want = []string{programData + "/facts/facts.d", programData + "/PuppetLabs/facter/facts.d"}
	case os.Geteuid() == 0:
		want = []string{"/etc/facts/facts.d", "/etc/puppetlabs/facter/facts.d", "/etc/facter/facts.d/", "/opt/puppetlabs/facter/facts.d"}
	default:
		want = []string{home + "/.facts/facts.d", home + "/.facter/facts.d", home + "/.puppetlabs/opt/facter/facts.d"}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CurrentDefaultExternalFactDirs() = %#v, want %#v", got, want)
	}
}

func testConfigEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func TestParseConfig_ignoresSectionNamesInsideStringsAndComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `global : {
  note : "facts : { ttls : [ { \"wrong\" : \"1 day\" } ] }",
  facts : { ttls : [ { "nested" : "1 day" } ] },
}
# cli : { debug : true }
facts : {
  ttls : [
    { "right" : "2 days" },
  ],
}
cli : {
  verbose : true,
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.TTLs, []FactTTL{{Fact: "right", TTL: "2 days"}}) {
		t.Fatalf("TTLs = %#v, want real facts section only", got.TTLs)
	}
	if got.Debug {
		t.Fatal("Debug = true, want commented cli section ignored")
	}
	if !got.Verbose {
		t.Fatal("Verbose = false, want real cli section parsed")
	}
}

func TestParseConfig_collectsRepeatedDirectoryEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `global : {
  external-dir : [ "/first/external" ],
}
cli : {
  external-dir : [ "/second/external" ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/first/external", "/second/external"}; !reflect.DeepEqual(got.ExternalDirs, want) {
		t.Fatalf("ExternalDirs = %#v, want %#v", got.ExternalDirs, want)
	}
}

func TestParseConfig_ignoresRetiredCustomFactKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `global : {
  external-dir : [ "/kept/external" ],
  custom-dir : [ "/retired/custom" ],
  no-custom-facts : true,
  no-ruby : true,
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var warnings []string
	logger := captureLogger(nil, &warnings, nil)

	got, err := ParseConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{ExternalDirs: []string{"/kept/external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseConfig() = %#v, want retired keys inert: %#v", got, want)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want retired keys silently ignored", warnings)
	}
}

func TestDefaultExternalFactDirsSearchesNativePathsBeforeFacterCompatPaths(t *testing.T) {
	tests := []struct {
		name           string
		windows        bool
		root           bool
		home           string
		windowsDataDir string
		want           []string
	}{
		{
			name: "linux root",
			root: true,
			want: []string{
				"/etc/facts/facts.d",
				"/etc/puppetlabs/facter/facts.d",
				"/etc/facter/facts.d/",
				"/opt/puppetlabs/facter/facts.d",
			},
		},
		{
			name: "darwin root",
			root: true,
			want: []string{
				"/etc/facts/facts.d",
				"/etc/puppetlabs/facter/facts.d",
				"/etc/facter/facts.d/",
				"/opt/puppetlabs/facter/facts.d",
			},
		},
		{
			name: "freebsd root",
			root: true,
			want: []string{
				"/etc/facts/facts.d",
				"/etc/puppetlabs/facter/facts.d",
				"/etc/facter/facts.d/",
				"/opt/puppetlabs/facter/facts.d",
			},
		},
		{
			name:           "windows with data dir",
			windows:        true,
			root:           true,
			windowsDataDir: `C:\ProgramData`,
			want: []string{
				`C:\ProgramData/facts/facts.d`,
				`C:\ProgramData/PuppetLabs/facter/facts.d`,
			},
		},
		{
			name:    "windows without data dir",
			windows: true,
			root:    true,
			want:    nil,
		},
		{
			name: "non root with home",
			home: "/home/alice",
			want: []string{
				"/home/alice/.facts/facts.d",
				"/home/alice/.facter/facts.d",
				"/home/alice/.puppetlabs/opt/facter/facts.d",
			},
		},
		{
			name: "non root without home",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultExternalFactDirs(tt.windows, tt.root, tt.home, tt.windowsDataDir)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("DefaultExternalFactDirs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestPlatformDefaultConfigPathMatchesRubyConfigReader(t *testing.T) {
	tests := []struct {
		name string
		goos string
		want string
	}{
		{
			name: "linux",
			goos: "linux",
			want: "/etc/puppetlabs/facter/facter.conf",
		},
		{
			name: "darwin",
			goos: "darwin",
			want: "/etc/puppetlabs/facter/facter.conf",
		},
		{
			name: "freebsd",
			goos: "freebsd",
			want: "/etc/puppetlabs/facter/facter.conf",
		},
		{
			name: "windows",
			goos: "windows",
			want: "C:/ProgramData/PuppetLabs/facter/etc/facter.conf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := platformDefaultConfigPathFor(tt.goos)
			if got != tt.want {
				t.Fatalf("platformDefaultConfigPathFor(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestPlatformNativeDefaultConfigPath(t *testing.T) {
	tests := []struct {
		name string
		goos string
		want string
	}{
		{
			name: "linux",
			goos: "linux",
			want: "/etc/facts/facts.conf",
		},
		{
			name: "darwin",
			goos: "darwin",
			want: "/etc/facts/facts.conf",
		},
		{
			name: "freebsd",
			goos: "freebsd",
			want: "/etc/facts/facts.conf",
		},
		{
			name: "windows",
			goos: "windows",
			want: "C:/ProgramData/facts/facts.conf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := platformNativeDefaultConfigPathFor(tt.goos)
			if got != tt.want {
				t.Fatalf("platformNativeDefaultConfigPathFor(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

// TestParseConfig_nativeDefaultConfigDiscovery pins the default-config
// precedence of the facts-native input surface: with no explicit path, the
// facts-native facts.conf is consulted first, the facter-compatible
// facter.conf second, and the first existing file wins.
func TestParseConfig_nativeDefaultConfigDiscovery(t *testing.T) {
	dir := t.TempDir()
	nativePath := filepath.Join(dir, "facts.conf")
	compatPath := filepath.Join(dir, "facter.conf")
	nativeContent := []byte(`global : { external-dir : "/native/external" }`)
	compatContent := []byte(`global : { external-dir : "/compat/external" }`)

	tests := []struct {
		name   string
		native bool
		compat bool
		want   []string
	}{
		{name: "native only", native: true, want: []string{"/native/external"}},
		{name: "compat only", compat: true, want: []string{"/compat/external"}},
		{name: "both present native wins", native: true, compat: true, want: []string{"/native/external"}},
		{name: "neither", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.RemoveAll(nativePath); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(compatPath); err != nil {
				t.Fatal(err)
			}
			if tt.native {
				if err := os.WriteFile(nativePath, nativeContent, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tt.compat {
				if err := os.WriteFile(compatPath, compatContent, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			oldNative := NativeDefaultConfigPath
			oldCompat := DefaultConfigPath
			NativeDefaultConfigPath = func() string { return nativePath }
			DefaultConfigPath = func() string { return compatPath }
			t.Cleanup(func() {
				NativeDefaultConfigPath = oldNative
				DefaultConfigPath = oldCompat
			})

			got, err := ParseConfig("", discardLog())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.ExternalDirs, tt.want) {
				t.Fatalf("ExternalDirs = %#v, want %#v", got.ExternalDirs, tt.want)
			}
		})
	}
}

func TestParseConfig_acceptsBareDirectoryPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `global : {
  external-dir : [ /first/external, /second/external ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/first/external", "/second/external"}; !reflect.DeepEqual(got.ExternalDirs, want) {
		t.Fatalf("ExternalDirs = %#v, want %#v", got.ExternalDirs, want)
	}
}

func TestParseConfig_warnsAndIgnoresUnreadableConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.conf")
	warnings := []string{}
	logger := captureLogger(nil, &warnings, nil)

	got, err := ParseConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, Config{}) {
		t.Fatalf("ParseConfig() = %#v, want empty options", got)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want one warning", warnings)
	}
	if !strings.Contains(warnings[0], "Facts failed to read config file") {
		t.Fatalf("warning = %q, want config read warning", warnings[0])
	}
}

func TestParseConfig_warnsAndIgnoresInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	if err := os.WriteFile(path, []byte("some corrupt information"), 0o600); err != nil {
		t.Fatal(err)
	}
	warnings := []string{}
	logger := captureLogger(nil, &warnings, nil)

	got, err := ParseConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, Config{}) {
		t.Fatalf("ParseConfig() = %#v, want empty options", got)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want one warning", warnings)
	}
	if !strings.Contains(warnings[0], "Facts failed to read config file") {
		t.Fatalf("warning = %q, want config read warning", warnings[0])
	}
}

func TestParseConfig_emptyReadableConfigReturnsEmptySections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	warnings := []string{}
	logger := captureLogger(nil, &warnings, nil)

	options, err := ParseConfig(path, logger)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(options, Config{}) {
		t.Fatalf("ParseConfig() = %#v, want empty options", options)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	if config.Disabled != nil {
		t.Fatalf("Config.Disabled = %#v, want nil", config.Disabled)
	}
	if config.TTLs != nil {
		t.Fatalf("Config.TTLs = %#v, want nil", config.TTLs)
	}
	if config.FactGroups != nil {
		t.Fatalf("Config.FactGroups = %#v, want nil", config.FactGroups)
	}

	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}

func TestParseConfig_collectsRepeatedBooleanOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `global : {
	no-external-facts : false,
	force-dot-resolution : false,
}
cli : {
	no-external-facts : true,
	force-dot-resolution : true,
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	if !got.NoExternalFacts {
		t.Fatal("NoExternalFacts = false, want true")
	}
	if !got.ForceDotResolution {
		t.Fatal("ForceDotResolution = false, want true")
	}
}

func TestParseConfig_retiredShowLegacyKeyIsInert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `global : {
	show-legacy : true,
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	warnings := []string{}
	logger := captureLogger(nil, &warnings, nil)

	got, err := ParseConfig(path, logger)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v, want nil for retired show-legacy key", err)
	}
	if !reflect.DeepEqual(got, Config{}) {
		t.Fatalf("ParseConfig() = %#v, want zero options: show-legacy is retired and inert", got)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none for retired show-legacy key", warnings)
	}
}

func TestParseConfig_readsConfiguredSequentialLikeRubyOptionStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `global : {
	sequential : false,
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	if !got.SequentialSet {
		t.Fatal("SequentialSet = false, want true")
	}
	if got.Sequential {
		t.Fatal("Sequential = true, want false")
	}
}

func TestParseConfig_collectsRepeatedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `facts : {
  blocklist : [ "ec2", "os" ],
}
cli : {
  blocklist : [ "networking" ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.Disabled
	if want := []string{"ec2", "os", "networking"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.Disabled = %#v, want %#v", got, want)
	}
}

func TestParseConfig_nativeDisableKeyPopulatesDisabledSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.conf")
	content := `facts : {
  disable : [ "EC2", "packages" ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.Disabled
	if want := []string{"ec2", "packages"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.Disabled = %#v, want %#v", got, want)
	}
}

func TestParseConfig_nativeDisableKeySupersedesBlocklist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.conf")
	content := `facts : {
  blocklist : [ "networking" ],
  disable : [ "packages" ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.Disabled
	if want := []string{"packages"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.Disabled = %#v, want %#v (native disable must supersede blocklist)", got, want)
	}
}

func TestParseConfig_acceptsBareEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `facts : {
  blocklist : [ ec2, os.name ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.Disabled
	want := []string{"ec2", "os.name"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.Disabled = %#v, want %#v", got, want)
	}
}

func TestParseConfig_ignoresCommentedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `global : {
  # external-dir : [ "/commented/external" ],
  // external-dir : [ "/commented/external-two" ],
  no-external-facts : false,
  # no-external-facts : true,
}
facts : {
  blocklist : [ "os" ],
  // blocklist : [ "networking" ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	options, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	if len(options.ExternalDirs) != 0 {
		t.Fatalf("ExternalDirs = %#v, want commented entry ignored", options.ExternalDirs)
	}
	if options.NoExternalFacts {
		t.Fatal("NoExternalFacts = true, want commented true value ignored")
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	blocklist := config.Disabled
	if want := []string{"os"}; !reflect.DeepEqual(blocklist, want) {
		t.Fatalf("Config.Disabled = %#v, want %#v", blocklist, want)
	}
}

func TestParseConfig_returnsConfiguredFactTTLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `facts : {
  ttls : [
    { "timezone" : "30 days" },
    { "networking" : "1 hour" },
    { "operating system" : "30 minutes" }
  ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.TTLs
	want := []FactTTL{
		{Fact: "timezone", TTL: "30 days"},
		{Fact: "networking", TTL: "1 hour"},
		{Fact: "operating system", TTL: "30 minutes"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.TTLs = %#v, want %#v", got, want)
	}
}

func TestParseConfig_TTLsUseExactFactsSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `facts-extra : {
  ttls : [
    { "bad" : "1 hour" }
  ],
}
facts : {
  ttls : [
    { "timezone" : "30 days" }
  ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.TTLs
	want := []FactTTL{{Fact: "timezone", TTL: "30 days"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.TTLs = %#v, want %#v", got, want)
	}
}

func TestParseConfig_acceptsBareFactNamesAndValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `facts : {
  ttls : [
    { memory : 10000 },
    { hostname : 30 h }
  ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.TTLs
	want := []FactTTL{
		{Fact: "memory", TTL: "10000"},
		{Fact: "hostname", TTL: "30 h"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.TTLs = %#v, want %#v", got, want)
	}
}

func TestGroupTTLSeconds_convertsRubyCompatibleUnits(t *testing.T) {
	tests := []struct {
		name string
		ttls []FactTTL
		fact string
		want int64
	}{
		{name: "minutes", ttls: []FactTTL{{Fact: "operating system", TTL: "30 minutes"}}, fact: "operating system", want: 1800},
		{name: "nanoseconds", ttls: []FactTTL{{Fact: "operating system", TTL: "10000000000000 ns"}}, fact: "operating system", want: 10000},
		{name: "bare milliseconds", ttls: []FactTTL{{Fact: "memory", TTL: "10000"}}, fact: "memory", want: 10},
		{name: "short hours", ttls: []FactTTL{{Fact: "hostname", TTL: "30 h"}}, fact: "hostname", want: 108000},
		{name: "singular hour", ttls: []FactTTL{{Fact: "operating system", TTL: "1 hour"}}, fact: "operating system", want: 3600},
		{name: "singular day", ttls: []FactTTL{{Fact: "memory", TTL: "1 day"}}, fact: "memory", want: 86400},
		{name: "nanos alias", ttls: []FactTTL{{Fact: "operating system", TTL: "10000000000000 nanos"}}, fact: "operating system", want: 10000},
		{name: "micros alias", ttls: []FactTTL{{Fact: "memory", TTL: "10000000 micros"}}, fact: "memory", want: 10},
		{name: "milis alias", ttls: []FactTTL{{Fact: "hostname", TTL: "10000 milis"}}, fact: "hostname", want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GroupTTLSeconds(tt.ttls, tt.fact, discardLog())
			if !ok {
				t.Fatalf("GroupTTLSeconds(%q) did not find TTL", tt.fact)
			}
			if got != tt.want {
				t.Fatalf("GroupTTLSeconds(%q) = %d, want %d", tt.fact, got, tt.want)
			}
		})
	}
}

func TestTTLUnitScaleSupportsRubyCompatibleAliases(t *testing.T) {
	tests := []struct {
		unit       string
		multiplier int64
		divisor    int64
	}{
		{unit: "ns", multiplier: 1, divisor: 1_000_000_000},
		{unit: "nanosecond", multiplier: 1, divisor: 1_000_000_000},
		{unit: "nanoseconds", multiplier: 1, divisor: 1_000_000_000},
		{unit: "us", multiplier: 1, divisor: 1_000_000},
		{unit: "microsecond", multiplier: 1, divisor: 1_000_000},
		{unit: "microseconds", multiplier: 1, divisor: 1_000_000},
		{unit: "ms", multiplier: 1, divisor: 1_000},
		{unit: "millisecond", multiplier: 1, divisor: 1_000},
		{unit: "milliseconds", multiplier: 1, divisor: 1_000},
		{unit: "s", multiplier: 1, divisor: 1},
		{unit: "second", multiplier: 1, divisor: 1},
		{unit: "seconds", multiplier: 1, divisor: 1},
		{unit: "m", multiplier: 60, divisor: 1},
		{unit: "minute", multiplier: 60, divisor: 1},
		{unit: "h", multiplier: 3600, divisor: 1},
		{unit: "hours", multiplier: 3600, divisor: 1},
		{unit: "d", multiplier: 86400, divisor: 1},
		{unit: "days", multiplier: 86400, divisor: 1},
	}
	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			multiplier, divisor, ok := ttlUnitScale(tt.unit)
			if !ok || multiplier != tt.multiplier || divisor != tt.divisor {
				t.Fatalf("ttlUnitScale(%q) = %d, %d, %v; want %d, %d, true", tt.unit, multiplier, divisor, ok, tt.multiplier, tt.divisor)
			}
		})
	}
	if _, _, ok := ttlUnitScale("fortnight"); ok {
		t.Fatal("ttlUnitScale(fortnight) ok = true, want false")
	}
	if got := rubyTTLLogUnit("fortnight"); got != "fortnights" {
		t.Fatalf("rubyTTLLogUnit(fortnight) = %q, want fortnights", got)
	}
	if got := rubyTTLLogUnit("ms"); got != "ms" {
		t.Fatalf("rubyTTLLogUnit(ms) = %q, want ms", got)
	}
}

func TestFactGroupName_returnsGroupContainingFact(t *testing.T) {
	groups := []FactGroup{{Name: "operating system", Facts: []string{"os", "os.name"}}}

	got, ok := FactGroupName(groups, "os")
	if !ok {
		t.Fatal("FactGroupName(os) did not find group")
	}
	if got != "operating system" {
		t.Fatalf("FactGroupName(os) = %q, want %q", got, "operating system")
	}

	if got, ok := FactGroupName(groups, "memory"); ok {
		t.Fatalf("FactGroupName(memory) = %q, true; want false", got)
	}
}

func TestMergeFactGroupsReplacesDefaultsAndAppendsNewGroups(t *testing.T) {
	defaults := []FactGroup{
		{Name: "operating system", Facts: []string{"os"}},
		{Name: "memory", Facts: []string{"memory"}},
	}
	configured := []FactGroup{
		{Name: "operating system", Facts: []string{"kernel"}},
		{Name: "site", Facts: []string{"site_role"}},
	}

	got := MergeFactGroups(defaults, configured)
	want := []FactGroup{
		{Name: "operating system", Facts: []string{"kernel"}},
		{Name: "memory", Facts: []string{"memory"}},
		{Name: "site", Facts: []string{"site_role"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeFactGroups() = %#v, want %#v", got, want)
	}
	if got := MergeFactGroups(defaults, nil); !reflect.DeepEqual(got, defaults) {
		t.Fatalf("MergeFactGroups(defaults, nil) = %#v, want %#v", got, defaults)
	}
}

func TestFactGroupName_returnsGroupContainingDescendantFact(t *testing.T) {
	groups := []FactGroup{{Name: "operating system", Facts: []string{"os", "os.name"}}}

	got, ok := FactGroupName(groups, "os.name.full")
	if !ok {
		t.Fatal("FactGroupName(os.name.full) did not find group")
	}
	if got != "operating system" {
		t.Fatalf("FactGroupName(os.name.full) = %q, want %q", got, "operating system")
	}
}

func TestFormatFactGroupsRendersRubyCompatibleRows(t *testing.T) {
	groups := []FactGroup{
		{Name: "hardware", Facts: []string{"dmi", "processors"}},
		{Name: "path"},
	}

	got := FormatFactGroups(groups)
	want := "hardware\n- dmi\n- processors\npath"
	if got != want {
		t.Fatalf("FormatFactGroups() = %q, want %q", got, want)
	}
}

func TestGroupTTLSeconds_returnsFalseForMissingOrInvalidTTL(t *testing.T) {
	tests := []struct {
		name string
		ttls []FactTTL
		fact string
	}{
		{name: "missing", ttls: []FactTTL{{Fact: "operating system", TTL: "30 minutes"}}, fact: "memory"},
		{name: "invalid unit", ttls: []FactTTL{{Fact: "hostname", TTL: "30 invalid_unit"}}, fact: "hostname"},
		{name: "invalid number", ttls: []FactTTL{{Fact: "hostname", TTL: "many hours"}}, fact: "hostname"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := GroupTTLSeconds(tt.ttls, tt.fact, discardLog()); ok {
				t.Fatalf("GroupTTLSeconds(%q) = %d, true; want false", tt.fact, got)
			}
		})
	}
}

func TestGroupTTLSeconds_logsRubyCompatibleInvalidUnitError(t *testing.T) {
	errors := []string{}
	logger := captureLogger(nil, nil, &errors)

	if got, ok := GroupTTLSeconds([]FactTTL{{Fact: "hostname", TTL: "30 invalid_unit"}}, "hostname", logger); ok {
		t.Fatalf("GroupTTLSeconds(hostname) = %d, true; want false", got)
	}
	if len(errors) != 1 {
		t.Fatalf("errors = %#v, want one error", errors)
	}
	want := "Could not parse time unit invalid_units (try ns, nanos, nanoseconds, us, micros, microseconds, ms, milis, milliseconds, s, seconds, m, minutes, h, hours, d, days)"
	if errors[0] != want {
		t.Fatalf("error = %q, want %q", errors[0], want)
	}
}

func TestGroupTTLSecondsAcceptsCaseInsensitiveUnits(t *testing.T) {
	tests := []struct {
		ttl  string
		want int64
	}{
		{ttl: "1000000000 NANOSECOND", want: 1},
		{ttl: "1000000 MICROSECOND", want: 1},
		{ttl: "1000 MILLISECOND", want: 1},
		{ttl: "2 SECOND", want: 2},
		{ttl: "2 MINUTE", want: 120},
		{ttl: "2 HOUR", want: 7200},
		{ttl: "2 DAY", want: 172800},
	}

	for _, tt := range tests {
		t.Run(tt.ttl, func(t *testing.T) {
			got, ok := GroupTTLSeconds([]FactTTL{{Fact: "os", TTL: tt.ttl}}, "os", nil)
			if !ok || got != tt.want {
				t.Fatalf("GroupTTLSeconds(%q) = %d, %v; want %d, true", tt.ttl, got, ok, tt.want)
			}
		})
	}
}

func TestGroupTTLSecondsRejectsMalformedTTLTokens(t *testing.T) {
	for _, ttl := range []string{"", "+", "-", "999999999999999999999999999999999999 seconds", "1 hour extra"} {
		t.Run(ttl, func(t *testing.T) {
			if got, ok := GroupTTLSeconds([]FactTTL{{Fact: "os", TTL: ttl}}, "os", discardLog()); ok {
				t.Fatalf("GroupTTLSeconds(%q) = %d, true; want false", ttl, got)
			}
		})
	}
}

func TestDisabledFactsForFiltering_retiredLegacyGroupBlocksNothing(t *testing.T) {
	blocked := DisabledFactsWithGroups([]string{"legacy"}, nil)

	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin"},
		{Name: "networking.hostname", Value: "host.example.com"},
		{Name: "processors.count", Value: 8},
	}
	got := FilterDisabledFacts(facts, blocked)
	if !reflect.DeepEqual(got, facts) {
		t.Fatalf("FilterDisabledFacts(legacy blocklist) = %#v, want discovery unchanged %#v", got, facts)
	}
}

func TestDisabledFactsWithGroups_expandsGroupWithoutBlockingGroupName(t *testing.T) {
	got := DisabledFactsWithGroups([]string{"blocked_group", "blocked_fact"}, []FactGroup{{Name: "blocked_group", Facts: []string{"fact1", "fact2"}}})
	want := map[string]bool{"fact1": true, "fact2": true, "blocked_fact": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DisabledFactsWithGroups() = %#v, want %#v", got, want)
	}
}

func TestFilterDisabledFacts_blocksExactNameAndRoot(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin", Type: "core"},
		{Name: "networking.hostname", Value: "host.example.com", Type: "core"},
	}

	got := FilterDisabledFacts(facts, map[string]bool{"os.name": true})
	want := []ResolvedFact{{Name: "networking.hostname", Value: "host.example.com", Type: "core"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterDisabledFacts(os.name) = %#v, want %#v", got, want)
	}

	got = FilterDisabledFacts(facts, map[string]bool{"networking": true})
	want = []ResolvedFact{{Name: "os.name", Value: "Darwin", Type: "core"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterDisabledFacts(networking) = %#v, want %#v", got, want)
	}
}

func TestFilterDisabledFacts_prunesDisabledDescendantsFromStructuredParents(t *testing.T) {
	facts := []ResolvedFact{
		{
			Name: "os",
			Value: map[string]any{
				"name": "Ubuntu",
				"release": map[string]any{
					"full":  "24.04",
					"major": "24",
				},
			},
			Type: "core",
		},
	}

	got := FilterDisabledFacts(facts, map[string]bool{"os.release.major": true})
	want := []ResolvedFact{
		{
			Name: "os",
			Value: map[string]any{
				"name": "Ubuntu",
				"release": map[string]any{
					"full": "24.04",
				},
			},
			Type: "core",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterDisabledFacts(os.release.major) = %#v, want %#v", got, want)
	}
}

func TestFilterDisabledFactsPrunesEmptyMapsAndKeepsOriginalValue(t *testing.T) {
	original := map[string]any{
		"name": "Ubuntu",
		"release": map[string]any{
			"full":  "24.04",
			"major": "24",
		},
	}
	facts := []ResolvedFact{{Name: "os", Value: original, Type: "core"}}

	got := FilterDisabledFacts(facts, map[string]bool{
		"os.release.full":    true,
		"os.release.major":   true,
		"os.missing.nothing": true,
	})
	want := []ResolvedFact{{Name: "os", Value: map[string]any{"name": "Ubuntu"}, Type: "core"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterDisabledFacts() = %#v, want %#v", got, want)
	}
	if _, ok := original["release"]; !ok {
		t.Fatalf("original value = %#v, want unpruned source map", original)
	}
	if got := pruneDottedValue(map[string]any{"name": "Ubuntu"}, nil); !reflect.DeepEqual(got, map[string]any{"name": "Ubuntu"}) {
		t.Fatalf("pruneDottedValue(empty path) = %#v, want unchanged value", got)
	}
}

func TestParseConfig_returnsConfiguredFactGroups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `fact-groups : {
  cached-custom-facts : [ "site_role", "site_location" ],
  hardware : [ "dmi" ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.FactGroups
	want := []FactGroup{
		{Name: "cached-custom-facts", Facts: []string{"site_role", "site_location"}},
		{Name: "hardware", Facts: []string{"dmi"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.FactGroups = %#v, want %#v", got, want)
	}
}

func TestParseConfig_acceptsQuotedGroupNamesWithSpaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `fact-groups : {
  "operating system" : [ "os", "os.name" ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.FactGroups
	want := []FactGroup{{Name: "operating system", Facts: []string{"os", "os.name"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.FactGroups = %#v, want %#v", got, want)
	}
}

func TestParseConfig_acceptsBareFactNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `fact-groups : {
  cached-custom-facts : [ site_role, site_location ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.FactGroups
	want := []FactGroup{{Name: "cached-custom-facts", Facts: []string{"site_role", "site_location"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.FactGroups = %#v, want %#v", got, want)
	}
}

func TestParseConfig_acceptsScalarFactGroupValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := `fact-groups : {
  kernel : kernelversion,
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(path, discardLog())
	if err != nil {
		t.Fatal(err)
	}
	got := config.FactGroups
	want := []FactGroup{{Name: "kernel", Facts: []string{"kernelversion"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.FactGroups = %#v, want %#v", got, want)
	}
}

// TestConfigParser_pinnedSubsetBoundary pins the supported facter.conf
// subset boundary documented in docs/FACTER_CONF_COMPATIBILITY.md: accepted
// syntax on the supported side, and the behavior of general-HOCON features
// (includes, substitutions) the Go port does not implement.
func TestConfigParser_pinnedSubsetBoundary(t *testing.T) {
	writeConfig := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "facter.conf")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("equals separators and json style are accepted", func(t *testing.T) {
		path := writeConfig(t, `global = {
  external-dir = [ "/json/external" ]
}`)
		got, err := ParseConfig(path, discardLog())
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"/json/external"}; !reflect.DeepEqual(got.ExternalDirs, want) {
			t.Fatalf("ExternalDirs = %#v, want %#v", got.ExternalDirs, want)
		}
	})

	t.Run("single-quoted strings are accepted leniently", func(t *testing.T) {
		path := writeConfig(t, `cli : {
  log-level : 'trace',
}`)
		got, err := ParseConfig(path, discardLog())
		if err != nil {
			t.Fatal(err)
		}
		if got.LogLevel != "trace" {
			t.Fatalf("LogLevel = %q, want %q", got.LogLevel, "trace")
		}
	})

	t.Run("substitutions are never expanded", func(t *testing.T) {
		path := writeConfig(t, `base-dir : "/resolved"
global : {
  external-dir : [ "${base-dir}" ],
}`)
		got, err := ParseConfig(path, discardLog())
		if err != nil {
			t.Fatal(err)
		}
		for _, dir := range got.ExternalDirs {
			if dir == "/resolved" {
				t.Fatalf("ExternalDirs = %#v, substitution must not be expanded", got.ExternalDirs)
			}
		}
	})

	t.Run("include directives are not processed", func(t *testing.T) {
		dir := t.TempDir()
		included := filepath.Join(dir, "included.conf")
		if err := os.WriteFile(included, []byte(`global : { external-dir : [ "/from/include" ] }`), 0o600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "facter.conf")
		content := "include \"" + included + "\"\nglobal : {\n  external-dir : [ \"/direct/external\" ],\n}"
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		got, err := ParseConfig(path, discardLog())
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"/direct/external"}; !reflect.DeepEqual(got.ExternalDirs, want) {
			t.Fatalf("ExternalDirs = %#v, include must not be processed: want %#v", got.ExternalDirs, want)
		}
	})

	t.Run("config without key separators warns and is ignored", func(t *testing.T) {
		path := writeConfig(t, "this is not hocon at all")
		var warnings []string
		logger := captureLogger(nil, &warnings, nil)

		got, err := ParseConfig(path, logger)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, Config{}) {
			t.Fatalf("ParseConfig() = %#v, want empty options", got)
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], "invalid config") {
			t.Fatalf("warnings = %#v, want one invalid config warning", warnings)
		}
	})

	t.Run("inline comments after values are ignored", func(t *testing.T) {
		path := writeConfig(t, `global : {
  external-dir : [ "/kept" ], # comment with , and : inside
  no-external-facts : true // trailing comment
}`)
		got, err := ParseConfig(path, discardLog())
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"/kept"}; !reflect.DeepEqual(got.ExternalDirs, want) {
			t.Fatalf("ExternalDirs = %#v, want %#v", got.ExternalDirs, want)
		}
		if !got.NoExternalFacts {
			t.Fatal("NoExternalFacts = false, want true")
		}
	})

	t.Run("comment at end of file is ignored", func(t *testing.T) {
		path := writeConfig(t, `global : {
  external-dir : [ "/kept" ],
}
# no newline`)
		got, err := ParseConfig(path, discardLog())
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"/kept"}; !reflect.DeepEqual(got.ExternalDirs, want) {
			t.Fatalf("ExternalDirs = %#v, want %#v", got.ExternalDirs, want)
		}
	})
}

func TestFirstConfigValueReturnsFirstNonEmptyValue(t *testing.T) {
	if got, want := firstConfigValue("", "", "kept", "ignored"), "kept"; got != want {
		t.Fatalf("firstConfigValue() = %q, want %q", got, want)
	}
	if got := firstConfigValue("", ""); got != "" {
		t.Fatalf("firstConfigValue(all empty) = %q, want empty", got)
	}
}

func TestDisabledUnion_mergesConfigFlagAndEnvSources(t *testing.T) {
	config := Config{Disabled: []string{"os"}}
	got := DisabledUnion(config, []string{"processors"}, []string{"FACTS_DISABLE=networking"})
	for _, name := range []string{"os", "processors", "networking"} {
		if !got[name] {
			t.Fatalf("DisabledUnion() missing %q from the union; got %#v", name, got)
		}
	}
}

func TestDisabledUnion_nilEnvironExcludesEnvSource(t *testing.T) {
	config := Config{Disabled: []string{"os"}}
	got := DisabledUnion(config, nil, nil)
	if !got["os"] {
		t.Fatalf("DisabledUnion() dropped config.Disabled; got %#v", got)
	}
	if got["networking"] {
		t.Fatalf("DisabledUnion(nil environ) included an env source; got %#v", got)
	}
}

func TestDisabledUnion_expandsGroups(t *testing.T) {
	config := Config{
		Disabled:   []string{"web"},
		FactGroups: []FactGroup{{Name: "web", Facts: []string{"networking", "os.name"}}},
	}
	got := DisabledUnion(config, nil, nil)
	for _, name := range []string{"networking", "os.name"} {
		if !got[name] {
			t.Fatalf("DisabledUnion() did not expand group member %q; got %#v", name, got)
		}
	}
}
