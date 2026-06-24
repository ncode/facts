package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSELinuxFactsReadsConfigAndMountpoint(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mountpoint := filepath.Join(dir, "selinux")
	if err := os.Mkdir(mountpoint, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "mounts"), "none "+mountpoint+" selinuxfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enforcing\nSELINUXTYPE=targeted\n")
	writeFile(t, filepath.Join(mountpoint, "enforce"), "1")
	writeFile(t, filepath.Join(mountpoint, "policyvers"), "33")

	core := selinuxFacts(filepath.Join(dir, "mounts"), filepath.Join(dir, "config"), os.ReadFile)
	collection := Collection(core)
	if got, want := collection["os"].(map[string]any)["selinux"], map[string]any{
		"config_mode":    "enforcing",
		"config_policy":  "targeted",
		"current_mode":   "enforcing",
		"enabled":        true,
		"enforced":       true,
		"policy_version": "33",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selinuxFacts() core = %#v, want %#v", got, want)
	}
}

func TestSELinuxFactsDisabledWithoutMountpointOrConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mounts"), "rootfs / rootfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enforcing\n")

	core := selinuxFacts(filepath.Join(dir, "mounts"), filepath.Join(dir, "config"), os.ReadFile)
	if got := Collection(core)["os"].(map[string]any)["selinux"].(map[string]any)["enabled"]; got != false {
		t.Fatalf("os.selinux.enabled = %#v, want false", got)
	}

	writeFile(t, filepath.Join(dir, "mounts"), "none /sys/fs/selinux selinuxfs rw 0 0\n")
	core = selinuxFacts(filepath.Join(dir, "mounts"), filepath.Join(dir, "missing-config"), os.ReadFile)
	selinux := Collection(core)["os"].(map[string]any)["selinux"].(map[string]any)
	if got := selinux["enabled"]; got != true {
		t.Fatalf("os.selinux.enabled = %#v, want true with selinuxfs even without config", got)
	}
	if _, ok := selinux["config_mode"]; ok {
		t.Fatalf("os.selinux.config_mode = %#v, want omitted without config", selinux["config_mode"])
	}
}

func TestSELinuxFactsForPlatform_omittedOutsideLinux(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mounts"), "none /sys/fs/selinux selinuxfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enforcing\nSELINUXTYPE=targeted\n")

	for _, goos := range []string{"darwin", "freebsd", "openbsd", "netbsd", "windows"} {
		if got := selinuxFactsForPlatform(goos, filepath.Join(dir, "mounts"), filepath.Join(dir, "config"), os.ReadFile); got != nil {
			t.Fatalf("selinuxFactsForPlatform(%s) = %#v, want nil", goos, got)
		}
	}
}

func TestSELinuxFactsForPlatform_resolvesOnLinux(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mounts"), "rootfs / rootfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enforcing\n")

	core := selinuxFactsForPlatform("linux", filepath.Join(dir, "mounts"), filepath.Join(dir, "config"), os.ReadFile)
	if got := Collection(core)["os"].(map[string]any)["selinux"].(map[string]any)["enabled"]; got != false {
		t.Fatalf("os.selinux.enabled = %#v, want false on Linux without selinuxfs", got)
	}
}

func TestSELinuxFactsKeepsMissingPolicyVersionNil(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mountpoint := filepath.Join(dir, "selinux")
	writeFile(t, filepath.Join(dir, "mounts"), "none "+mountpoint+" selinuxfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enabled\nSELINUXTYPE=targeted\n")
	writeFile(t, filepath.Join(mountpoint, "enforce"), "")

	core := selinuxFacts(filepath.Join(dir, "mounts"), filepath.Join(dir, "config"), os.ReadFile)

	if got := Collection(core)["os"].(map[string]any)["selinux"].(map[string]any)["policy_version"]; got != nil {
		t.Fatalf("os.selinux.policy_version = %#v, want nil", got)
	}
}

func TestSELinuxFactsOmitsEnforcementWhenEnforceIsUnreadableOrInvalid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mountpoint := filepath.Join(dir, "selinux")
	if err := os.Mkdir(mountpoint, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "mounts"), "none "+mountpoint+" selinuxfs rw 0 0\n")
	writeFile(t, filepath.Join(dir, "config"), "SELINUX=enforcing\nSELINUXTYPE=targeted\n")
	writeFile(t, filepath.Join(mountpoint, "enforce"), "unexpected\n")

	core := selinuxFacts(filepath.Join(dir, "mounts"), filepath.Join(dir, "config"), os.ReadFile)
	selinux := Collection(core)["os"].(map[string]any)["selinux"].(map[string]any)
	if _, ok := selinux["enforced"]; ok {
		t.Fatalf("os.selinux.enforced = %#v, want omitted for invalid enforce content", selinux["enforced"])
	}
	if _, ok := selinux["current_mode"]; ok {
		t.Fatalf("os.selinux.current_mode = %#v, want omitted for invalid enforce content", selinux["current_mode"])
	}
}
