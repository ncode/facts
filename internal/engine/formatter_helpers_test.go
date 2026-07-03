package engine

// These four zero-flag formatter helpers had one production caller — the
// version fast path — which now routes through BuildFormatter. They survive
// only as terse test conveniences over the *WithDottedFacts/*Colored variants,
// so the production build no longer exports them.

// FormatJSON renders facts using Facter's JSON presentation contract.
func FormatJSON(facts []ResolvedFact) (string, error) {
	return FormatJSONWithDottedFacts(facts, false)
}

// FormatYAML renders facts using Facter's YAML presentation contract.
func FormatYAML(facts []ResolvedFact) string {
	return FormatYAMLWithDottedFacts(facts, false)
}

// FormatHOCON renders facts using Facter's HOCON presentation contract.
func FormatHOCON(facts []ResolvedFact) string {
	return FormatHOCONWithDottedFacts(facts, false)
}

// FormatLegacy renders facts using the original key => value text format.
func FormatLegacy(facts []ResolvedFact) string {
	return FormatLegacyColored(facts, false, false)
}
