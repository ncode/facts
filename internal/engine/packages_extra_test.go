package engine

import (
	"os"
	"reflect"
	"testing"
)

// nixReferencesFixture is verbatim `nix-store -q --references
// /run/current-system/sw` output captured from a NixOS 26.05 guest. It exercises
// the real shapes: plain name-version, split outputs (-man/-info/-doc/-bin) that
// must dedup onto their base version, glibc's multi-component "2.42-67" version
// that must stay intact, and unversioned environment members that must be dropped.
const nixReferencesFixture = `/nix/store/mxq1r9w2w2y9lsqb5fkcyb5xbbki1n57-ncurses-6.6
/nix/store/6cblb0wy8kknk5jwj2gzah0jz6ihj3kx-ncurses-6.6-man
/nix/store/0641h8qfqaxnwrsw2nzrz6i1wbzyx92l-bash-interactive-5.3p9
/nix/store/pspjmjsphdkjsi20gpxh2p1aq6p73n1c-bash-interactive-5.3p9-info
/nix/store/0cx4sx34abcvd49mwzjyxq4j5sxlbbmp-linux-pam-1.7.1-doc
/nix/store/pv8aczqkk94gzxhx0gk168gpxcll0svi-linux-pam-1.7.1
/nix/store/sr26flm2nkfa12dkrwj2630kqsfakky4-coreutils-9.11
/nix/store/1v1hd7hnss1y3d40dkib5x0dqppkadp8-sudo-1.9.17p2
/nix/store/0sbi54dacjmi5grcgwyxz5zhc1pd26bp-sudo-1.9.17p2-doc
/nix/store/gxxyccld1qfy1v98hbzv8g9yk1saqx82-zstd-1.5.7-bin
/nix/store/1b2phk81syp3chqny7m713295rg519v6-zstd-1.5.7-man
/nix/store/521dd0054ifhzmjfmpx9mz0hr7wh7sig-glibc-2.42-67-bin
/nix/store/cdqbzw7gmcd62s8pyif19psr47l4000q-getent-glibc-2.42-67
/nix/store/292rq2lcrzax7lcfhr2qwwxcqidv0df0-nixos-help
/nix/store/16c4zsnxzvm782n0ddhj6cabl23ar2vw-nixos-configuration-reference-manpage
`

func TestNixPackages_dedupOutputsKeepsMultiComponentVersion(t *testing.T) {
	t.Parallel()
	got := nixPackages(func(name string, args ...string) string {
		if name != "/run/current-system/sw/bin/nix-store" || !reflect.DeepEqual(args, []string{"-q", "--references", "/run/current-system/sw"}) {
			t.Fatalf("command = %q %v", name, args)
		}
		return nixReferencesFixture
	})
	want := []any{
		map[string]any{"name": "bash-interactive", "version": "5.3p9"},
		map[string]any{"name": "coreutils", "version": "9.11"},
		map[string]any{"name": "getent-glibc", "version": "2.42-67"},
		map[string]any{"name": "glibc", "version": "2.42-67"},
		map[string]any{"name": "linux-pam", "version": "1.7.1"},
		map[string]any{"name": "ncurses", "version": "6.6"},
		map[string]any{"name": "sudo", "version": "1.9.17p2"},
		map[string]any{"name": "zstd", "version": "1.5.7"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nixPackages() = %#v\nwant %#v", got, want)
	}
}

func TestParseNixNameVersion_digitLeadingNames(t *testing.T) {
	t.Parallel()
	// A package whose name starts with a digit (7zip, 0ad, 389-ds-base) must not
	// be mistaken for a version and dropped: the name is always at least the
	// first hyphen component, so the version boundary is >= 1.
	cases := []struct{ tail, name, version string }{
		{"7zip-24.09", "7zip", "24.09"},
		{"389-ds-base-2.6.0", "389-ds-base", "2.6.0"},
		{"0ad-0.0.26", "0ad", "0.0.26"},
		{"glibc-2.42-67-bin", "glibc", "2.42-67"}, // multi-component version, output stripped
		{"nixos-help", "", ""},                    // unversioned member dropped
	}
	for _, c := range cases {
		if n, v := parseNixNameVersion(c.tail); n != c.name || v != c.version {
			t.Errorf("parseNixNameVersion(%q) = (%q, %q), want (%q, %q)", c.tail, n, v, c.name, c.version)
		}
	}
}

// nixEnvDefaultProfileFixture is verbatim `nix-env -q --profile
// /nix/var/nix/profiles/default` output from a daemon (multi-user) nix install
// on the nlab ubuntu2404 guest — the non-NixOS shape (ADR-0014: the installed
// profile set is the default profile AND the NixOS system profile).
const nixEnvDefaultProfileFixture = `nix-2.34.7
nix-manual-2.34.7-man
nss-cacert-3.117
`

func TestNixPackages_fallsBackToDefaultProfileOnNonNixOS(t *testing.T) {
	t.Parallel()
	var calls []string
	got := nixPackages(func(name string, args ...string) string {
		calls = append(calls, name)
		switch name {
		case "/run/current-system/sw/bin/nix-store":
			return "" // not NixOS: no system profile environment
		case "/nix/var/nix/profiles/default/bin/nix-env":
			if !reflect.DeepEqual(args, []string{"-q", "--profile", "/nix/var/nix/profiles/default"}) {
				t.Fatalf("nix-env args = %v", args)
			}
			return nixEnvDefaultProfileFixture
		default:
			t.Fatalf("unexpected command %q", name)
			return ""
		}
	})
	want := []any{
		map[string]any{"name": "nix", "version": "2.34.7"},
		map[string]any{"name": "nix-manual", "version": "2.34.7"}, // -man output suffix stripped
		map[string]any{"name": "nss-cacert", "version": "3.117"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nixPackages(default profile) = %#v\nwant %#v", got, want)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v, want nix-store then nix-env fallback", calls)
	}
}

func TestNixPackages_systemProfileWinsWithoutFallbackSpawn(t *testing.T) {
	t.Parallel()
	var calls []string
	got := nixPackages(func(name string, args ...string) string {
		calls = append(calls, name)
		return "/nix/store/mxq1r9w2w2y9lsqb5fkcyb5xbbki1n57-ncurses-6.6\n"
	})
	if want := []any{map[string]any{"name": "ncurses", "version": "6.6"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nixPackages(system) = %#v, want %#v", got, want)
	}
	// NixOS: the system profile is canonical; the default-profile fallback must
	// not run (it could double-report nix itself).
	if want := []string{"/run/current-system/sw/bin/nix-store"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestNixPackages_emptyProfileYieldsNothing(t *testing.T) {
	t.Parallel()
	if got := nixPackages(func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("nixPackages(empty) = %#v, want nil", got)
	}
}

// snapListFixture is verbatim `snap list` output from the nlab ubuntu2404
// guest (snapd 2.75.2) after installing hello-world.
const snapListFixture = `Name         Version             Rev    Tracking       Publisher    Notes
core         16-2.61.4-20260225  17292  latest/stable  canonical**  core
hello-world  6.4                 29     latest/stable  canonical**  -
snapd        2.75.2              26865  latest/stable  canonical**  snapd
`

func TestSnapPackages_skipsHeader(t *testing.T) {
	t.Parallel()
	got := snapPackages(func(name string, args ...string) string {
		if name != "snap" || !reflect.DeepEqual(args, []string{"list"}) {
			t.Fatalf("command = %q %v", name, args)
		}
		return snapListFixture
	})
	want := []any{
		map[string]any{"name": "core", "version": "16-2.61.4-20260225"},
		map[string]any{"name": "hello-world", "version": "6.4"},
		map[string]any{"name": "snapd", "version": "2.75.2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapPackages() = %#v\nwant %#v", got, want)
	}
}

func TestSnapPackages_noSnapsInstalledYieldsNothing(t *testing.T) {
	t.Parallel()
	got := snapPackages(func(string, ...string) string {
		return "No snaps are installed yet. Try 'snap install hello-world'.\n"
	})
	if got != nil {
		t.Fatalf("snapPackages(no snaps) = %#v, want nil", got)
	}
}

// flatpakListFixture is verbatim tab-separated output of
// `flatpak list --columns=application,version,arch,branch` from the nlab
// ubuntu2404 guest after installing org.vim.Vim from flathub. It exercises the
// two real shapes the format-only fixture missed: the SAME application id
// installed twice with identical version+arch, distinguishable only by branch
// (GL.default 25.08 vs 25.08-extra), and an extension with an empty version
// (codecs-extra), which is dropped by the name+version invariant.
const flatpakListFixture = "org.freedesktop.Platform.GL.default\t26.0.8\tx86_64\t25.08\n" +
	"org.freedesktop.Platform.GL.default\t26.0.8\tx86_64\t25.08-extra\n" +
	"org.freedesktop.Platform.codecs-extra\t\tx86_64\t25.08-extra\n" +
	"org.freedesktop.Sdk\tfreedesktop-sdk-25.08.13\tx86_64\t25.08\n" +
	"org.vim.Vim\tv9.2.0758\tx86_64\tstable\n"

func TestFlatpakPackages_branchDistinguishesSiblings(t *testing.T) {
	t.Parallel()
	got := flatpakPackages(func(name string, args ...string) string {
		if name != "flatpak" || !reflect.DeepEqual(args, []string{"list", "--columns=application,version,arch,branch"}) {
			t.Fatalf("command = %q %v", name, args)
		}
		return flatpakListFixture
	})
	want := []any{
		map[string]any{"name": "org.freedesktop.Platform.GL.default", "version": "26.0.8", "architecture": "x86_64", "branch": "25.08"},
		map[string]any{"name": "org.freedesktop.Platform.GL.default", "version": "26.0.8", "architecture": "x86_64", "branch": "25.08-extra"},
		map[string]any{"name": "org.freedesktop.Sdk", "version": "freedesktop-sdk-25.08.13", "architecture": "x86_64", "branch": "25.08"},
		map[string]any{"name": "org.vim.Vim", "version": "v9.2.0758", "architecture": "x86_64", "branch": "stable"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flatpakPackages() = %#v\nwant %#v", got, want)
	}
}

func TestFlatpakPackages_emptyYieldsNothing(t *testing.T) {
	t.Parallel()
	if got := flatpakPackages(func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("flatpakPackages(empty) = %#v, want nil", got)
	}
}

func TestExtraPackageSourcePresenceGates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		gate func(func(string) (os.FileInfo, error)) bool
		dir  string
	}{
		{"snap", snapdPresent, "/var/lib/snapd"},
		{"flatpak", flatpakPresent, "/var/lib/flatpak"},
	}
	for _, tc := range cases {
		present := func(path string) (os.FileInfo, error) {
			if path != tc.dir {
				t.Fatalf("%s gate stat path = %q, want %q", tc.name, path, tc.dir)
			}
			return fakeFileInfo{name: tc.dir, mode: os.ModeDir, isDir: true}, nil
		}
		if !tc.gate(present) {
			t.Fatalf("%s gate = false with %s present, want true", tc.name, tc.dir)
		}
		absent := func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		if tc.gate(absent) {
			t.Fatalf("%s gate = true with %s absent, want false", tc.name, tc.dir)
		}
		notDir := func(string) (os.FileInfo, error) {
			return fakeFileInfo{name: tc.dir}, nil
		}
		if tc.gate(notDir) {
			t.Fatalf("%s gate = true with %s a non-directory, want false", tc.name, tc.dir)
		}
	}
}

func TestNixProfilePresent_eitherSystemOrDefault(t *testing.T) {
	t.Parallel()
	// ADR-0014: the nix profile set is the NixOS system profile AND the
	// default profile — the gate must open when either exists.
	only := func(dir string) func(string) (os.FileInfo, error) {
		return func(path string) (os.FileInfo, error) {
			if path == dir {
				return fakeFileInfo{name: dir, mode: os.ModeDir, isDir: true}, nil
			}
			return nil, os.ErrNotExist
		}
	}
	if !nixProfilePresent(only("/nix/var/nix/profiles/system")) {
		t.Fatal("nixProfilePresent = false with only the NixOS system profile")
	}
	if !nixProfilePresent(only("/nix/var/nix/profiles/default")) {
		t.Fatal("nixProfilePresent = false with only the default profile")
	}
	if nixProfilePresent(func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }) {
		t.Fatal("nixProfilePresent = true with neither profile")
	}
}
