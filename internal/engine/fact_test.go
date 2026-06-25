package engine

import "testing"

func TestFactTypeLabelMatchesRubyResolverMessages(t *testing.T) {
	tests := []struct {
		factType string
		want     string
	}{
		{factType: "", want: "Fact"},
		{factType: "external", want: "External"},
		{factType: "custom", want: "Custom"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := factTypeLabel(tt.factType); got != tt.want {
				t.Fatalf("factTypeLabel(%q) = %q, want %q", tt.factType, got, tt.want)
			}
		})
	}
}
