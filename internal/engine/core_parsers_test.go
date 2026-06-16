package engine

import (
	"reflect"
	"testing"
)

// numericValue feeds the memory/byte conversions. The contract that matters:
// strings ALWAYS succeed (via Ruby to_i semantics, so junk → 0 but ok=true),
// while a non-numeric, non-string type (e.g. bool) fails. Getting this wrong
// would either drop real string-valued memory facts or accept garbage types.
func TestNumericValue(t *testing.T) {
	tests := []struct {
		name   string
		in     any
		want   float64
		wantOK bool
	}{
		{"nil fails", nil, 0, false},
		{"int", 5, 5, true},
		{"int64", int64(9), 9, true},
		{"float64", 2.5, 2.5, true},
		{"numeric string", "1024", 1024, true},
		{"string with trailing junk uses leading digits", "1024 kB", 1024, true},
		{"non-numeric string still ok at zero", "abc", 0, true},
		{"bool is not numeric", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := numericValue(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("numericValue(%#v) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// byteValue differs from numericValue in two contract-relevant ways: it returns
// the original textual form (so huge values can be echoed verbatim) and a
// non-parseable string FAILS rather than coercing to zero.
func TestByteValue(t *testing.T) {
	tests := []struct {
		name         string
		in           any
		wantNum      float64
		wantOriginal string
		wantOK       bool
	}{
		{"nil fails", nil, 0, "", false},
		{"int keeps decimal text", 42, 42, "42", true},
		{"int64", int64(42), 42, "42", true},
		{"float64 trims trailing zeros", 1.5, 1.5, "1.5", true},
		{"parseable string", "2048", 2048, "2048", true},
		{"non-numeric string fails but echoes original", "abc", 0, "abc", false},
		{"bool fails", true, 0, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, original, ok := byteValue(tt.in)
			if num != tt.wantNum || original != tt.wantOriginal || ok != tt.wantOK {
				t.Errorf("byteValue(%#v) = (%v, %q, %v), want (%v, %q, %v)",
					tt.in, num, original, ok, tt.wantNum, tt.wantOriginal, tt.wantOK)
			}
		})
	}
}

// numericIdentityValue decides whether a uid/gid is reported as an int or kept
// as a string. Numeric ids must become ints (Facter reports uid as a number);
// names and unparseable values stay strings.
func TestNumericIdentityValue(t *testing.T) {
	tests := []struct {
		in   string
		want any
	}{
		{"0", 0},
		{"1000", 1000},
		{"-1", -1},
		{"root", "root"},
		{"", ""},
		{"1.5", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := numericIdentityValue(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("numericIdentityValue(%q) = %#v (%T), want %#v (%T)", tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}

// parseLinuxMountEntries parses /proc/self/mounts. The behaviors that matter:
// lines with fewer than 4 fields are dropped, options are comma-split, and
// octal-escaped device/mount fields are unescaped — so a space in a mount path
// (\040) survives as a real space rather than corrupting the fact.
func TestParseLinuxMountEntries(t *testing.T) {
	input := "" +
		"/dev/sda1 / ext4 rw,relatime 0 0\n" +
		"/dev/sda2 /mnt/my\\040drive ext4 rw 0 0\n" +
		"tmpfs /run tmpfs rw,nosuid,nodev 0 0\n" +
		"garbage line\n" +
		"\n"

	want := []mountEntry{
		{Device: "/dev/sda1", Path: "/", Filesystem: "ext4", Options: []string{"rw", "relatime"}},
		{Device: "/dev/sda2", Path: "/mnt/my drive", Filesystem: "ext4", Options: []string{"rw"}},
		{Device: "tmpfs", Path: "/run", Filesystem: "tmpfs", Options: []string{"rw", "nosuid", "nodev"}},
	}

	if got := parseLinuxMountEntries(input); !reflect.DeepEqual(got, want) {
		t.Errorf("parseLinuxMountEntries() =\n%#v\nwant\n%#v", got, want)
	}
}

// unescapeMountField pins the octal escapes the kernel uses in /proc/*/mounts.
func TestUnescapeMountField(t *testing.T) {
	tests := []struct{ in, want string }{
		{`/plain/path`, "/plain/path"},
		{`/a\040b`, "/a b"},
		{`/a\011b`, "/a\tb"},
		{`/a\012b`, "/a\nb"},
		{`/a\134b`, `/a\b`},
		{`/a\040b\134c`, "/a b\\c"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := unescapeMountField(tt.in); got != tt.want {
				t.Errorf("unescapeMountField(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// normalizeDarwinMountOption maps macOS mount option names onto the Ruby Facter
// vocabulary. Unknown options must pass through unchanged.
func TestNormalizeDarwinMountOption(t *testing.T) {
	tests := []struct{ in, want string }{
		{"read-only", "readonly"},
		{"asynchronous", "async"},
		{"synchronous", "noasync"},
		{"quotas", "quota"},
		{"rootfs", "root"},
		{"defwrite", "deferwrites"},
		{"local", "local"},
		{"journaled", "journaled"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeDarwinMountOption(tt.in); got != tt.want {
				t.Errorf("normalizeDarwinMountOption(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// parseDFP512Stats parses `df -P` output measured in 512-byte blocks (OpenBSD).
// The math that matters: every block count is multiplied by 512, the header and
// placeholder ("-") rows are skipped, and the mountpoint is the last field.
func TestParseDFP512Stats(t *testing.T) {
	input := "" +
		"Filesystem 512-blocks Used Avail Capacity Mounted on\n" +
		"/dev/sd0a 2048 1024 1024 50% /\n" +
		"/dev/sd0d - - - - /skip\n" +
		"/dev/sd0e 200 50 150 25% /home\n"

	got := parseDFP512Stats(input)
	want := map[string]mountStat{
		"/":     {SizeBytes: 2048 * 512, UsedBytes: 1024 * 512, AvailableBytes: 1024 * 512},
		"/home": {SizeBytes: 200 * 512, UsedBytes: 50 * 512, AvailableBytes: 150 * 512},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDFP512Stats() = %#v, want %#v", got, want)
	}
	if _, ok := got["/skip"]; ok {
		t.Error("parseDFP512Stats() included a row with '-' placeholder columns")
	}
}

// parseXenDomains extracts guest domain names from `xl/xm list` output, skipping
// the header row and the host domain (Domain-0).
func TestParseXenDomains(t *testing.T) {
	input := "" +
		"Name                                        ID   Mem VCPUs\tState\tTime(s)\n" +
		"Domain-0                                     0  4096     4 r-----  100.0\n" +
		"guest-web                                    1  2048     2 -b----   10.0\n" +
		"guest-db                                     2  2048     2 -b----   20.0\n"

	got := parseXenDomains(input)
	want := []string{"guest-web", "guest-db"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseXenDomains() = %#v, want %#v", got, want)
	}
}

// parseXenDomains on empty output must return a non-nil empty slice (the caller
// treats it as "Xen present, no guests").
func TestParseXenDomainsEmpty(t *testing.T) {
	got := parseXenDomains("")
	if got == nil || len(got) != 0 {
		t.Errorf("parseXenDomains(%q) = %#v, want non-nil empty slice", "", got)
	}
}
