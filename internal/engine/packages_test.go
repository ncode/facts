package engine

import (
	"os"
	"reflect"
	"testing"
)

// dpkgStatusFixture is a trimmed /var/lib/dpkg/status: two installed packages
// (one multiarch sibling pair) plus a removed package that must be filtered.
const dpkgStatusFixture = `Package: adduser
Status: install ok installed
Priority: important
Architecture: all
Version: 3.134
Description: add and remove users

Package: libc6
Status: install ok installed
Architecture: amd64
Version: 2.36-9+deb12u7

Package: libc6
Status: install ok installed
Architecture: i386
Version: 2.36-9+deb12u7

Package: nginx
Status: hold ok installed
Architecture: amd64
Version: 1.22.1-9

Package: removed-pkg
Status: deinstall ok config-files
Architecture: amd64
Version: 1.0
`

func TestDpkgPackages_installedOnlyWithMultiarchSiblings(t *testing.T) {
	t.Parallel()
	got := dpkgPackages(func(path string) ([]byte, error) {
		if path != "/var/lib/dpkg/status" {
			t.Fatalf("readFile path = %q", path)
		}
		return []byte(dpkgStatusFixture), nil
	})
	want := []any{
		map[string]any{"name": "adduser", "version": "3.134", "architecture": "all"},
		map[string]any{"name": "libc6", "version": "2.36-9+deb12u7", "architecture": "amd64"},
		map[string]any{"name": "libc6", "version": "2.36-9+deb12u7", "architecture": "i386"},
		map[string]any{"name": "nginx", "version": "1.22.1-9", "architecture": "amd64"}, // held package kept
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dpkgPackages() = %#v\nwant %#v", got, want)
	}
}

// rpmQueryFixture is the output of the epoch-bearing rpm -qa query. gpg-pubkey
// must be filtered; the (none) epoch must be stripped; a real epoch is kept.
const rpmQueryFixture = `bash|(none):5.1.8-9.el9|x86_64
glibc|(none):2.34-266.el9_8|x86_64
kernel-core|(none):5.14.0-687.5.3.el9_8|x86_64
grub2-common|1:2.06-104.el9_8|noarch
gpg-pubkey|(none):fd431d51-4ae0493b|(none)
`

func TestRpmPackages_stripsNoneEpochKeepsRealEpochFiltersPubkey(t *testing.T) {
	t.Parallel()
	got := rpmPackages(func(name string, args ...string) string {
		if name != "rpm" {
			t.Fatalf("command = %q %v", name, args)
		}
		return rpmQueryFixture
	})
	want := []any{
		map[string]any{"name": "bash", "version": "5.1.8-9.el9", "architecture": "x86_64"},
		map[string]any{"name": "glibc", "version": "2.34-266.el9_8", "architecture": "x86_64"},
		map[string]any{"name": "grub2-common", "version": "1:2.06-104.el9_8", "architecture": "noarch"},
		map[string]any{"name": "kernel-core", "version": "5.14.0-687.5.3.el9_8", "architecture": "x86_64"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rpmPackages() = %#v\nwant %#v", got, want)
	}
}

// apkInstalledFixture is two stanzas from /lib/apk/db/installed.
const apkInstalledFixture = `C:Q1FVD1ypez9RDWd52MokUzPbtTaj8=
P:alpine-base
V:3.24.1-r0
A:x86_64
S:1322
T:Meta package for minimal alpine base

C:Q18Pz9wvTaBYr2RzvEXE6rYpFiJbI=
P:musl
V:1.2.5-r9
A:x86_64
`

func TestApkPackages_parsesStanzas(t *testing.T) {
	t.Parallel()
	got := apkPackages(func(path string) ([]byte, error) {
		if path != "/lib/apk/db/installed" {
			t.Fatalf("readFile path = %q", path)
		}
		return []byte(apkInstalledFixture), nil
	})
	want := []any{
		map[string]any{"name": "alpine-base", "version": "3.24.1-r0", "architecture": "x86_64"},
		map[string]any{"name": "musl", "version": "1.2.5-r9", "architecture": "x86_64"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("apkPackages() = %#v\nwant %#v", got, want)
	}
}

// pacmanDescFixture is one /var/lib/pacman/local/<pkg>/desc.
const pacmanDescFixture = `%NAME%
acl

%VERSION%
2.3.2-2

%BASE%
acl

%ARCH%
x86_64
`

func TestPacmanPackages_parsesDescFiles(t *testing.T) {
	t.Parallel()
	got := pacmanPackages(
		func(pattern string) ([]string, error) {
			if pattern != "/var/lib/pacman/local/*/desc" {
				t.Fatalf("glob pattern = %q", pattern)
			}
			return []string{"/var/lib/pacman/local/zlib-1.3.1-2/desc", "/var/lib/pacman/local/acl-2.3.2-2/desc"}, nil
		},
		func(path string) ([]byte, error) {
			switch path {
			case "/var/lib/pacman/local/acl-2.3.2-2/desc":
				return []byte(pacmanDescFixture), nil
			case "/var/lib/pacman/local/zlib-1.3.1-2/desc":
				return []byte("%NAME%\nzlib\n\n%VERSION%\n1.3.1-2\n\n%ARCH%\nx86_64\n"), nil
			default:
				t.Fatalf("readFile path = %q", path)
				return nil, nil
			}
		},
	)
	want := []any{
		map[string]any{"name": "acl", "version": "2.3.2-2", "architecture": "x86_64"},
		map[string]any{"name": "zlib", "version": "1.3.1-2", "architecture": "x86_64"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pacmanPackages() = %#v\nwant %#v", got, want)
	}
}

func TestDpkgPackages_absentDatabaseYieldsNothing(t *testing.T) {
	t.Parallel()
	if got := dpkgPackages(func(string) ([]byte, error) { return nil, os.ErrNotExist }); got != nil {
		t.Fatalf("dpkgPackages(absent) = %#v, want nil", got)
	}
}
