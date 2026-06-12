package engine

import "testing"

func BenchmarkLoadExternalEnvFacts(b *testing.B) {
	env := []string{
		"PATH=/usr/bin:/bin",
		"HOME=/tmp/home",
		"FACTER_site_location=lab",
		"FACTER_role=agent",
		"FACTER_os_name=EnvOS",
		"facter_owner=platform",
	}

	b.ReportAllocs()
	for b.Loop() {
		facts, err := loadExternalEnvFacts(env)
		if err != nil {
			b.Fatal(err)
		}
		if len(facts) != 4 {
			b.Fatalf("len(facts) = %d, want 4", len(facts))
		}
	}
}
