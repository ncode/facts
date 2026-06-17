package engine

import (
	"reflect"
	"testing"
)

// TestFilesystemCapacityUsesAvailableDenominator covers the df/Facter capacity
// definition used/(used+available): a reserved-block filesystem reports the df
// percentage, a fully used read-only mount (available == 0) reports 100%, and an
// empty or unknown mount reports 0%. (tasks 1.2, 2.2)
func TestFilesystemCapacityUsesAvailableDenominator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		used      int
		available int
		want      string
	}{
		// Root fs with reserved blocks: used 6.75% of size but 7.26% of
		// (used+available), matching df/Facter.
		{name: "reserved blocks match df", used: 7_260, available: 92_740, want: "7.26%"},
		// Full read-only mount: bavail == 0 must read 100%, not 0%.
		{name: "full read-only mount is 100%", used: 12_345, available: 0, want: "100%"},
		// A mount with available space but nothing used is 0%.
		{name: "empty is 0%", used: 0, available: 100, want: "0%"},
		// Zero-size special filesystems (used == available == 0, e.g. /dev/pts,
		// hugepages) report 100% in Facter — available == 0 means full.
		{name: "zero-size fs is 100%", used: 0, available: 0, want: "100%"},
		// Simple half-full mount with no reserved blocks.
		{name: "half full", used: 50, available: 50, want: "50.00%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := filesystemCapacity(tt.used, tt.available); got != tt.want {
				t.Fatalf("filesystemCapacity(%d, %d) = %q, want %q", tt.used, tt.available, got, tt.want)
			}
		})
	}
}

// TestMountpointsFactCapacityUsesAvailableNotSize confirms the shared mountpoints
// builder now derives capacity from used/(used+available), so reserved-block
// filesystems report the df percentage rather than used/size. used_bytes itself
// is unchanged. (tasks 2.2, 2.3)
func TestMountpointsFactCapacityUsesAvailableNotSize(t *testing.T) {
	t.Parallel()

	// size 100_000, used 7_260, free (incl reserved) 92_740, but only 90_000
	// available to an unprivileged writer -> df reports 7260/(7260+90000)=7.47%.
	stats := func(string) (mountStat, bool) {
		return mountStat{SizeBytes: 100_000, AvailableBytes: 90_000, UsedBytes: 7_260}, true
	}

	got := mountpointsFact([]mountEntry{{Path: "/"}}, stats)
	mountpoint := got["/"].(map[string]any)
	if got, want := mountpoint["capacity"], "7.46%"; got != want {
		t.Fatalf("capacity = %#v, want %#v (used/(used+available), not used/size)", got, want)
	}
	if got, want := mountpoint["used_bytes"], 7_260; got != want {
		t.Fatalf("used_bytes = %#v, want %#v (unchanged)", got, want)
	}
}

// TestCurrentNetworkingDataExpandsLinuxInterfaceBindings asserts the Linux branch
// flattens the first IPv4 binding into ip/netmask/network and the first IPv6
// binding into ip6/netmask6/network6/scope6, matching Facter and the other POSIX
// platforms. (task 3.1)
func TestCurrentNetworkingDataExpandsLinuxInterfaceBindings(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"eth0": map[string]any{
			"mtu": 1500,
			"mac": "52:54:00:12:34:56",
			"bindings": []any{
				map[string]any{"address": "10.0.2.15", "netmask": "255.255.255.0", "network": "10.0.2.0"},
			},
			"bindings6": []any{
				map[string]any{"address": "fe80::5054:ff:fe12:3456", "netmask": "ffff:ffff:ffff:ffff::", "network": "fe80::", "scope6": "link"},
			},
		},
	}

	_, got := currentNetworkingData("linux", interfaces, func(string, ...string) string { return "" })

	eth0 := got["eth0"].(map[string]any)
	want := map[string]any{
		"ip":       "10.0.2.15",
		"netmask":  "255.255.255.0",
		"network":  "10.0.2.0",
		"ip6":      "fe80::5054:ff:fe12:3456",
		"netmask6": "ffff:ffff:ffff:ffff::",
		"network6": "fe80::",
		"scope6":   "link",
	}
	for key, wantValue := range want {
		if eth0[key] != wantValue {
			t.Fatalf("eth0[%s] = %#v, want %#v", key, eth0[key], wantValue)
		}
	}
}

// TestCurrentNetworkingDataLinuxOmitsSummaryKeysWithoutBindings asserts an
// interface with no usable bindings gains no empty ip/netmask/network/scope6
// keys, preserving the not-applicable-facts-are-omitted rule. (task 3.2)
func TestCurrentNetworkingDataLinuxOmitsSummaryKeysWithoutBindings(t *testing.T) {
	t.Parallel()

	interfaces := map[string]any{
		"eth0": map[string]any{
			"mtu": 1500,
			"mac": "52:54:00:12:34:56",
		},
	}

	_, got := currentNetworkingData("linux", interfaces, func(string, ...string) string { return "" })

	eth0 := got["eth0"].(map[string]any)
	for _, key := range []string{"ip", "netmask", "network", "ip6", "netmask6", "network6", "scope6"} {
		if _, present := eth0[key]; present {
			t.Fatalf("eth0[%s] present = %#v, want absent", key, eth0[key])
		}
	}
	// Existing keys are preserved.
	want := map[string]any{"mtu": 1500, "mac": "52:54:00:12:34:56"}
	if !reflect.DeepEqual(eth0, want) {
		t.Fatalf("eth0 = %#v, want %#v", eth0, want)
	}
}
