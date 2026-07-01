package engine

import (
	"os"
	"reflect"
	"testing"
)

// pkgngQueryFixtures are real `pkg query -a '%n|%v|%q'` outputs captured from the
// nlab FreeBSD and DragonFly guests. They exercise PORTEPOCH commas in versions
// (flac|1.4.3,1), ABIs with internal colons (dragonfly:6.4:x86:64), and the
// architecture-independent "*" ABI wildcard.
func TestPkgngPackages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		out  string
		want []any
	}{
		{
			name: "dragonfly",
			out: "bash|5.2.15|dragonfly:6.4:x86:64\n" +
				"ca_root_nss|3.89.1|dragonfly:6.4:*\n" +
				"flac|1.4.3,1|dragonfly:6.4:x86:64\n" +
				"libedit|3.1.20221030,1|dragonfly:6.4:x86:64\n",
			want: []any{
				map[string]any{"name": "bash", "version": "5.2.15", "architecture": "dragonfly:6.4:x86:64"},
				map[string]any{"name": "ca_root_nss", "version": "3.89.1", "architecture": "dragonfly:6.4:*"},
				map[string]any{"name": "flac", "version": "1.4.3,1", "architecture": "dragonfly:6.4:x86:64"},
				map[string]any{"name": "libedit", "version": "3.1.20221030,1", "architecture": "dragonfly:6.4:x86:64"},
			},
		},
		{
			name: "freebsd",
			out: "firstboot-freebsd-update|1.4|FreeBSD:14:*\n" +
				"pkg|2.1.2|FreeBSD:14:amd64\n",
			want: []any{
				map[string]any{"name": "firstboot-freebsd-update", "version": "1.4", "architecture": "FreeBSD:14:*"},
				map[string]any{"name": "pkg", "version": "2.1.2", "architecture": "FreeBSD:14:amd64"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pkgngPackages(func(name string, args ...string) string {
				if name != "/usr/local/sbin/pkg" || len(args) == 0 || args[0] != "query" {
					t.Fatalf("command = %q %v", name, args)
				}
				return tt.out
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("pkgngPackages() = %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestPkgngPackagesEmptyYieldsNothing(t *testing.T) {
	t.Parallel()
	if got := pkgngPackages(func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("pkgngPackages(empty) = %#v, want nil", got)
	}
}

// openbsdContentsFixture is a trimmed real bash-5.2.15 +CONTENTS; the @arch
// annotation carries the architecture while the @comment line (which also
// contains the substring "arch") must not be mistaken for it.
const openbsdContentsFixture = `@name bash-5.2.15
@option manual-installation
@comment pkgpath=shells/bash ftp=yes
@arch amd64
+DESC
`

// TestOpenbsdPackages enumerates real /var/db/pkg entry names (incl. the
// vim-9.0.2035-no_x11 flavor case and the gettext-runtime dashed stem), skips a
// non-directory index/lock entry, and reads architecture from +CONTENTS,
// omitting it for the architecture-independent "*" package.
func TestOpenbsdPackages(t *testing.T) {
	t.Parallel()
	entries := []os.DirEntry{
		fakeDirEntry{name: "bash-5.2.15", mode: os.ModeDir, isDir: true},
		fakeDirEntry{name: "gettext-runtime-0.22.2", mode: os.ModeDir, isDir: true},
		fakeDirEntry{name: "vim-9.0.2035-no_x11", mode: os.ModeDir, isDir: true},
		fakeDirEntry{name: "quirks-6.160", mode: os.ModeDir, isDir: true},
		fakeDirEntry{name: ".lock", isDir: false},
	}
	contents := map[string]string{
		"/var/db/pkg/bash-5.2.15/+CONTENTS":            openbsdContentsFixture,
		"/var/db/pkg/gettext-runtime-0.22.2/+CONTENTS": "@name gettext-runtime-0.22.2\n@arch amd64\n",
		"/var/db/pkg/vim-9.0.2035-no_x11/+CONTENTS":    "@name vim-9.0.2035-no_x11\n@option manual-installation\n@arch amd64\n",
		"/var/db/pkg/quirks-6.160/+CONTENTS":           "@name quirks-6.160\n@arch *\n",
	}
	got := openbsdPackages(
		func(path string) ([]os.DirEntry, error) {
			if path != "/var/db/pkg" {
				t.Fatalf("readDir path = %q", path)
			}
			return entries, nil
		},
		func(path string) ([]byte, error) {
			data, ok := contents[path]
			if !ok {
				t.Fatalf("readFile path = %q", path)
			}
			return []byte(data), nil
		},
	)
	want := []any{
		map[string]any{"name": "bash", "version": "5.2.15", "architecture": "amd64"},
		map[string]any{"name": "gettext-runtime", "version": "0.22.2", "architecture": "amd64"},
		map[string]any{"name": "quirks", "version": "6.160"},
		map[string]any{"name": "vim", "version": "9.0.2035", "architecture": "amd64"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("openbsdPackages() = %#v\nwant %#v", got, want)
	}
}

func TestOpenbsdPackagesAbsentDatabaseYieldsNothing(t *testing.T) {
	t.Parallel()
	got := openbsdPackages(
		func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
	)
	if got != nil {
		t.Fatalf("openbsdPackages(absent) = %#v, want nil", got)
	}
}

// TestPkgsrcPackages discovers PKG_DBDIR by probing the standard candidates in
// order. Entry names are real NetBSD /usr/pkg/pkgdb names, including the
// pkg_install underscore stem, the vim-share dashed stem, and the
// pkgdb.byfile.db index (a plain file that must be skipped).
func TestPkgsrcPackages(t *testing.T) {
	t.Parallel()
	entries := []os.DirEntry{
		fakeDirEntry{name: "bash-5.2.21nb1", mode: os.ModeDir, isDir: true},
		fakeDirEntry{name: "ca-certificates-20230311nb3", mode: os.ModeDir, isDir: true},
		fakeDirEntry{name: "pkg_install-20211115nb1", mode: os.ModeDir, isDir: true},
		fakeDirEntry{name: "vim-share-9.0.2122", mode: os.ModeDir, isDir: true},
		fakeDirEntry{name: "pkgdb.byfile.db", isDir: false},
	}
	want := []any{
		map[string]any{"name": "bash", "version": "5.2.21nb1"},
		map[string]any{"name": "ca-certificates", "version": "20230311nb3"},
		map[string]any{"name": "pkg_install", "version": "20211115nb1"},
		map[string]any{"name": "vim-share", "version": "9.0.2122"},
	}
	tests := []struct {
		name  string
		dbdir string
	}{
		{name: "default pkgdb", dbdir: "/usr/pkg/pkgdb"},
		{name: "legacy fallback", dbdir: "/var/db/pkg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pkgsrcPackages(func(path string) ([]os.DirEntry, error) {
				if path == tt.dbdir {
					return entries, nil
				}
				return nil, os.ErrNotExist
			})
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pkgsrcPackages() = %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestPkgsrcPackagesAbsentDatabaseYieldsNothing(t *testing.T) {
	t.Parallel()
	got := pkgsrcPackages(func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist })
	if got != nil {
		t.Fatalf("pkgsrcPackages(absent) = %#v, want nil", got)
	}
}

// ipsListFixture is real `pkg list -H` output from the nlab OmniOS guest (3
// columns: NAME VERSION IFO). The image installs only from the preferred
// publisher, so no package renders a "(publisher)" column; the system/library
// line is a format-accurate synthetic entry from the extra.omnios publisher to
// exercise the parenthesised-publisher branch.
const ipsListFixture = `SUNWcs                                            0.5.11-151058.0            i--
compress/xz                                       5.8.3-151058.0             i--
database/sqlite-3                                 3.51.3-151058.0            i--
developer/macro/cpp                               20240422-151058.0          i--
system/library (extra.omnios)                     1.2.3-151046.0             i--
`

func TestIpsPackages(t *testing.T) {
	t.Parallel()
	got := ipsPackages(func(name string, args ...string) string {
		if name != "pkg" || len(args) < 2 || args[0] != "list" || args[1] != "-H" {
			t.Fatalf("command = %q %v", name, args)
		}
		return ipsListFixture
	})
	want := []any{
		map[string]any{"name": "SUNWcs", "version": "0.5.11-151058.0"},
		map[string]any{"name": "compress/xz", "version": "5.8.3-151058.0"},
		map[string]any{"name": "database/sqlite-3", "version": "3.51.3-151058.0"},
		map[string]any{"name": "developer/macro/cpp", "version": "20240422-151058.0"},
		map[string]any{"name": "system/library", "version": "1.2.3-151046.0"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ipsPackages() = %#v\nwant %#v", got, want)
	}
}

func TestIpsPackagesEmptyYieldsNothing(t *testing.T) {
	t.Parallel()
	if got := ipsPackages(func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("ipsPackages(empty) = %#v, want nil", got)
	}
}

// TestSplitBsdPackageName covers the OpenBSD/pkgsrc <name>-<version>[-flavor]
// convention: the version is the last hyphen component (after a non-empty stem)
// starting with a digit, keeping dashed and numeric-suffixed stems intact.
func TestSplitBsdPackageName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dir         string
		wantName    string
		wantVersion string
	}{
		{"autoconf-2.69p3", "autoconf", "2.69p3"},
		{"quirks-7.55", "quirks", "7.55"},
		{"xz-5.4.5", "xz", "5.4.5"},
		{"gettext-runtime-0.22.2", "gettext-runtime", "0.22.2"},
		{"vim-9.0.2035-no_x11", "vim", "9.0.2035"},
		{"pcre2-10.37p1", "pcre2", "10.37p1"},
		{"pkg_install-20211115nb1", "pkg_install", "20211115nb1"},
		{"gcc-11-11.2.0", "gcc-11", "11.2.0"},
		{"pkgdb.byfile.db", "", ""},
		{"quirks", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			t.Parallel()
			name, version := splitBSDPackageName(tt.dir)
			if name != tt.wantName || version != tt.wantVersion {
				t.Fatalf("splitBSDPackageName(%q) = (%q, %q), want (%q, %q)", tt.dir, name, version, tt.wantName, tt.wantVersion)
			}
		})
	}
}
