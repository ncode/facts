package engine

import (
	"os"
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
	host := &fakeHostOS{
		emptyRunDefault: true,
		stats:           map[string]os.FileInfo{"/opt/puppetlabs/puppet/bin/augparse": fakeFileInfo{name: "augparse"}},
		runOutputs: map[string]string{
			fakeRunKey("/opt/puppetlabs/puppet/bin/augparse", "--version"): "augparse 1.12.0 <http://augeas.net/>",
		},
	}
	s := NewSession()
	s.host = host

	got := currentAugeasVersion(s)

	if got != "1.12.0" {
		t.Fatalf("currentAugeasVersion() = %q, want 1.12.0", got)
	}
	want := []fakeHostRunCall{{name: "/opt/puppetlabs/puppet/bin/augparse", args: []string{"--version"}}}
	if !reflect.DeepEqual(host.runCalls, want) {
		t.Fatalf("commands = %#v, want puppet-agent augparse --version", host.runCalls)
	}
}

func TestCurrentAugeasVersion_usesPathAugparseWhenPuppetAgentAugparseIsAbsent(t *testing.T) {
	host := &fakeHostOS{
		emptyRunDefault: true,
		runOutputs: map[string]string{
			fakeRunKey("augparse", "--version"): "augparse 1.14.1 <http://augeas.net/>",
		},
	}
	s := NewSession()
	s.host = host

	got := currentAugeasVersion(s)

	if got != "1.14.1" {
		t.Fatalf("currentAugeasVersion() = %q, want 1.14.1", got)
	}
	want := []fakeHostRunCall{{name: "augparse", args: []string{"--version"}}}
	if !reflect.DeepEqual(host.runCalls, want) {
		t.Fatalf("commands = %#v, want path augparse --version", host.runCalls)
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
