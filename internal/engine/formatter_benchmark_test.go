package engine

import "testing"

func BenchmarkFormatJSON(b *testing.B) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin"},
		{Name: "os.family", Value: "Darwin"},
		{Name: "os.architecture", Value: "arm64"},
		{Name: "processors.count", Value: 10},
		{Name: "processors.models", Value: []string{"Apple M4 Pro"}},
		{Name: "networking.hostname", Value: "host"},
	}
	projection := NewProjection(facts, false)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := FormatJSON(projection); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatLegacy(b *testing.B) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin"},
		{Name: "os.family", Value: "Darwin"},
		{Name: "os.architecture", Value: "arm64"},
		{Name: "processors.count", Value: 10},
		{Name: "processors.models", Value: []string{"Apple M4 Pro"}},
		{Name: "networking.hostname", Value: "host"},
	}
	projection := NewProjection(facts, false)

	b.ReportAllocs()
	for b.Loop() {
		_ = FormatLegacy(projection)
	}
}

func BenchmarkFormatHOCON(b *testing.B) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin"},
		{Name: "os.family", Value: "Darwin"},
		{Name: "os.architecture", Value: "arm64"},
		{Name: "processors.count", Value: 10},
		{Name: "processors.models", Value: []string{"Apple M4 Pro"}},
		{Name: "networking.hostname", Value: "host"},
	}
	projection := NewProjection(facts, false)

	b.ReportAllocs()
	for b.Loop() {
		_ = FormatHOCON(projection)
	}
}
