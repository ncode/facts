package engine

import "regexp"

var augeasVersionPattern = regexp.MustCompile(`\b(\d+\.\d+(?:\.\d+)?)\b`)

func probeAugeasVersion(s *Session) string {
	return currentAugeasVersion(s)
}

func currentAugeasVersion(s *Session) string {
	augparse := "augparse"
	if fileExists(s.host, "/opt/puppetlabs/puppet/bin/augparse") {
		augparse = "/opt/puppetlabs/puppet/bin/augparse"
	}
	return parseAugeasVersion(s.commandOutput(augparse, "--version"))
}

func parseAugeasVersion(out string) string {
	return augeasVersionPattern.FindString(out)
}

func augeasFacts(out string) []ResolvedFact {
	return augeasVersionFacts(parseAugeasVersion(out))
}

// augeasVersionFacts returns the augeas fact, or nothing when no augparse
// binary produced a version: Ruby Facter omits the fact instead of emitting
// an empty version string.
func augeasVersionFacts(version string) []ResolvedFact {
	if version == "" {
		return nil
	}
	return []ResolvedFact{
		{Name: "augeas.version", Value: version},
	}
}

// augeasCoreFacts assembles the augeas category fact (augeasversion), emitted
// only when augparse is available.
func augeasCoreFacts(s *Session) []ResolvedFact {
	return augeasVersionFacts(s.cachedAugeasVersion())
}
