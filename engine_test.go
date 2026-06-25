package facts

// Engine-level contract tests for the facts library API (openspec change
// introduce-facts-library-api, task 3.7). These pin the facts-library-api
// spec scenarios and the engine-classified behaviors from
// openspec/changes/introduce-facts-library-api/test-migration-map.md.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// hermeticSnapshot is discovered once and shared by read-only query tests;
// discovery re-probes the host, so per-test discovery would be needlessly
// slow.
var hermeticSnapshot = sync.OnceValue(func() *Snapshot {
	eng, err := New()
	if err != nil {
		panic(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		panic(err)
	}
	return snap
})

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNew_defaultEngineIsHermetic(t *testing.T) {
	t.Setenv("FACTER_hermetic_probe", "leaked")

	eng, err := New()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if _, err := snap.Value("os.family"); err != nil {
		t.Fatalf("Value(os.family) err = %v, want core fact", err)
	}
	if _, err := snap.Value("hermetic_probe"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(hermetic_probe) err = %v, want ErrFactNotFound from hermetic engine", err)
	}
}

func TestNewIgnoresNilOptionsAndReturnsOptionErrors(t *testing.T) {
	if eng, err := New(nil); err != nil || eng == nil {
		t.Fatalf("New(nil) = %#v, %v; want engine, nil", eng, err)
	}
	if eng, err := New(WithConfigFile("")); err == nil || eng != nil {
		t.Fatalf("New(WithConfigFile(empty)) = %#v, %v; want nil engine and error", eng, err)
	}
}

func TestEngineDiscover_uninitializedReceiverReturnsError(t *testing.T) {
	var nilEngine *Engine
	if snap, err := nilEngine.Discover(context.Background()); err == nil || snap != nil {
		t.Fatalf("nil Engine Discover() = %#v, %v, want nil snapshot and error", snap, err)
	}

	var zero Engine
	if snap, err := zero.Discover(context.Background()); err == nil || snap != nil {
		t.Fatalf("zero Engine Discover() = %#v, %v, want nil snapshot and error", snap, err)
	}
}

func TestWithExternalDirs_loadsExactlyOptedDirs(t *testing.T) {
	t.Setenv("FACTER_env_probe", "leaked")
	dir := t.TempDir()
	writeTestFile(t, dir, "site.txt", "site_location=lab\n")

	eng, err := New(WithExternalDirs(dir))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if got, err := snap.Value("site_location"); err != nil || got != "lab" {
		t.Fatalf("Value(site_location) = %#v, %v, want lab", got, err)
	}
	if _, err := snap.Value("env_probe"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(env_probe) err = %v, want environment facts excluded", err)
	}
}

func TestFacterlibHasNoEffectOnDiscovery(t *testing.T) {
	facterlib := t.TempDir()
	writeTestFile(t, facterlib, "extra.rb", "Facter.add(:facterlib_probe) do\n  setcode do\n    'leaked'\n  end\nend\n")
	t.Setenv("FACTERLIB", facterlib)

	eng, err := New(WithSystemDefaults())
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background(), "facterlib_probe")
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if _, err := snap.Value("facterlib_probe"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(facterlib_probe) err = %v, want FACTERLIB ignored", err)
	}
}

func TestWithConfigFile_loadsConfiguredDirs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "site.txt", "site_location=lab\n")
	configPath := writeTestFile(t, t.TempDir(), "facter.conf", "global : {\n  external-dir : [ \""+dir+"\" ],\n}\n")

	eng, err := New(WithConfigFile(configPath))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if got, err := snap.Value("site_location"); err != nil || got != "lab" {
		t.Fatalf("Value(site_location) = %#v, %v, want configured external dir loaded", got, err)
	}
}

func TestWithConfigFile_recomputesDiscoveryPolicyEachDiscover(t *testing.T) {
	cacheDir := redirectCacheDir(t)
	firstDir := t.TempDir()
	writeTestFile(t, firstDir, "site.txt", "site_location=first\n")
	writeTestFile(t, firstDir, "blocked.txt", "blocked_probe=blocked\n")
	secondDir := t.TempDir()
	writeTestFile(t, secondDir, "site.txt", "site_location=second\nblocked_probe=visible\n")
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "facter.conf")
	writeTestFile(t, configDir, "facter.conf", `global : {
  external-dir : [ "`+firstDir+`" ],
}
facts : {
  blocklist : [ "blocked.txt" ],
}
`)

	eng, err := New(
		WithCache(),
		WithConfigFile(configPath),
		WithFact("cache_probe", func(context.Context) (any, error) { return "cached", nil }),
	)
	if err != nil {
		t.Fatal(err)
	}
	first, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("first Discover() err = %v", err)
	}
	if got, err := first.Value("site_location"); err != nil || got != "first" {
		t.Fatalf("first Value(site_location) = %#v, %v, want first", got, err)
	}
	if _, err := first.Value("blocked_probe"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("first Value(blocked_probe) err = %v, want ErrFactNotFound", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "cache_probe")); !os.IsNotExist(err) {
		t.Fatalf("cache file after first Discover stat err = %v, want not exist", err)
	}

	writeTestFile(t, configDir, "facter.conf", `global : {
  external-dir : [ "`+secondDir+`" ],
}
facts : {
  ttls : [ { "cache_probe" : "30 days" } ],
}
`)
	second, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("second Discover() err = %v", err)
	}
	if got, err := second.Value("site_location"); err != nil || got != "second" {
		t.Fatalf("second Value(site_location) = %#v, %v, want second", got, err)
	}
	if got, err := second.Value("blocked_probe"); err != nil || got != "visible" {
		t.Fatalf("second Value(blocked_probe) = %#v, %v, want visible", got, err)
	}
	data := readCacheFile(t, filepath.Join(cacheDir, "cache_probe"))
	if data["cache_probe"] != "cached" {
		t.Fatalf("cached cache_probe = %#v, want cached", data["cache_probe"])
	}
}

func TestWithConfigFile_blocklistSuppressesFacts(t *testing.T) {
	configPath := writeTestFile(t, t.TempDir(), "facter.conf", "facts : {\n  blocklist : [ \"ssh\" ],\n}\n")

	eng, err := New(WithConfigFile(configPath))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if _, err := snap.Value("ssh"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(ssh) err = %v, want blocklisted fact suppressed", err)
	}
}

func TestWithSystemDefaults_loadsEnvironmentFacts(t *testing.T) {
	t.Setenv("FACTER_env_probe", "from-env")

	eng, err := New(WithSystemDefaults())
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background(), "env_probe")
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if got, err := snap.Value("env_probe"); err != nil || got != "from-env" {
		t.Fatalf("Value(env_probe) = %#v, %v, want environment fact under system defaults", got, err)
	}
}

func TestDiscover_honorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eng, err := New()
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(ctx)
	if !errors.Is(err, ctx.Err()) {
		t.Fatalf("Discover() err = %v, want errors.Is(err, %v)", err, ctx.Err())
	}
	if snap == nil {
		t.Fatal("Discover() snapshot = nil, want partial snapshot")
	}
}

func TestDiscover_cancellationStopsCommandResolvers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	eng, err := New(WithFact("canceller", func(ctx context.Context) (any, error) {
		cancel()
		return "set", nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, err = eng.Discover(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Discover() err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("Discover() took %v after cancellation, want prompt return", elapsed)
	}
}

func TestDiscover_freshnessRequiresRediscovery(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "site.txt", "site_location=old\n")
	eng, err := New(WithExternalDirs(dir))
	if err != nil {
		t.Fatal(err)
	}

	before, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, dir, "site.txt", "site_location=new\n")

	if got, _ := before.Value("site_location"); got != "old" {
		t.Fatalf("existing snapshot Value(site_location) = %#v, want unchanged old", got)
	}
	after, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := after.Value("site_location"); got != "new" {
		t.Fatalf("new snapshot Value(site_location) = %#v, want new", got)
	}
}

func TestEngines_areIsolated(t *testing.T) {
	dirA := t.TempDir()
	writeTestFile(t, dirA, "isolated.txt", "isolated_role=role-a\n")
	dirB := t.TempDir()
	writeTestFile(t, dirB, "isolated.txt", "isolated_role=role-b\n")

	engA, err := New(WithExternalDirs(dirA), WithFact("isolated_fact", func(context.Context) (any, error) { return "a", nil }))
	if err != nil {
		t.Fatal(err)
	}
	engB, err := New(WithExternalDirs(dirB), WithFact("isolated_fact", func(context.Context) (any, error) { return "b", nil }))
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 3 {
		for _, tc := range []struct {
			eng      *Engine
			wantRole string
			wantFact string
		}{
			{engA, "role-a", "a"},
			{engB, "role-b", "b"},
		} {
			wg.Go(func() {
				snap, err := tc.eng.Discover(context.Background(), "isolated_role", "isolated_fact")
				if err != nil {
					t.Errorf("Discover() err = %v", err)
					return
				}
				if got, _ := snap.Value("isolated_role"); got != tc.wantRole {
					t.Errorf("Value(isolated_role) = %#v, want %q", got, tc.wantRole)
				}
				if got, _ := snap.Value("isolated_fact"); got != tc.wantFact {
					t.Errorf("Value(isolated_fact) = %#v, want %q", got, tc.wantFact)
				}
			})
		}
	}
	wg.Wait()
}

func TestSnapshotValue_dottedQueryMatchesTree(t *testing.T) {
	snap := hermeticSnapshot()

	osFact, ok := snap.Tree()["os"].(map[string]any)
	if !ok {
		t.Fatalf("Tree()[os] = %#v, want structured os fact", snap.Tree()["os"])
	}
	got, err := snap.Value("os.name")
	if err != nil {
		t.Fatalf("Value(os.name) err = %v", err)
	}
	if got != osFact["name"] || got == "" {
		t.Fatalf("Value(os.name) = %#v, want tree os.name %#v", got, osFact["name"])
	}
}

func TestSnapshotValue_missingNestedQueryDoesNotReturnNearestFact(t *testing.T) {
	snap := hermeticSnapshot()

	if _, err := snap.Value("os.no_such_key"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(os.no_such_key) err = %v, want ErrFactNotFound", err)
	}
}

func TestSnapshotValue_missingVersusNilValuedFact(t *testing.T) {
	eng, err := New(WithFact("nil_fact", func(context.Context) (any, error) { return nil, nil }))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	value, err := snap.Value("nil_fact")
	if err != nil || value != nil {
		t.Fatalf("Value(nil_fact) = %#v, %v, want (nil, nil) for resolved-nil fact", value, err)
	}
	if _, err := snap.Value("missing_fact"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(missing_fact) err = %v, want ErrFactNotFound", err)
	}
}

func TestDiscover_notApplicableFactsAreAbsentNotErrors(t *testing.T) {
	snap := hermeticSnapshot()

	if value, err := snap.Value("ec2_metadata"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(ec2_metadata) = %#v, %v, want not-applicable fact absent", value, err)
	}
}

func TestDiscover_partialFailureReturnsFactsAndJoinedError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exercises POSIX execute-bit semantics; covered on the POSIX platform gates")
	}
	dir := t.TempDir()
	writeTestFile(t, dir, "good.txt", "good_fact=ok\n")
	failing := filepath.Join(dir, "failing_fact")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	eng, err := New(WithExternalDirs(dir))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err == nil {
		t.Fatal("Discover() err = nil, want joined failure for failing executable")
	}
	if !strings.Contains(err.Error(), "failing_fact") {
		t.Fatalf("Discover() err = %v, want failure identifying failing_fact", err)
	}
	if got, valueErr := snap.Value("good_fact"); valueErr != nil || got != "ok" {
		t.Fatalf("Value(good_fact) = %#v, %v, want partial snapshot with resolved fact", got, valueErr)
	}
}

func TestWithFact_resolverErrorIsPartialFailure(t *testing.T) {
	boom := errors.New("boom")
	eng, err := New(WithFact("exploding_fact", func(context.Context) (any, error) { return nil, boom }))
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("Discover() err = %v, want joined resolver error", err)
	}
	if !strings.Contains(err.Error(), "exploding_fact") {
		t.Fatalf("Discover() err = %v, want failure identifying exploding_fact", err)
	}
	if _, valueErr := snap.Value("os.family"); valueErr != nil {
		t.Fatalf("Value(os.family) err = %v, want partial snapshot with core facts", valueErr)
	}
}

func TestWithFact_overridesCoreFactAndLosesToExternal(t *testing.T) {
	eng, err := New(WithFact("facterversion", func(context.Context) (any, error) { return "custom-version", nil }))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := snap.Value("facterversion"); got != "custom-version" {
		t.Fatalf("Value(facterversion) = %#v, want programmatic fact overriding core", got)
	}

	dir := t.TempDir()
	writeTestFile(t, dir, "site.txt", "facterversion=external-version\n")
	eng, err = New(
		WithFact("facterversion", func(context.Context) (any, error) { return "custom-version", nil }),
		WithExternalDirs(dir),
	)
	if err != nil {
		t.Fatal(err)
	}
	snap, err = eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := snap.Value("facterversion"); got != "external-version" {
		t.Fatalf("Value(facterversion) = %#v, want external fact overriding programmatic fact", got)
	}
}

func TestWithFact_normalizesValues(t *testing.T) {
	eng, err := New(
		WithFact("build_time", func(context.Context) (any, error) {
			return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), nil
		}),
		WithFact("typed_map", func(context.Context) (any, error) {
			return map[string]string{"role": "web"}, nil
		}),
		WithFact("arr_fact", func(context.Context) (any, error) {
			return []any{"x", "y", "z"}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if got, _ := snap.Value("build_time"); got != "2020-01-01T00:00:00Z" {
		t.Fatalf("Value(build_time) = %#v, want ISO 8601 normalization", got)
	}
	if got, _ := snap.Value("typed_map.role"); got != "web" {
		t.Fatalf("Value(typed_map.role) = %#v, want normalized string-keyed map", got)
	}
	if got, _ := snap.Value("arr_fact.0"); got != "x" {
		t.Fatalf("Value(arr_fact.0) = %#v, want array index query", got)
	}
	if _, err := snap.Value("arr_fact.3"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(arr_fact.3) err = %v, want out-of-range index missing", err)
	}
}

func TestNew_rejectsInvalidFactNames(t *testing.T) {
	if _, err := New(WithFact("null\x00byte", func(context.Context) (any, error) { return nil, nil })); err == nil {
		t.Fatal("New() err = nil, want null-byte fact name rejected")
	}
	if _, err := New(WithFact("", func(context.Context) (any, error) { return nil, nil })); err == nil {
		t.Fatal("New() err = nil, want empty fact name rejected")
	}
}

func TestWithFact_nullByteValueResolvesToNil(t *testing.T) {
	eng, err := New(WithFact("tainted_fact", func(context.Context) (any, error) { return "null\x00byte", nil }))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if value, err := snap.Value("tainted_fact"); err != nil || value != nil {
		t.Fatalf("Value(tainted_fact) = %#v, %v, want resolved nil for null-byte value", value, err)
	}
}

func TestExternalFactOverridesRegisteredAndCore(t *testing.T) {
	externalDir := t.TempDir()
	writeTestFile(t, externalDir, "site.txt", "site_role=from-external\n")

	eng, err := New(
		WithFact("site_role", func(context.Context) (any, error) { return "from-registered", nil }),
		WithExternalDirs(externalDir),
	)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := snap.Value("site_role"); got != "from-external" {
		t.Fatalf("Value(site_role) = %#v, want external fact winning precedence", got)
	}
}

func TestAs_decodesCanonicalShapes(t *testing.T) {
	externalDir := t.TempDir()
	writeTestFile(t, externalDir, "site.yaml", "site_owner:\n  team: platform\n  oncall: true\n")

	eng, err := New(
		WithFact("site_meta", func(context.Context) (any, error) {
			return map[string]any{"role": "web", "tier": 2}, nil
		}),
		WithExternalDirs(externalDir),
	)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	type osFact struct {
		Name   string `json:"name"`
		Family string `json:"family"`
	}
	if got, err := As[osFact](snap, "os"); err != nil || got.Name == "" || got.Family == "" {
		t.Fatalf("As[osFact](os) = %#v, %v, want decoded core fact", got, err)
	}

	type siteMeta struct {
		Role string `json:"role"`
		Tier int    `json:"tier"`
	}
	if got, err := As[siteMeta](snap, "site_meta"); err != nil || got.Role != "web" || got.Tier != 2 {
		t.Fatalf("As[siteMeta](site_meta) = %#v, %v, want decoded registered fact", got, err)
	}

	type siteOwner struct {
		Team   string `json:"team"`
		Oncall bool   `json:"oncall"`
	}
	if got, err := As[siteOwner](snap, "site_owner"); err != nil || got.Team != "platform" || !got.Oncall {
		t.Fatalf("As[siteOwner](site_owner) = %#v, %v, want decoded external fact", got, err)
	}
}

func TestAs_shapeMismatchFailsLoudly(t *testing.T) {
	externalDir := t.TempDir()
	writeTestFile(t, externalDir, "os.txt", "os=Ubuntu-ish\n")

	eng, err := New(WithExternalDirs(externalDir))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := snap.Value("os"); got != "Ubuntu-ish" {
		t.Fatalf("Value(os) = %#v, want external fact reshaping os", got)
	}

	type osFact struct {
		Name string `json:"name"`
	}
	got, err := As[osFact](snap, "os")
	if err == nil {
		t.Fatal("As[osFact](os) err = nil, want loud shape mismatch")
	}
	if got != (osFact{}) {
		t.Fatalf("As[osFact](os) = %#v, want zero value on mismatch", got)
	}
}

func TestAs_rejectsMapAnyKeyStringCollisions(t *testing.T) {
	eng, err := New(WithFact("ambiguous", func(context.Context) (any, error) {
		return map[any]any{"1": "string-key", 1: "int-key"}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	_, err = As[map[string]string](snap, "ambiguous")
	if err == nil || !strings.Contains(err.Error(), `duplicate map key after string normalization: "1"`) {
		t.Fatalf("As ambiguous err = %v, want duplicate normalized key error", err)
	}
}

func TestAs_normalizesNestedMapAnyValues(t *testing.T) {
	eng, err := New(WithFact("nested", func(context.Context) (any, error) {
		return map[any]any{
			"items": []any{
				map[any]any{"name": "web", "port": 443},
			},
		}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	type item struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}
	type nestedFact struct {
		Items []item `json:"items"`
	}
	got, err := As[nestedFact](snap, "nested")
	if err != nil {
		t.Fatal(err)
	}
	want := nestedFact{Items: []item{{Name: "web", Port: 443}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("As[nestedFact](nested) = %#v, want %#v", got, want)
	}
}

func TestAs_missingFactReturnsErrFactNotFound(t *testing.T) {
	snap := hermeticSnapshot()

	if _, err := As[string](snap, "missing_fact"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("As[string](missing_fact) err = %v, want ErrFactNotFound", err)
	}
}

func TestSnapshotTree_isACopy(t *testing.T) {
	snap := hermeticSnapshot()

	tree := snap.Tree()
	osFact, ok := tree["os"].(map[string]any)
	if !ok {
		t.Fatalf("Tree()[os] = %#v, want map", tree["os"])
	}
	original, err := snap.Value("os.name")
	if err != nil {
		t.Fatal(err)
	}
	osFact["name"] = "mutated"

	if got, _ := snap.Value("os.name"); got != original {
		t.Fatalf("Value(os.name) = %#v after tree mutation, want %#v", got, original)
	}
}

func TestSnapshotAll_iteratesSortedTopLevelEntries(t *testing.T) {
	snap := hermeticSnapshot()

	var names []string
	for name, value := range snap.All() {
		names = append(names, name)
		if value == nil {
			t.Fatalf("All() yielded %s = nil, want canonical-tree value", name)
		}
	}
	if len(names) == 0 {
		t.Fatal("All() yielded nothing, want canonical tree entries")
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("All() order: %q before %q, want sorted names", names[i-1], names[i])
		}
	}
}

func TestDiscover_silentByDefaultSkipsRubyFileInExternalDir(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "weird.rb", "Facter.add(:weird_fact) do\n  setcode do\n    Complicated::Ruby.compute(42)\n  end\nend\n")

	eng, err := New(WithExternalDirs(dir))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want Ruby fact file skipped without error", err)
	}
	if _, err := snap.Value("weird_fact"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(weird_fact) err = %v, want Ruby fact file unread", err)
	}
}

// recordingHandler collects slog records verbatim, so assertions compare raw
// contract message text without any handler-format escaping (Windows paths
// contain backslashes that text handlers escape).
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, record)
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordingHandler) countByPrefix(prefix string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for _, record := range h.records {
		if strings.HasPrefix(record.Message, prefix) {
			count++
		}
	}
	return count
}

func (h *recordingHandler) hasWarn(message string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, record := range h.records {
		if record.Level == slog.LevelWarn && record.Message == message {
			return true
		}
	}
	return false
}

func TestWithLogger_routesDiagnosticsOncePerEngine(t *testing.T) {
	t.Setenv("FACTER_EXTERNAL_FACTS_RUNNING", "1")
	dir := t.TempDir()
	writeTestFile(t, dir, "weird.rb", "Facter.add(:weird_fact) do\n  setcode do\n    Complicated::Ruby.compute(42)\n  end\nend\n")
	handler := &recordingHandler{}
	logger := slog.New(handler)

	eng, err := New(WithExternalDirs(dir), WithLogger(logger))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := eng.Discover(context.Background()); err != nil {
			t.Fatalf("Discover() err = %v, want nil", err)
		}
	}
	if got := handler.countByPrefix("Recursion detected"); got != 1 {
		t.Fatalf("logger got %d once-only warnings across discoveries, want exactly 1", got)
	}

	want := fmt.Sprintf("Ruby fact files are not supported by the Go port; skipping %s. Rewrite it as an executable external fact (see docs/CUSTOM_FACT_MIGRATION.md).", filepath.Join(dir, "weird.rb"))
	if !handler.hasWarn(want) {
		t.Fatalf("logger records = %#v, want WARN with contract message %q", handler.records, want)
	}
}
