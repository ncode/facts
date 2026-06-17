package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkConfigCompatibilityParsing(b *testing.B) {
	path := filepath.Join(b.TempDir(), "facter.conf")
	content := `facts : {
  blocklist : [ "ec2", "networking" ],
  ttls : [
    { "timezone" : "30 days" },
    { "networking" : "1 hour" }
  ],
}
global : {
  external-dir : [ "/opt/facts", "/etc/facts" ],
  custom-dir : [ "/opt/custom" ],
  no-external-facts : false,
}
fact-groups : {
  cached-custom-facts : [ "site_role", "site_location" ],
  hardware : [ "dmi" ],
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := ParseConfig(path, discardLog()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGroupTTLSeconds(b *testing.B) {
	ttls := []FactTTL{
		{Fact: "timezone", TTL: "30 days"},
		{Fact: "networking", TTL: "1 hour"},
		{Fact: "operating system", TTL: "10000000000000 ns"},
	}

	b.ReportAllocs()
	for b.Loop() {
		seconds, ok := GroupTTLSeconds(ttls, "operating system", discardLog())
		if !ok || seconds != 10000 {
			b.Fatalf("GroupTTLSeconds() = %d, %t; want 10000, true", seconds, ok)
		}
	}
}
