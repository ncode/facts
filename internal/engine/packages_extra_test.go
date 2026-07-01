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

func TestNixPackages_emptyProfileYieldsNothing(t *testing.T) {
	t.Parallel()
	if got := nixPackages(func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("nixPackages(empty) = %#v, want nil", got)
	}
}

// snapListFixture is the canonical `snap list` layout (Name Version Rev Tracking
// Publisher Notes). Not guest-validated: no fleet guest has snaps installed.
const snapListFixture = `Name         Version    Rev    Tracking       Publisher   Notes
bare         1.0        5      latest/stable  canonical✓  base
core22       20240111   1122   latest/stable  canonical✓  base
hello-world  6.4        29     latest/stable  canonical✓  -
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
		map[string]any{"name": "bare", "version": "1.0"},
		map[string]any{"name": "core22", "version": "20240111"},
		map[string]any{"name": "hello-world", "version": "6.4"},
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

// flatpakListFixture is tab-separated `flatpak list --columns=application,version,arch`
// output. Not guest-validated: no fleet guest has flatpak installed.
const flatpakListFixture = "org.gnome.Platform\t46\tx86_64\n" +
	"org.mozilla.firefox\t124.0\tx86_64\n" +
	"com.spotify.Client\t1.2.31.1205\tx86_64\n"

func TestFlatpakPackages_parsesTabColumns(t *testing.T) {
	t.Parallel()
	got := flatpakPackages(func(name string, args ...string) string {
		if name != "flatpak" || !reflect.DeepEqual(args, []string{"list", "--columns=application,version,arch"}) {
			t.Fatalf("command = %q %v", name, args)
		}
		return flatpakListFixture
	})
	want := []any{
		map[string]any{"name": "com.spotify.Client", "version": "1.2.31.1205", "architecture": "x86_64"},
		map[string]any{"name": "org.gnome.Platform", "version": "46", "architecture": "x86_64"},
		map[string]any{"name": "org.mozilla.firefox", "version": "124.0", "architecture": "x86_64"},
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
		{"nix", nixProfilePresent, "/nix/var/nix/profiles/system"},
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
