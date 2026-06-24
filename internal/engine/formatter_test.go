package engine

import (
	"strings"
	"testing"
)

func TestBuildFormatterMatchesRubyFormatterFactory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts FormatOptions
		want string
	}{
		{name: "json", opts: FormatOptions{JSON: true}, want: "json"},
		{name: "yaml", opts: FormatOptions{YAML: true}, want: "yaml"},
		{name: "hocon", opts: FormatOptions{HOCON: true}, want: "hocon"},
		{name: "legacy", opts: FormatOptions{}, want: "legacy"},
		{name: "json preferred over yaml", opts: FormatOptions{JSON: true, YAML: true}, want: "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := BuildFormatter(tt.opts)
			if got.Name() != tt.want {
				t.Fatalf("BuildFormatter(%#v).Name() = %q, want %q", tt.opts, got.Name(), tt.want)
			}
		})
	}
}

func TestBuildFormatterWiresDottedAndColorOptions(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin"},
		{Name: "site.role", Value: "web", Type: "external"},
	}
	tests := []struct {
		name string
		opts FormatOptions
		want func([]ResolvedFact) (string, error)
	}{
		{
			name: "json dotted",
			opts: FormatOptions{JSON: true, IncludeTypedDotted: true},
			want: func(facts []ResolvedFact) (string, error) {
				return FormatJSONWithDottedFacts(facts, true)
			},
		},
		{
			name: "yaml dotted",
			opts: FormatOptions{YAML: true, IncludeTypedDotted: true},
			want: func(facts []ResolvedFact) (string, error) {
				return FormatYAMLWithDottedFacts(facts, true), nil
			},
		},
		{
			name: "hocon dotted",
			opts: FormatOptions{HOCON: true, IncludeTypedDotted: true},
			want: func(facts []ResolvedFact) (string, error) {
				return FormatHOCONWithDottedFacts(facts, true), nil
			},
		},
		{
			name: "legacy dotted color",
			opts: FormatOptions{IncludeTypedDotted: true, Colorize: true},
			want: func(facts []ResolvedFact) (string, error) {
				return FormatLegacyColored(facts, true, true), nil
			},
		},
		{
			name: "legacy plain",
			opts: FormatOptions{},
			want: func(facts []ResolvedFact) (string, error) {
				return FormatLegacyColored(facts, false, false), nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildFormatter(tt.opts).Format(facts)
			if err != nil {
				t.Fatal(err)
			}
			want, err := tt.want(facts)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("BuildFormatter(%#v).Format() = %q, want %q", tt.opts, got, want)
			}
		})
	}
}

func TestBuildFormatterMachineFormatsIgnoreColorize(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin"},
		{Name: "site.role", Value: "web", Type: "external"},
	}
	tests := []struct {
		name string
		opts FormatOptions
	}{
		{name: "json", opts: FormatOptions{JSON: true, IncludeTypedDotted: true}},
		{name: "yaml", opts: FormatOptions{YAML: true, IncludeTypedDotted: true}},
		{name: "hocon", opts: FormatOptions{HOCON: true, IncludeTypedDotted: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plain, err := BuildFormatter(tt.opts).Format(facts)
			if err != nil {
				t.Fatal(err)
			}
			colorizedOpts := tt.opts
			colorizedOpts.Colorize = true
			colorized, err := BuildFormatter(colorizedOpts).Format(facts)
			if err != nil {
				t.Fatal(err)
			}
			if colorized != plain {
				t.Fatalf("BuildFormatter(%#v).Format() = %q, want byte-identical %q", colorizedOpts, colorized, plain)
			}
		})
	}
}

func TestFormatJSON_noUserQueryBuildsStructuredFacts(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin"},
		{Name: "os.family", Value: "Darwin"},
		{Name: "os.architecture", Value: "x86_64"},
	}

	got, err := FormatJSON(facts)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"os\": {\n    \"architecture\": \"x86_64\",\n    \"family\": \"Darwin\",\n    \"name\": \"Darwin\"\n  }\n}"
	if got != want {
		t.Fatalf("FormatJSON() = %q, want %q", got, want)
	}
}

func TestFormatJSON_userQueriesUseOriginalQueryKeys(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin", UserQuery: "os.name"},
		{Name: "os.family", Value: "Darwin", UserQuery: "os.family"},
	}

	got, err := FormatJSON(facts)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"os.family\": \"Darwin\",\n  \"os.name\": \"Darwin\"\n}"
	if got != want {
		t.Fatalf("FormatJSON() = %q, want %q", got, want)
	}
}

func TestFormatJSON_userQueriesRenderStructuredEdgeValues(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "networking", Value: map[string]any{"ip6": "::1", "maybe": nil}, UserQuery: "networking"},
		{Name: "roles", Value: []any{"web", "db"}, UserQuery: "roles"},
		{Name: "path", Value: `C:\Program Files\Puppet Labs\Puppet\bin`, UserQuery: "path"},
		{Name: "a.b", Value: map[string]any{"c": "d"}, UserQuery: "a.b"},
	}

	got, err := FormatJSON(facts)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"a.b\": {\n    \"c\": \"d\"\n  },\n  \"networking\": {\n    \"ip6\": \"::1\",\n    \"maybe\": null\n  },\n  \"path\": \"C:\\\\Program Files\\\\Puppet Labs\\\\Puppet\\\\bin\",\n  \"roles\": [\n    \"web\",\n    \"db\"\n  ]\n}"
	if got != want {
		t.Fatalf("FormatJSON() = %q, want %q", got, want)
	}
}

func TestFormatYAML_noUserQueryBuildsStructuredFacts(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin"},
		{Name: "os.family", Value: "Darwin"},
		{Name: "os.architecture", Value: "x86_64"},
	}

	got := FormatYAML(facts)
	want := "os:\n  architecture: x86_64\n  family: \"Darwin\"\n  name: \"Darwin\"\n"
	if got != want {
		t.Fatalf("FormatYAML() = %q, want %q", got, want)
	}
}

func TestFormatYAML_singleUserQueryUsesOriginalQueryKey(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin", UserQuery: "os.name"},
	}

	got := FormatYAML(facts)
	want := "os.name: \"Darwin\"\n"
	if got != want {
		t.Fatalf("FormatYAML() = %q, want %q", got, want)
	}
}

func TestFormatYAML_multipleUserQueriesUseOriginalQueryKeys(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin", UserQuery: "os.name"},
		{Name: "os.family", Value: "Darwin", UserQuery: "os.family"},
	}

	got := FormatYAML(facts)
	want := "os.family: \"Darwin\"\nos.name: \"Darwin\"\n"
	if got != want {
		t.Fatalf("FormatYAML() = %q, want %q", got, want)
	}
}

func TestFormatYAML_quotesUnsafeKeys(t *testing.T) {
	tests := []struct {
		name string
		fact ResolvedFact
		want string
	}{
		{
			name: "control character",
			fact: ResolvedFact{Name: "bad\nkey", Value: "value", Type: "external"},
			want: "\"bad\\nkey\": value\n",
		},
		{
			name: "boolean token",
			fact: ResolvedFact{Name: "true", Value: "value", Type: "external"},
			want: "\"true\": value\n",
		},
		{
			name: "numeric token",
			fact: ResolvedFact{Name: "123", Value: "value", Type: "external"},
			want: "\"123\": value\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatYAML([]ResolvedFact{tt.fact})
			if got != tt.want {
				t.Fatalf("FormatYAML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatYAML_formatsArrayValuesAsSequences(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "arr_ext_fact", Value: []any{"ex1", "ex2"}, UserQuery: "arr_ext_fact"},
	}

	got := FormatYAML(facts)
	want := "arr_ext_fact:\n- ex1\n- ex2\n"
	if got != want {
		t.Fatalf("FormatYAML() = %q, want %q", got, want)
	}
}

func TestFormatYAML_quotesWindowsPath(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "path", Value: `C:\Program Files\Puppet Labs\Puppet\bin;C:\cygwin64\bin`},
	}

	got := FormatYAML(facts)
	want := "path: \"C:\\\\Program Files\\\\Puppet Labs\\\\Puppet\\\\bin;C:\\\\cygwin64\\\\bin\"\n"
	if got != want {
		t.Fatalf("FormatYAML() = %q, want %q", got, want)
	}
}

func TestFormatYAML_formatsFloatWithoutQuotes(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "load_average", Value: 1.35},
	}

	got := FormatYAML(facts)
	want := "load_average: 1.35\n"
	if got != want {
		t.Fatalf("FormatYAML() = %q, want %q", got, want)
	}
}

func TestFormatYAML_formatsNestedArrayValuesAsYAML(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "nested", Value: []any{[]any{"a", "b"}, map[string]any{"name": "c"}}, UserQuery: "nested"},
	}

	got := FormatYAML(facts)
	want := "nested:\n- [a, b]\n- name: c\n"
	if got != want {
		t.Fatalf("FormatYAML() = %q, want %q", got, want)
	}
}

func TestFormatYAML_quotesStringValuesThatYAMLWouldParseAsScalars(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "feature.enabled", Value: "true"},
		{Name: "feature.disabled", Value: "false"},
		{Name: "feature.empty", Value: "null"},
		{Name: "feature.upper_enabled", Value: "TRUE"},
		{Name: "feature.upper_disabled", Value: "FALSE"},
		{Name: "feature.upper_empty", Value: "NULL"},
	}

	got := FormatYAML(facts)
	want := "feature:\n  disabled: 'false'\n  empty: \"null\"\n  enabled: 'true'\n  upper_disabled: \"FALSE\"\n  upper_empty: \"NULL\"\n  upper_enabled: \"TRUE\"\n"
	if got != want {
		t.Fatalf("FormatYAML() = %q, want %q", got, want)
	}
}

func TestFormatYAML_userQueriesRenderMapsNilWindowsPathIPv6AndScalarQuoting(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "networking", Value: map[string]any{"ip6": "::1", "maybe": nil}, UserQuery: "networking"},
		{Name: "roles", Value: []any{"web", "db"}, UserQuery: "roles"},
		{Name: "path", Value: `C:\Program Files\Puppet Labs\Puppet\bin`, UserQuery: "path"},
		{Name: "enabled", Value: "true", UserQuery: "enabled"},
		{Name: "empty", Value: "null", UserQuery: "empty"},
		{Name: "under", Value: "x_y", UserQuery: "under"},
	}

	got := FormatYAML(facts)
	want := "empty: \"null\"\nenabled: 'true'\nnetworking:\n  ip6: \"::1\"\n  maybe: \"\"\npath: \"C:\\\\Program Files\\\\Puppet Labs\\\\Puppet\\\\bin\"\nroles:\n- web\n- db\nunder: \"x_y\"\n"
	if got != want {
		t.Fatalf("FormatYAML() = %q, want %q", got, want)
	}
}

func TestFormatHOCON_noUserQueryBuildsStructuredFacts(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin"},
		{Name: "os.family", Value: "Darwin"},
		{Name: "os.architecture", Value: "x86_64"},
	}

	got := FormatHOCON(facts)
	want := "os={\n    architecture=\"x86_64\"\n    family=Darwin\n    name=Darwin\n}\n"
	if got != want {
		t.Fatalf("FormatHOCON() = %q, want %q", got, want)
	}
}

func TestFormatHOCON_singleUserQueryReturnsScalar(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin", UserQuery: "os.name"},
	}

	if got, want := FormatHOCON(facts), "Darwin"; got != want {
		t.Fatalf("FormatHOCON() = %q, want %q", got, want)
	}
}

func TestFormatHOCON_multipleUserQueriesUseQuotedQueryKeys(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "os.name", Value: "Darwin", UserQuery: "os.name"},
		{Name: "os.family", Value: "Darwin", UserQuery: "os.family"},
	}

	got := FormatHOCON(facts)
	want := "\"os.family\"=Darwin\n\"os.name\"=Darwin\n"
	if got != want {
		t.Fatalf("FormatHOCON() = %q, want %q", got, want)
	}
}

func TestFormatHOCON_quotesUnsafeKeys(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "bad\x07key", Value: "value", Type: "external"},
	}

	got := FormatHOCON(facts)
	want := "\"bad\\u0007key\"=value\n"
	if got != want {
		t.Fatalf("FormatHOCON() = %q, want %q", got, want)
	}
}

func TestFormatHOCON_emptyFactsReturnsEmptyOutput(t *testing.T) {
	if got := FormatHOCON(nil); got != "" {
		t.Fatalf("FormatHOCON() = %q, want empty", got)
	}
}

func TestFormatHOCON_formatsArrayValues(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "processors.models", Value: []any{"Apple M4 Pro", "Apple M4 Max"}},
	}

	got := FormatHOCON(facts)
	want := "processors={\n    models=[\"Apple M4 Pro\",\"Apple M4 Max\"]\n}\n"
	if got != want {
		t.Fatalf("FormatHOCON() = %q, want %q", got, want)
	}
}

func TestFormatHOCON_quotesUnsafeStringValues(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "external.payload", Value: "a=b # not syntax"},
	}

	got := FormatHOCON(facts)
	want := "external={\n    payload=\"a=b # not syntax\"\n}\n"
	if got != want {
		t.Fatalf("FormatHOCON() = %q, want %q", got, want)
	}
}

func TestFormatHOCON_preservesFloatPrecision(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "load_average", Value: 1.35},
	}

	got := FormatHOCON(facts)
	want := "load_average=1.35\n"
	if got != want {
		t.Fatalf("FormatHOCON() = %q, want %q", got, want)
	}
}

func TestFormatHOCON_singleNilQueryReturnsEmptyScalar(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "my_external_fact", UserQuery: "my_external_fact", Value: nil},
	}

	if got := FormatHOCON(facts); got != "" {
		t.Fatalf("FormatHOCON() = %q, want empty", got)
	}
}

func TestFormatLegacy_singleUserQueryDigsIntoArraysAndMaps(t *testing.T) {
	fact := ResolvedFact{
		Name:      "my.nested.fact",
		UserQuery: "my.nested.fact.1.name",
		Value: []any{
			"first",
			map[string]any{"name": "second"},
		},
	}

	if got, want := FormatLegacy([]ResolvedFact{fact}), "second"; got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_wrongNestedQueryWithStringLeafReturnsEmpty(t *testing.T) {
	fact := ResolvedFact{
		Name:      "mountpoints",
		UserQuery: "mountpoints./tmp.asd",
		Value:     map[string]any{"/tmp": "something"},
	}

	if got := FormatLegacy([]ResolvedFact{fact}); got != "" {
		t.Fatalf("FormatLegacy() = %q, want empty", got)
	}
}

func TestFormatLegacy_multipleQueriesPrintEmptyValues(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "nil_resolved_fact1", UserQuery: "nil_resolved_fact1"},
		{Name: "resolved_fact2", UserQuery: "resolved_fact2", Value: "resolved_fact2_value"},
	}

	want := "nil_resolved_fact1 => \nresolved_fact2 => resolved_fact2_value"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_multipleQueriesPrintNestedNilValuesWithUserQueryKey(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "my.nested.fact2", UserQuery: "my.nested.fact2"},
		{Name: "nil_resolved_fact1", UserQuery: "nil_resolved_fact1"},
	}

	want := "my.nested.fact2 => \nnil_resolved_fact1 => "
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryOmitsNilFacts(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "nil_resolved_fact1"},
		{Name: "resolved_fact2", Value: "resolved_fact2_value"},
	}

	want := "resolved_fact2 => resolved_fact2_value"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryPreservesNewlinesInValues(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "custom_fact", Value: "value1 \n value2"},
	}

	want := "custom_fact => value1 \n value2"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_quotesIPv6StringsInsideMaps(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "networking", UserQuery: "networking", Value: map[string]any{"ip6": "::1"}},
	}

	want := "{\n  ip6 => \"::1\"\n}"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryPreservesWindowsPathFactNames(t *testing.T) {
	facts := []ResolvedFact{
		{Name: `C:\Program Files\App`, Value: "bin_dir"},
	}

	want := `C:\Program Files\App => bin_dir`
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryPreservesWindowsPathValues(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "path", Value: `C:\Program Files\Puppet Labs\Puppet\bin;C:\cygwin64\bin`},
	}

	want := `path => C:\Program Files\Puppet Labs\Puppet\bin;C:\cygwin64\bin`
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

// The expectations below are pinned to Ruby Facter 4.10.0's
// LegacyFactFormatter output, verified by running the gem's exact gsub
// pipeline (lib/facter/framework/formatters/legacy_fact_formatter.rb) on the
// same inputs.

func TestFormatLegacy_noUserQueryQuotesNestedStringsAndSeparatesEntriesWithCommas(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "identity", Value: map[string]any{
			"gid":        20,
			"group":      "staff",
			"privileged": false,
			"uid":        501,
			"user":       "ncode",
		}},
		{Name: "kernel", Value: "Darwin"},
	}

	want := "identity => {\n  gid => 20,\n  group => \"staff\",\n  privileged => false,\n  uid => 501,\n  user => \"ncode\"\n}\nkernel => Darwin"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryRendersArraysMultiLine(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "processors", Value: map[string]any{
			"count":  2,
			"models": []any{"Apple M4", "Apple M4"},
		}},
	}

	want := "processors => {\n  count => 2,\n  models => [\n    \"Apple M4\",\n    \"Apple M4\"\n  ]\n}"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryRendersTopLevelArray(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "roles", Value: []any{"web", "db"}},
	}

	want := "roles => [\n  \"web\",\n  \"db\"\n]"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryStripsCommaBetweenAdjacentTopLevelMaps(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "a", Value: map[string]any{"x": 1}},
		{Name: "b", Value: map[string]any{"y": 2}},
	}

	want := "a => {\n  x => 1\n}\nb => {\n  y => 2\n}"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryRendersFloatsLikeRuby(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "load_averages", Value: map[string]any{"15m": 1.35, "1m": 1.46, "5m": 1.4}},
	}

	want := "load_averages => {\n  15m => 1.35,\n  1m => 1.46,\n  5m => 1.4\n}"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryRendersEmptyMap(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "mountpoints", Value: map[string]any{}},
	}

	if got, want := FormatLegacy(facts), "mountpoints => {}"; got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryUnescapesQuotesInTopLevelStringValues(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "motd", Value: `say "hi" now`},
	}

	if got, want := FormatLegacy(facts), `motd => say "hi" now`; got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

// A value containing the `": ` byte sequence collides with Ruby's
// /":\s/ key-delimiter rewrite and the greedy key unquote; the mangled result
// below is exactly what Ruby Facter 4.10.0 prints. Identical mangling on both
// sides is the parity contract — this test documents the quirk.
func TestFormatLegacy_valueContainingKeyDelimiterManglesExactlyLikeRuby(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "weird", Value: `x": y`},
	}

	if got, want := FormatLegacy(facts), `weird => x\ => y`; got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_multipleQueriesUnquoteTopLevelStringsAndRenderNilEmpty(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "identity.user", Value: "ncode", UserQuery: "identity.user"},
		{Name: "kernel", Value: "Darwin", UserQuery: "kernel"},
		{Name: "missing", Value: nil, UserQuery: "missing"},
		{Name: "load", Value: 1.35, UserQuery: "load"},
	}

	want := "identity.user => ncode\nkernel => Darwin\nload => 1.35\nmissing => "
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_multipleQueriesKeepNestedQuotingAndCommas(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "identity", Value: map[string]any{"gid": 20, "user": "ncode"}, UserQuery: "identity"},
		{Name: "kernel", Value: "Darwin", UserQuery: "kernel"},
	}

	want := "identity => {\n  gid => 20,\n  user => \"ncode\"\n}\nkernel => Darwin"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_singleQueryKeepsBracesCommasAndNestedQuotes(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "identity", UserQuery: "identity", Value: map[string]any{
			"gid":   20,
			"group": "staff",
			"user":  "ncode",
		}},
	}

	want := "{\n  gid => 20,\n  group => \"staff\",\n  user => \"ncode\"\n}"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_singleQueryRendersArrayMultiLineWithQuotedElements(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "processors.models", UserQuery: "processors.models", Value: []any{"Apple M4", "Apple M4"}},
	}

	want := "[\n  \"Apple M4\",\n  \"Apple M4\"\n]"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

// Single-query mode never runs Ruby's handle_newlines: a literal \n stays as
// the two-character sequence, unlike full output where it expands.
func TestFormatLegacy_singleQueryDoesNotExpandEmbeddedNewlines(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "ssh", UserQuery: "ssh", Value: map[string]any{"key": "a\nb"}},
	}

	want := "{\n  key => \"a\\nb\"\n}"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_noUserQueryExpandsEmbeddedNewlines(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "sshfp", Value: "SSHFP 1 1 abc\nSSHFP 2 2 def"},
	}

	want := "sshfp => SSHFP 1 1 abc\nSSHFP 2 2 def"
	if got := FormatLegacy(facts); got != want {
		t.Fatalf("FormatLegacy() = %q, want %q", got, want)
	}
}

func TestFormatLegacy_singleQueryNonStringScalars(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "int", value: 42, want: "42"},
		{name: "float", value: 1.35, want: "1.35"},
		{name: "bool", value: false, want: "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			facts := []ResolvedFact{{Name: "q", UserQuery: "q", Value: tt.value}}
			if got := FormatLegacy(facts); got != tt.want {
				t.Fatalf("FormatLegacy() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatLegacyColored_colorsKeysByNestingDepth(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "dmi", Value: map[string]any{"product": map[string]any{"name": "Mac16,10"}}},
		{Name: "kernel", Value: "Darwin"},
	}

	want := "\x1b[36mdmi\x1b[0m => {\n" +
		"  \x1b[33mproduct\x1b[0m => {\n" +
		"    \x1b[32mname\x1b[0m => \"Mac16,10\"\n" +
		"  }\n" +
		"}\n" +
		"\x1b[36mkernel\x1b[0m => Darwin"
	if got := FormatLegacyColored(facts, false, true); got != want {
		t.Fatalf("FormatLegacyColored() = %q, want %q", got, want)
	}
}

func TestFormatLegacyColored_paletteCyclesPastDepthFive(t *testing.T) {
	depth5 := map[string]any{"f": "leaf"}
	tree := map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"d": map[string]any{"e": depth5}}}}}
	facts := []ResolvedFact{{Name: "root", Value: tree}}

	got := FormatLegacyColored(facts, false, true)
	for _, want := range []string{
		"\x1b[36mroot\x1b[0m => {",        // depth 0: cyan
		"  \x1b[33ma\x1b[0m => {",         // depth 1: yellow
		"    \x1b[32mb\x1b[0m => {",       // depth 2: green
		"      \x1b[35mc\x1b[0m => {",     // depth 3: magenta
		"        \x1b[34md\x1b[0m => {",   // depth 4: blue
		"          \x1b[36me\x1b[0m => {", // depth 5 cycles back to cyan
		"            \x1b[33mf\x1b[0m => \"leaf\"",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatLegacyColored() = %q, want substring %q", got, want)
		}
	}
}

func TestFormatLegacyColored_offLeavesOutputByteIdentical(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "identity", Value: map[string]any{"user": "ncode"}},
		{Name: "kernel", Value: "Darwin"},
	}

	plain := FormatLegacy(facts)
	if got := FormatLegacyColored(facts, false, false); got != plain {
		t.Fatalf("FormatLegacyColored(colorize=false) = %q, want %q", got, plain)
	}
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("FormatLegacy() = %q, want no ANSI escapes", plain)
	}
}

func TestFormatLegacyColored_singleQueryColorsKeysOnly(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "identity", UserQuery: "identity", Value: map[string]any{"gid": 20, "user": "ncode"}},
	}

	want := "{\n  \x1b[33mgid\x1b[0m => 20,\n  \x1b[33muser\x1b[0m => \"ncode\"\n}"
	if got := FormatLegacyColored(facts, false, true); got != want {
		t.Fatalf("FormatLegacyColored() = %q, want %q", got, want)
	}
}

func TestFormatLegacyColored_multipleQueriesColorTopLevelKeys(t *testing.T) {
	facts := []ResolvedFact{
		{Name: "kernel", Value: "Darwin", UserQuery: "kernel"},
		{Name: "identity", Value: map[string]any{"user": "ncode"}, UserQuery: "identity"},
	}

	want := "\x1b[36midentity\x1b[0m => {\n  \x1b[33muser\x1b[0m => \"ncode\"\n}\n\x1b[36mkernel\x1b[0m => Darwin"
	if got := FormatLegacyColored(facts, false, true); got != want {
		t.Fatalf("FormatLegacyColored() = %q, want %q", got, want)
	}
}

func TestValueForQueryRejectsInvalidArrayIndexes(t *testing.T) {
	fact := ResolvedFact{Name: "arr_fact", Value: []string{"x", "y", "z"}}

	tests := []struct {
		name  string
		query string
		want  any
	}{
		{name: "valid", query: "arr_fact.0", want: "x"},
		{name: "missing", query: "arr_fact.3", want: nil},
		{name: "non numeric", query: "arr_fact.abc", want: nil},
		{name: "negative", query: "arr_fact.-1", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fact.UserQuery = tt.query
			if got := ValueForQuery(fact); got != tt.want {
				t.Fatalf("ValueForQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}
