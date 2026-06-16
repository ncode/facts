package engine

import "testing"

// lspciHypervisor maps `lspci` output to a hypervisor name. Each case is a
// distinct detection rule; if a substring match regresses, the `virtual` fact
// silently misidentifies the host. One representative line per vendor, plus the
// "bare metal" case where nothing matches.
func TestLspciHypervisor(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"virtualbox", "00:02.0 VGA compatible controller: InnoTek Systemberatung GmbH VirtualBox Graphics Adapter", "virtualbox"},
		{"xen", "00:00.0 Unassigned class [ff80]: XenSource, Inc. Xen Platform Device", "xenhvm"},
		{"hyperv", "00:08.0 VGA compatible controller: Microsoft Corporation Hyper-V virtual VGA", "hyperv"},
		{"gce", "00:01.0 Unclassified device [00ff]: Class 8007: Google, Inc.", "gce"},
		{"vmware", "00:0f.0 VGA compatible controller: VMware SVGA II Adapter", "vmware"},
		{"vmware uppercase W", "00:0f.0 VGA compatible controller: VMWare SVGA II Adapter", "vmware"},
		{"parallels by vendor id", "00:02.0 VGA compatible controller [0300]: 1ab8:4005", "parallels"},
		{"parallels by name", "00:02.0 VGA compatible controller: Parallels Display Adapter", "parallels"},
		{"kvm via virtio", "00:03.0 Ethernet controller: Red Hat, Inc. Virtio network device", "kvm"},
		{"bare metal", "00:1f.2 SATA controller: Intel Corporation 8 Series SATA Controller", ""},
		{"empty output", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lspciHypervisor(tt.line); got != tt.want {
				t.Errorf("lspciHypervisor(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

// vserverVirtualization reads /proc/self/status. A context id of 0 means the
// host node; anything else means a guest. The field key may be either
// "s_context:" or "VxID:", and lines that are not exactly "key value" are
// ignored.
func TestVserverVirtualization(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"s_context host", "Name:\tsh\ns_context: 0\n", "vserver_host"},
		{"s_context guest", "s_context: 49\n", "vserver"},
		{"VxID host", "VxID: 0\n", "vserver_host"},
		{"VxID guest", "VxID: 1\n", "vserver"},
		{"unrelated fields ignored", "Pid:\t1\nUid:\t0\t0\n", ""},
		{"malformed key-value with extra field ignored", "s_context: 0 extra\n", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vserverVirtualization(tt.in); got != tt.want {
				t.Errorf("vserverVirtualization(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// lastCGroupPathSegment extracts the final path component from a /proc/1/cgroup
// line, used to recover a container id. It must split on the ":/" delimiter and
// return the deepest segment.
func TestLastCGroupPathSegment(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"docker container id", "12:devices:/docker/abc123def456", "abc123def456"},
		{"cgroup v2 unified", "0::/system.slice/docker-abc.scope", "docker-abc.scope"},
		{"root cgroup yields empty segment", "0::/", ""},
		{"no delimiter", "garbage without delimiter", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lastCGroupPathSegment(tt.in); got != tt.want {
				t.Errorf("lastCGroupPathSegment(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
