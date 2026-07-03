package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSSHHostPublicKeyBuildsStructuredFacts(t *testing.T) {
	entry, ok := parseSSHHostPublicKey("ssh-rsa YWJj root@example")
	if !ok {
		t.Fatal("parseSSHHostPublicKey() ok = false, want true")
	}
	if got, want := entry.Name, "rsa"; got != want {
		t.Fatalf("entry.Name = %q, want %q", got, want)
	}
	if got, want := entry.Type, "ssh-rsa"; got != want {
		t.Fatalf("entry.Type = %q, want %q", got, want)
	}
	if got, want := entry.Key, "YWJj"; got != want {
		t.Fatalf("entry.Key = %q, want %q", got, want)
	}
	if got, want := entry.SHA1, "SSHFP 1 1 a9993e364706816aba3e25717850c26c9cd0d89d"; got != want {
		t.Fatalf("entry.SHA1 = %q, want %q", got, want)
	}
	if got, want := entry.SHA256, "SSHFP 1 2 ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"; got != want {
		t.Fatalf("entry.SHA256 = %q, want %q", got, want)
	}

	collection := Collection(sshFacts([]sshHostKey{entry}))
	ssh, ok := collection["ssh"].(map[string]any)
	if !ok {
		t.Fatalf("ssh fact = %#v, want map", collection["ssh"])
	}
	rsa, ok := ssh["rsa"].(map[string]any)
	if !ok {
		t.Fatalf("ssh.rsa = %#v, want map", ssh["rsa"])
	}
	fingerprints, ok := rsa["fingerprints"].(map[string]any)
	if !ok {
		t.Fatalf("ssh.rsa.fingerprints = %#v, want map", rsa["fingerprints"])
	}
	if fingerprints["sha1"] != entry.SHA1 || fingerprints["sha256"] != entry.SHA256 {
		t.Fatalf("fingerprints = %#v, want sha1/sha256", fingerprints)
	}
	for _, name := range []string{"sshrsakey", "sshfp_rsa", "sshfp_rsa_algorithm"} {
		if got, ok := collection[name]; ok {
			t.Fatalf("%s = %#v, want no legacy alias fact", name, got)
		}
	}
}

func TestParseSSHHostPublicKeyRejectsInvalidBase64Key(t *testing.T) {
	if entry, ok := parseSSHHostPublicKey("ssh-rsa -_YWJj root@example"); ok {
		t.Fatalf("parseSSHHostPublicKey() = %#v, true; want rejected malformed key", entry)
	}
}

func TestDiscoverSSHHostKeysLinuxSearchesRubyPathsAndOrder(t *testing.T) {
	t.Parallel()
	assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t, "linux")
}

func TestDiscoverSSHHostKeysDarwinSearchesRubyPathsAndOrder(t *testing.T) {
	t.Parallel()
	assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t, "darwin")
}

func TestDiscoverSSHHostKeysFreeBSDSearchesRubyPathsAndOrder(t *testing.T) {
	t.Parallel()
	assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t, "freebsd")
}

func TestDiscoverSSHHostKeysOpenBSDSearchesRubyPathsAndOrder(t *testing.T) {
	t.Parallel()
	assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t, "openbsd")
}

func TestDiscoverSSHHostKeysNetBSDSearchesRubyPathsAndOrder(t *testing.T) {
	t.Parallel()
	assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t, "netbsd")
}

func assertDiscoverSSHHostKeysPOSIXSearchesRubyPathsAndOrder(t *testing.T, goos string) {
	t.Helper()

	readFile := func(path string) ([]byte, error) {
		switch path {
		case filepath.Join("/etc", "ssh_host_rsa_key.pub"):
			return []byte("ssh-rsa YWJj root@example"), nil
		case filepath.Join("/etc", "ssh_host_ecdsa_key.pub"):
			return []byte("ecdsa-sha2-nistp256 ZGVm root@example"), nil
		case filepath.Join("/etc/opt/ssh", "ssh_host_ed25519_key.pub"):
			return []byte("ssh-ed25519 Z2hp root@example"), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	got := discoverSSHHostKeysForPlatform(goos, "", readFile)
	want := []struct {
		name string
		typ  string
		key  string
	}{
		{name: "rsa", typ: "ssh-rsa", key: "YWJj"},
		{name: "ecdsa", typ: "ecdsa-sha2-nistp256", key: "ZGVm"},
		{name: "ed25519", typ: "ssh-ed25519", key: "Z2hp"},
	}
	if len(got) != len(want) {
		t.Fatalf("discoverSSHHostKeysForPlatform(%s) returned %d keys, want %d: %#v", goos, len(got), len(want), got)
	}
	for i, wantKey := range want {
		if got[i].Name != wantKey.name || got[i].Type != wantKey.typ || got[i].Key != wantKey.key {
			t.Fatalf("key %d = %#v, want name=%q type=%q key=%q", i, got[i], wantKey.name, wantKey.typ, wantKey.key)
		}
		if got[i].SHA1 == "" || got[i].SHA256 == "" {
			t.Fatalf("key %d = %#v, want populated SSHFP fingerprints", i, got[i])
		}
	}
}

func TestDiscoverSSHHostKeysWindowsReadsProgramDataSSH(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		switch path {
		case `C:\ProgramData\ssh\ssh_host_rsa_key.pub`:
			return []byte("ssh-rsa YWJj root@example"), nil
		case `C:\ProgramData\ssh\ssh_host_ecdsa_key.pub`:
			return []byte("ecdsa-sha2-nistp256 ZGVm root@example"), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	keys := discoverSSHHostKeysForPlatform("windows", `C:\ProgramData`, readFile)

	if len(keys) != 2 {
		t.Fatalf("discoverSSHHostKeysForPlatform() returned %d keys, want 2: %#v", len(keys), keys)
	}
	if keys[0].Name != "rsa" || keys[1].Name != "ecdsa" {
		t.Fatalf("key order = %#v, want rsa then ecdsa", keys)
	}
}

func TestSSHFactsPreserveFirstDuplicateKeyType(t *testing.T) {
	collection := Collection(sshFacts([]sshHostKey{
		{Name: "rsa", Type: "ssh-rsa", Key: "first", SHA1: "first-sha1", SHA256: "first-sha256"},
		{Name: "rsa", Type: "ssh-rsa", Key: "second", SHA1: "second-sha1", SHA256: "second-sha256"},
	}))

	ssh := collection["ssh"].(map[string]any)
	rsa := ssh["rsa"].(map[string]any)
	if got := rsa["key"]; got != "first" {
		t.Fatalf("ssh.rsa.key = %#v, want first discovered key", got)
	}
}

func TestSSHFactsWindowsUnprivilegedSkipsDiscovery(t *testing.T) {
	t.Parallel()

	called := false
	facts := sshFactsForPlatformWithPrivilege("windows", false, func() []sshHostKey {
		called = true
		return []sshHostKey{{Name: "rsa", Type: "ssh-rsa", Key: "YWJj"}}
	})

	if called {
		t.Fatal("unprivileged Windows SSH fact collection discovered host keys")
	}
	collection := Collection(facts)
	if got := collection["ssh"]; got != nil {
		t.Fatalf("ssh = %#v, want nil", got)
	}
	for _, name := range []string{"sshrsakey", "sshfp_rsa"} {
		if got, ok := collection[name]; ok {
			t.Fatalf("%s = %#v, want no legacy alias fact", name, got)
		}
	}
}

func TestSSHFactsOpenBSDEmptyResolverReturnsEmptyStructuredFact(t *testing.T) {
	t.Parallel()

	collection := Collection(sshFactsForPlatform("openbsd", nil))
	got, ok := collection["ssh"].(map[string]any)
	if !ok {
		t.Fatalf("ssh = %#v, want empty map", collection["ssh"])
	}
	if len(got) != 0 {
		t.Fatalf("ssh = %#v, want empty map", got)
	}
}

func TestSSHFactsOpenBSDReturnsStructuredFacts(t *testing.T) {
	t.Parallel()

	keys := []sshHostKey{
		{Name: "ecdsa", Type: "ecdsa", Key: "test", SHA1: "sha11", SHA256: "sha2561"},
		{Name: "rsa", Type: "rsa", Key: "test", SHA1: "sha12", SHA256: "sha2562"},
	}
	collection := Collection(sshFactsForPlatform("openbsd", keys))

	ssh, ok := collection["ssh"].(map[string]any)
	if !ok {
		t.Fatalf("ssh = %#v, want map", collection["ssh"])
	}
	for _, key := range keys {
		entry, ok := ssh[key.Name].(map[string]any)
		if !ok {
			t.Fatalf("ssh.%s = %#v, want map", key.Name, ssh[key.Name])
		}
		fingerprints, ok := entry["fingerprints"].(map[string]any)
		if !ok {
			t.Fatalf("ssh.%s.fingerprints = %#v, want map", key.Name, entry["fingerprints"])
		}
		if fingerprints["sha1"] != key.SHA1 || fingerprints["sha256"] != key.SHA256 {
			t.Fatalf("ssh.%s.fingerprints = %#v, want sha1/sha256", key.Name, fingerprints)
		}
		if got := entry["key"]; got != key.Key {
			t.Fatalf("ssh.%s.key = %#v, want %q", key.Name, got, key.Key)
		}
		if got := entry["type"]; got != key.Type {
			t.Fatalf("ssh.%s.type = %#v, want %q", key.Name, got, key.Type)
		}
		for _, name := range []string{"ssh" + key.Name + "key", "sshfp_" + key.Name} {
			if got, ok := collection[name]; ok {
				t.Fatalf("%s = %#v, want no legacy alias fact", name, got)
			}
		}
	}
}

func TestParseSSHHostPublicKeyRejectsUnknownOrInvalidKeys(t *testing.T) {
	for _, line := range []string{
		"ssh-unknown YWJj root@example",
		"ssh-rsa not-base64 root@example",
		"ssh-rsa -- root@example",
		"ssh-rsa",
		"",
	} {
		t.Run(strings.ReplaceAll(line, " ", "_"), func(t *testing.T) {
			if entry, ok := parseSSHHostPublicKey(line); ok {
				t.Fatalf("parseSSHHostPublicKey(%q) = %#v, true; want false", line, entry)
			}
		})
	}
}

// A fake windows host drives the full windows assembly path — ProgramData env
// lookup (case-insensitive via the Session env seam), whoami privilege gate,
// and backslash-joined host-key paths — from any development platform.
func TestSSHCoreFactsFakeWindowsHostUsesProgramDataAndPrivilegeGate(t *testing.T) {
	host := &fakeHostOS{
		platform:       "windows",
		environEntries: []string{`ProgramData=C:\ProgramData`},
		runOutputs: map[string]string{
			fakeRunKey("whoami"):            "corp\\admin\n",
			fakeRunKey("whoami", "/groups"): `BUILTIN\Administrators S-1-5-32-544 Enabled group` + "\n",
		},
		files: map[string][]byte{
			// Forward-slash key: production joins the windows path with
			// backslashes, but fakeHostOS.readFile looks up through fakeHostPath
			// (filepath.ToSlash), so the fixture must use slashes to match on a
			// real Windows runner (on unix ToSlash is a no-op).
			"C:/ProgramData/ssh/ssh_host_rsa_key.pub": []byte("ssh-rsa AAAA host\n"),
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	facts := sshCoreFacts(s)
	if len(facts) != 1 || facts[0].Name != "ssh" {
		t.Fatalf("sshCoreFacts() = %#v, want one ssh fact", facts)
	}
	structured, ok := facts[0].Value.(map[string]any)
	if !ok || structured["rsa"] == nil {
		t.Fatalf("ssh fact value = %#v, want structured rsa key", facts[0].Value)
	}

	// The same host without administrator membership must not discover keys.
	unprivileged := &fakeHostOS{
		platform:       "windows",
		environEntries: []string{`ProgramData=C:\ProgramData`},
		runOutputs: map[string]string{
			fakeRunKey("whoami"):            "corp\\user\n",
			fakeRunKey("whoami", "/groups"): "Everyone S-1-1-0 Mandatory group\n",
		},
		files: map[string][]byte{
			// Forward-slash key: production joins the windows path with
			// backslashes, but fakeHostOS.readFile looks up through fakeHostPath
			// (filepath.ToSlash), so the fixture must use slashes to match on a
			// real Windows runner (on unix ToSlash is a no-op).
			"C:/ProgramData/ssh/ssh_host_rsa_key.pub": []byte("ssh-rsa AAAA host\n"),
		},
	}
	s2 := NewSessionContext(context.Background())
	s2.host = unprivileged
	facts = sshCoreFacts(s2)
	if len(facts) != 1 || facts[0].Value != nil {
		t.Fatalf("unprivileged sshCoreFacts() = %#v, want nil ssh fact", facts)
	}
	if len(unprivileged.readFileCalls) != 0 {
		t.Fatalf("unprivileged discovery read %v, want no host-key reads", unprivileged.readFileCalls)
	}
}
