package engine

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
)

type sshHostKey struct {
	Name   string
	Type   string
	Key    string
	SHA1   string
	SHA256 string
}

func discoverSSHHostKeysForPlatform(goos, programData string, readFile fileReader) []sshHostKey {
	const maxHostKeys = 20
	paths := []string{"/etc/ssh", "/usr/local/etc/ssh", "/etc", "/usr/local/etc", "/etc/opt/ssh"}
	if goos == "windows" {
		if programData == "" {
			return nil
		}
		paths = []string{sshJoin(goos, programData, "ssh")}
	}
	files := []string{"ssh_host_rsa_key.pub", "ssh_host_dsa_key.pub", "ssh_host_ecdsa_key.pub", "ssh_host_ed25519_key.pub"}
	keys := make([]sshHostKey, 0, len(files))
	for _, dir := range paths {
		for _, file := range files {
			data, err := readFile(sshJoin(goos, dir, file))
			if err != nil {
				continue
			}
			key, ok := parseSSHHostPublicKey(string(data))
			if !ok {
				continue
			}
			keys = append(keys, key)
			if len(keys) >= maxHostKeys {
				return keys
			}
		}
	}
	return keys
}

func sshJoin(goos, dir, name string) string {
	if goos == "windows" {
		return strings.TrimRight(dir, `\/`) + `\` + name
	}
	return filepath.Join(dir, name)
}

func parseSSHHostPublicKey(line string) (sshHostKey, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return sshHostKey{}, false
	}
	name, fingerprintAlgorithm, ok := sshKeyName(fields[0])
	if !ok {
		return sshHostKey{}, false
	}
	if fields[1] == "" {
		return sshHostKey{}, false
	}
	decoded, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return sshHostKey{}, false
	}
	sha1Sum := sha1.Sum(decoded)
	sha256Sum := sha256.Sum256(decoded)
	return sshHostKey{
		Name:   name,
		Type:   fields[0],
		Key:    fields[1],
		SHA1:   fmt.Sprintf("SSHFP %d 1 %x", fingerprintAlgorithm, sha1Sum),
		SHA256: fmt.Sprintf("SSHFP %d 2 %x", fingerprintAlgorithm, sha256Sum),
	}, true
}

func sshKeyName(keyType string) (string, int, bool) {
	switch keyType {
	case "ssh-rsa":
		return "rsa", 1, true
	case "ssh-dss":
		return "dsa", 2, true
	case "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521":
		return "ecdsa", 3, true
	case "ssh-ed25519":
		return "ed25519", 4, true
	default:
		return "", 0, false
	}
}

func sshFacts(keys []sshHostKey) []ResolvedFact {
	return sshFactsForPlatform("", keys)
}

func sshFactsForPlatform(goos string, keys []sshHostKey) []ResolvedFact {
	if len(keys) == 0 {
		if goos == "openbsd" {
			return []ResolvedFact{{Name: "ssh", Value: map[string]any{}}}
		}
		return []ResolvedFact{{Name: "ssh", Value: nil}}
	}
	structured := make(map[string]any, len(keys))
	for _, key := range keys {
		if _, exists := structured[key.Name]; exists {
			continue
		}
		structured[key.Name] = map[string]any{
			"fingerprints": map[string]any{
				"sha1":   key.SHA1,
				"sha256": key.SHA256,
			},
			"key":  key.Key,
			"type": key.Type,
		}
	}
	return []ResolvedFact{{Name: "ssh", Value: structured}}
}

func sshFactsForPlatformWithPrivilege(goos string, privileged bool, discover func() []sshHostKey) []ResolvedFact {
	if goos == "windows" && !privileged {
		return []ResolvedFact{{Name: "ssh", Value: nil}}
	}
	return sshFactsForPlatform(goos, discover())
}

func identityPrivileged(identity map[string]any) bool {
	privileged, _ := identity["privileged"].(bool)
	return privileged
}

// sshCoreFacts assembles the ssh category fact (the discovered host-key
// fingerprints) for the current host, honoring the Windows privilege gate.
func sshCoreFacts(s *Session) []ResolvedFact {
	goos := s.goos()
	identity := s.cachedIdentity()
	return sshFactsForPlatformWithPrivilege(goos, identityPrivileged(identity), func() []sshHostKey {
		return discoverSSHHostKeysForPlatform(goos, s.getenv("programdata"), s.readFile)
	})
}
