package engine

import (
	"reflect"
	"testing"
)

func TestAugeasFacts_returnsStructuredVersion(t *testing.T) {
	got := augeasFacts("augparse 1.14.1 <http://augeas.net/>")
	want := []ResolvedFact{
		{Name: "augeas.version", Value: "1.14.1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("augeasFacts() = %#v, want %#v", got, want)
	}
}

func TestAugeasFacts_skipsMissingVersion(t *testing.T) {
	if got := augeasFacts(""); got != nil {
		t.Fatalf("augeasFacts() = %#v, want nil", got)
	}
}

func TestAugeasVersionFacts_omittedWhenAugparseUnavailable(t *testing.T) {
	t.Parallel()

	if got := augeasVersionFacts(""); got != nil {
		t.Fatalf("augeasVersionFacts(\"\") = %#v, want nil", got)
	}
	want := []ResolvedFact{{Name: "augeas.version", Value: "1.14.1"}}
	if got := augeasVersionFacts("1.14.1"); !reflect.DeepEqual(got, want) {
		t.Fatalf("augeasVersionFacts(1.14.1) = %#v, want %#v", got, want)
	}
}

func TestCurrentAugeasVersion_prefersPuppetAgentAugparse(t *testing.T) {
	var gotName string
	var gotArgs []string

	got := currentAugeasVersion(
		func(path string) bool { return path == "/opt/puppetlabs/puppet/bin/augparse" },
		func(name string, args ...string) string {
			gotName = name
			gotArgs = args
			return "augparse 1.12.0 <http://augeas.net/>"
		},
	)

	if got != "1.12.0" {
		t.Fatalf("currentAugeasVersion() = %q, want 1.12.0", got)
	}
	if gotName != "/opt/puppetlabs/puppet/bin/augparse" {
		t.Fatalf("augparse command = %q, want puppet-agent augparse", gotName)
	}
	if !reflect.DeepEqual(gotArgs, []string{"--version"}) {
		t.Fatalf("augparse args = %#v, want --version", gotArgs)
	}
}

func TestCurrentAugeasVersion_usesPathAugparseWhenPuppetAgentAugparseIsAbsent(t *testing.T) {
	var gotName string

	got := currentAugeasVersion(
		func(string) bool { return false },
		func(name string, args ...string) string {
			gotName = name
			return "augparse 1.14.1 <http://augeas.net/>"
		},
	)

	if got != "1.14.1" {
		t.Fatalf("currentAugeasVersion() = %q, want 1.14.1", got)
	}
	if gotName != "augparse" {
		t.Fatalf("augparse command = %q, want path augparse", gotName)
	}
}

func TestParseAugeasVersion(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "augparse", out: "augparse 1.14.1 <http://augeas.net/>", want: "1.14.1"},
		{name: "package suffix", out: "augparse 1.12.0-2ubuntu1", want: "1.12.0"},
		{name: "no version", out: "augparse unknown", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAugeasVersion(tt.out); got != tt.want {
				t.Fatalf("parseAugeasVersion(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}
