package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestDetectLinuxVirtualization_detectsContainerMarkers(t *testing.T) {
	tests := []struct {
		name  string
		input linuxVirtualizationInput
		want  virtualization
	}{
		{
			name:  "docker environment file",
			input: linuxVirtualizationInput{DockerEnv: true},
			want:  virtualization{Name: "docker", IsVirtual: true},
		},
		{
			name:  "podman environment file",
			input: linuxVirtualizationInput{ContainerEnv: true},
			want:  virtualization{Name: "podman", IsVirtual: true},
		},
		{
			name:  "docker cgroup",
			input: linuxVirtualizationInput{CGroup: "0::/docker/abcdef\n"},
			want:  virtualization{Name: "docker", IsVirtual: true},
		},
		{
			name:  "kubernetes cgroup",
			input: linuxVirtualizationInput{CGroup: "0::/kubepods.slice/pod123\n"},
			want:  virtualization{Name: "kubernetes", IsVirtual: true},
		},
		{
			name:  "lxc cgroup",
			input: linuxVirtualizationInput{CGroup: "11:name=systemd:/lxc/test\n"},
			want:  virtualization{Name: "lxc", IsVirtual: true},
		},
		{
			name:  "systemd-nspawn environment",
			input: linuxVirtualizationInput{ContainerRuntime: "systemd-nspawn"},
			want:  virtualization{Name: "systemd_nspawn", IsVirtual: true},
		},
		{
			name:  "lxc environment",
			input: linuxVirtualizationInput{ContainerRuntime: "lxc"},
			want:  virtualization{Name: "lxc", IsVirtual: true},
		},
		{
			name:  "lxc-virtwhat environment",
			input: linuxVirtualizationInput{ContainerRuntime: "lxc-virtwhat"},
			want:  virtualization{Name: "lxc-virtwhat", IsVirtual: true},
		},
		{
			name:  "unknown container environment",
			input: linuxVirtualizationInput{ContainerRuntime: "UNKNOWN"},
			want:  virtualization{Name: "physical", IsVirtual: false},
		},
		{
			name:  "physical host",
			input: linuxVirtualizationInput{},
			want:  virtualization{Name: "physical", IsVirtual: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectLinuxVirtualization(tt.input)
			if got != tt.want {
				t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDetectLinuxVirtualization_detectsOpenVZ(t *testing.T) {
	tests := []struct {
		name  string
		input linuxVirtualizationInput
		want  virtualization
	}{
		{
			name: "OpenVZ host",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				ProcVZEntries: 3,
				ProcStatus:    "envID: 0\n",
			},
			want: virtualization{Name: "openvzhn", IsVirtual: true},
		},
		{
			name: "OpenVZ container",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				ProcVZEntries: 3,
				ProcStatus:    "envID: 101\n",
			},
			want: virtualization{Name: "openvzve", IsVirtual: true},
		},
		{
			name: "CloudLinux LVE is not OpenVZ",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				LVEList:       true,
				ProcVZEntries: 3,
				ProcStatus:    "envID: 101\n",
			},
			want: virtualization{Name: "physical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectLinuxVirtualization(tt.input)
			if got != tt.want {
				t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestOpenVZEnvIDRequiresUsableStatusAndHostSignals(t *testing.T) {
	tests := []struct {
		name  string
		input linuxVirtualizationInput
		want  int
		ok    bool
	}{
		{
			name: "OpenVZ host envID",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				ProcVZEntries: 3,
				ProcStatus:    "Name:\tcat\nenvID:\t0\n",
			},
			ok: true,
		},
		{
			name: "OpenVZ container envID",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				ProcVZEntries: 3,
				ProcStatus:    "envID: 101\n",
			},
			want: 101,
			ok:   true,
		},
		{
			name: "missing proc vz",
			input: linuxVirtualizationInput{
				ProcVZEntries: 3,
				ProcStatus:    "envID: 101\n",
			},
		},
		{
			name: "CloudLinux LVE marker",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				LVEList:       true,
				ProcVZEntries: 3,
				ProcStatus:    "envID: 101\n",
			},
		},
		{
			name: "not enough proc vz entries",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				ProcVZEntries: 2,
				ProcStatus:    "envID: 101\n",
			},
		},
		{
			name: "invalid envID",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				ProcVZEntries: 3,
				ProcStatus:    "envID: not-a-number\n",
			},
		},
		{
			name: "missing envID",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				ProcVZEntries: 3,
				ProcStatus:    "Name:\tcat\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := openVZEnvID(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("openVZEnvID() = %d, %v; want %d, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestDetectLinuxVirtualization_detectsKVMFromDMI(t *testing.T) {
	tests := []struct {
		name  string
		input linuxVirtualizationInput
		want  virtualization
	}{
		{
			name: "qemu system vendor",
			input: linuxVirtualizationInput{
				DMISysVendor:   "QEMU",
				DMIProductName: "Standard PC (i440FX + PIIX, 1996)",
			},
			want: virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name: "seabios vendor",
			input: linuxVirtualizationInput{
				DMIBIOSVendor:  "SeaBIOS",
				DMIProductName: "Standard PC (i440FX + PIIX, 1996)",
			},
			want: virtualization{Name: "kvm", IsVirtual: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectLinuxVirtualization(tt.input)
			if got != tt.want {
				t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDetectLinuxVirtualization_detectsDMIProductHypervisors(t *testing.T) {
	got := detectLinuxVirtualization(linuxVirtualizationInput{DMIProductName: "Bochs Machine"})
	want := virtualization{Name: "bochs", IsVirtual: true}
	if got != want {
		t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, want)
	}
}

func TestDetectLinuxVirtualization_detectsGoogleComputeEngine(t *testing.T) {
	got := detectLinuxVirtualization(linuxVirtualizationInput{DMIBIOSVendor: "Google Engine"})
	want := virtualization{Name: "gce", IsVirtual: true}
	if got != want {
		t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, want)
	}
}

func TestDetectLinuxVirtualization_detectsVMwareCommandLikeRubyResolver(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   virtualization
	}{
		{
			name:   "valid two field VMware command output",
			output: "VmWare Fusion",
			want:   virtualization{Name: "vmware_fusion", IsVirtual: true},
		},
		{
			name:   "invalid version-bearing output",
			output: "vmware fusion 7.1",
			want:   virtualization{Name: "physical"},
		},
		{
			name: "missing command output",
			want: virtualization{Name: "physical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectLinuxVirtualization(linuxVirtualizationInput{VMwareCommand: tt.output})
			if got != tt.want {
				t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDetectLinuxVirtualization_detectsIllumosLX(t *testing.T) {
	got := detectLinuxVirtualization(linuxVirtualizationInput{
		ContainerRuntime: "lxc",
		KernelVersion:    "BrandZ virtual linux",
	})
	want := virtualization{Name: "illumos-lx", IsVirtual: true}
	if got != want {
		t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, want)
	}
}

func TestDetectLinuxVirtualizationDetectsVirtWhatKVMWithRedHatNoiseLikeRubyResolver(t *testing.T) {
	got := detectLinuxVirtualization(linuxVirtualizationInput{VirtWhatOutput: "redhat\nkvm"})
	want := virtualization{Name: "kvm", IsVirtual: true}
	if got != want {
		t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, want)
	}
}

func TestDetectLinuxVirtualizationDetectsVirtWhatXenHVMLikeRubyResolver(t *testing.T) {
	got := detectLinuxVirtualization(linuxVirtualizationInput{VirtWhatOutput: "xen\nxen-hvm"})
	want := virtualization{Name: "xenhvm", IsVirtual: true}
	if got != want {
		t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, want)
	}
}

func TestDetectLinuxVirtualizationDetectsVirtWhatVServerHostLikeRubyResolver(t *testing.T) {
	got := detectLinuxVirtualization(linuxVirtualizationInput{
		VirtWhatOutput: "linux_vserver",
		ProcStatus:     "Name:\tcat\nVxID: 0\n",
	})
	want := virtualization{Name: "vserver_host", IsVirtual: true}
	if got != want {
		t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, want)
	}
}

func TestDetectLinuxVirtualization_detectsLspciHypervisors(t *testing.T) {
	tests := []struct {
		name  string
		input linuxVirtualizationInput
		want  virtualization
	}{
		{
			name: "VMware PCI device",
			input: linuxVirtualizationInput{
				LspciOutput: "00:0f.0 VGA compatible controller: VMware SVGA II Adapter\n",
			},
			want: virtualization{Name: "vmware", IsVirtual: true},
		},
		{
			name: "Xen PCI platform device",
			input: linuxVirtualizationInput{
				LspciOutput: "00:03.0 Non-VGA unclassified device: XenSource, Inc. Xen Platform Device\n",
			},
			want: virtualization{Name: "xenhvm", IsVirtual: true},
		},
		{
			name:  "no hypervisor PCI device",
			input: linuxVirtualizationInput{LspciOutput: "lspci output with no hypervisor"},
			want:  virtualization{Name: "physical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectLinuxVirtualization(tt.input)
			if got != tt.want {
				t.Fatalf("detectLinuxVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDetectFreeBSDVirtualization(t *testing.T) {
	tests := []struct {
		name  string
		input freeBSDVirtualizationInput
		want  virtualization
	}{
		{
			name:  "bare metal",
			input: freeBSDVirtualizationInput{},
			want:  virtualization{Name: "physical"},
		},
		{
			name:  "jail takes precedence over guest VM",
			input: freeBSDVirtualizationInput{Jailed: true, VMGuest: "xen"},
			want:  virtualization{Name: "jail", IsVirtual: true},
		},
		{
			name:  "xen vm guest maps to xenu",
			input: freeBSDVirtualizationInput{VMGuest: "xen"},
			want:  virtualization{Name: "xenu", IsVirtual: true},
		},
		{
			name:  "non-xen vm guest",
			input: freeBSDVirtualizationInput{VMGuest: "bhyve"},
			want:  virtualization{Name: "bhyve", IsVirtual: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectFreeBSDVirtualization(tt.input)
			if got != tt.want {
				t.Fatalf("detectFreeBSDVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDetectVirtualizationUsesSessionPlatform(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "freebsd",
		runOutputs: map[string]string{
			fakeRunKey("sysctl", "-n", "security.jail.jailed"): "1\n",
			fakeRunKey("sysctl", "-n", "kern.vm_guest"):        "xen\n",
			fakeRunKey("uname", "-r"):                          "",
			fakeRunKey("dmidecode"):                            "",
			fakeRunKey("virt-what"):                            "",
			fakeRunKey("vmware", "-v"):                         "",
			fakeRunKey("lspci"):                                "",
		},
	}

	got := detectVirtualization(s)
	want := virtualization{Name: "jail", IsVirtual: true}
	if got != want {
		t.Fatalf("detectVirtualization() = %#v, want %#v", got, want)
	}
}

func TestDetectVirtualizationDispatchesSessionPlatform(t *testing.T) {
	tests := []struct {
		name string
		host *fakeHostOS
		want virtualization
	}{
		{
			name: "linux physical",
			host: &fakeHostOS{platform: "linux", runOutputs: map[string]string{
				fakeRunKey("uname", "-r"):  "",
				fakeRunKey("dmidecode"):    "",
				fakeRunKey("virt-what"):    "",
				fakeRunKey("vmware", "-v"): "",
				fakeRunKey("lspci"):        "",
			}},
			want: virtualization{Name: "physical"},
		},
		{
			name: "freebsd jail",
			host: &fakeHostOS{platform: "freebsd", runOutputs: map[string]string{
				fakeRunKey("sysctl", "-n", "security.jail.jailed"): "1\n",
				fakeRunKey("sysctl", "-n", "kern.vm_guest"):        "none\n",
			}},
			want: virtualization{Name: "jail", IsVirtual: true},
		},
		{
			name: "openbsd vmm",
			host: &fakeHostOS{platform: "openbsd", runOutputs: map[string]string{
				fakeRunKey("sysctl", "-n", "hw.product"): "VMM\n",
				fakeRunKey("sysctl", "-n", "hw.vendor"):  "OpenBSD\n",
			}},
			want: virtualization{Name: "vmm", IsVirtual: true},
		},
		{
			name: "netbsd kvm",
			host: &fakeHostOS{platform: "netbsd", runOutputs: map[string]string{
				fakeRunKey("/sbin/sysctl", "-n", "machdep.dmi.system-vendor"):  "QEMU\n",
				fakeRunKey("/sbin/sysctl", "-n", "machdep.dmi.system-product"): "Standard PC\n",
			}},
			want: virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name: "dragonfly kvm",
			host: &fakeHostOS{platform: "dragonfly", runOutputs: map[string]string{
				fakeRunKey("/usr/local/sbin/dmidecode", "-t", "system"): "Manufacturer: QEMU\nProduct Name: Standard PC\n",
				fakeRunKey("/usr/local/sbin/dmidecode", "-t", "bios"):   "Vendor: SeaBIOS\n",
				fakeRunKey("kenv", "smbios.system.maker"):               "",
				fakeRunKey("kenv", "smbios.system.product"):             "",
				fakeRunKey("kenv", "smbios.bios.vendor"):                "",
				fakeRunKey("pciconf", "-lv"):                            "",
			}},
			want: virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name: "illumos vmware",
			host: &fakeHostOS{platform: "illumos", runOutputs: map[string]string{
				fakeRunKey("/usr/sbin/smbios", "-t", "SMB_TYPE_SYSTEM"): "Manufacturer: VMware, Inc.\nProduct: VMware Virtual Platform\n",
				fakeRunKey("/usr/sbin/smbios", "-t", "SMB_TYPE_BIOS"):   "",
				fakeRunKey("/usr/sbin/prtconf", "-pv"):                  "",
			}},
			want: virtualization{Name: "vmware", IsVirtual: true},
		},
		{
			name: "windows unknown",
			host: &fakeHostOS{platform: "windows", runOutputs: map[string]string{
				fakeRunKey("wmic", "computersystem", "get", "Manufacturer,Model,OEMStringArray", "/value"): "",
				fakeRunKey("wmic", "bios", "get", "Manufacturer", "/value"):                                "",
				fakeRunKey("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Services`):                       "",
			}},
			want: virtualization{Unknown: true},
		},
		{
			name: "plan9 unknown",
			host: &fakeHostOS{platform: "plan9"},
			want: virtualization{Unknown: true},
		},
		{
			name: "unsupported physical",
			host: &fakeHostOS{platform: "hurd"},
			want: virtualization{Name: "physical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSessionContext(context.Background())
			s.host = tt.host
			if got := detectVirtualization(s); got != tt.want {
				t.Fatalf("detectVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCurrentBSDVirtualizationInputsQueryPlatformSources(t *testing.T) {
	t.Parallel()

	t.Run("freebsd", func(t *testing.T) {
		t.Parallel()

		got := currentFreeBSDVirtualizationInput(func(name string, args ...string) string {
			switch fakeRunKey(name, args...) {
			case fakeRunKey("sysctl", "-n", "security.jail.jailed"):
				return "1\n"
			case fakeRunKey("sysctl", "-n", "kern.vm_guest"):
				return "bhyve\n"
			default:
				t.Fatalf("unexpected command %q %#v", name, args)
				return ""
			}
		})
		want := freeBSDVirtualizationInput{Jailed: true, VMGuest: "bhyve"}
		if got != want {
			t.Fatalf("currentFreeBSDVirtualizationInput() = %#v, want %#v", got, want)
		}
	})

	t.Run("openbsd", func(t *testing.T) {
		t.Parallel()

		got := currentOpenBSDVirtualizationInput(func(name string, args ...string) string {
			switch fakeRunKey(name, args...) {
			case fakeRunKey("sysctl", "-n", "hw.product"):
				return "VMM\n"
			case fakeRunKey("sysctl", "-n", "hw.vendor"):
				return "OpenBSD\n"
			default:
				t.Fatalf("unexpected command %q %#v", name, args)
				return ""
			}
		})
		want := openBSDVirtualizationInput{ProductName: "VMM", Vendor: "OpenBSD"}
		if got != want {
			t.Fatalf("currentOpenBSDVirtualizationInput() = %#v, want %#v", got, want)
		}
	})

	t.Run("netbsd", func(t *testing.T) {
		t.Parallel()

		got := currentNetBSDVirtualizationInput(func(name string, args ...string) string {
			switch fakeRunKey(name, args...) {
			case fakeRunKey("/sbin/sysctl", "-n", "machdep.dmi.system-vendor"):
				return "QEMU\n"
			case fakeRunKey("/sbin/sysctl", "-n", "machdep.dmi.system-product"):
				return "Standard PC\n"
			default:
				t.Fatalf("unexpected command %q %#v", name, args)
				return ""
			}
		})
		want := dmiVirtualizationInput{Manufacturer: "QEMU", ProductName: "Standard PC"}
		if got != want {
			t.Fatalf("currentNetBSDVirtualizationInput() = %#v, want %#v", got, want)
		}
	})
}

func TestCurrentDMIPlatformVirtualizationInputsParseCommandOutput(t *testing.T) {
	t.Parallel()

	t.Run("dragonfly", func(t *testing.T) {
		t.Parallel()

		calls := map[string]bool{}
		got := currentDragonFlyVirtualizationInput(func(name string, args ...string) string {
			key := fakeRunKey(name, args...)
			calls[key] = true
			switch key {
			case fakeRunKey("/usr/local/sbin/dmidecode", "-t", "system"):
				return "Manufacturer: QEMU\nProduct Name: Standard PC\n"
			case fakeRunKey("/usr/local/sbin/dmidecode", "-t", "bios"):
				return "Vendor: SeaBIOS\n"
			case fakeRunKey("kenv", "smbios.system.maker"), fakeRunKey("kenv", "smbios.system.product"), fakeRunKey("kenv", "smbios.bios.vendor"):
				return ""
			case fakeRunKey("pciconf", "-lv"):
				return "virtio_pci0@pci0:0:4:0\n"
			default:
				t.Fatalf("unexpected command %q %#v", name, args)
				return ""
			}
		})
		want := dmiVirtualizationInput{
			Manufacturer: "QEMU",
			ProductName:  "Standard PC",
			BIOSVendor:   "SeaBIOS",
			PCIOutput:    "virtio_pci0@pci0:0:4:0\n",
		}
		if got != want {
			t.Fatalf("currentDragonFlyVirtualizationInput() = %#v, want %#v", got, want)
		}
		for _, key := range []string{
			fakeRunKey("kenv", "smbios.system.maker"),
			fakeRunKey("kenv", "smbios.system.product"),
			fakeRunKey("kenv", "smbios.bios.vendor"),
		} {
			if !calls[key] {
				t.Fatalf("currentDragonFlyVirtualizationInput() did not call %q", key)
			}
		}
	})

	t.Run("illumos", func(t *testing.T) {
		t.Parallel()

		got := currentIllumosVirtualizationInput(func(name string, args ...string) string {
			switch fakeRunKey(name, args...) {
			case fakeRunKey("/usr/sbin/smbios", "-t", "SMB_TYPE_SYSTEM"):
				return "Manufacturer: QEMU\nProduct: Standard PC\n"
			case fakeRunKey("/usr/sbin/smbios", "-t", "SMB_TYPE_BIOS"):
				return "Vendor: SeaBIOS\n"
			case fakeRunKey("/usr/sbin/prtconf", "-pv"):
				return "pci15ad,1976\n"
			default:
				t.Fatalf("unexpected command %q %#v", name, args)
				return ""
			}
		})
		want := dmiVirtualizationInput{
			Manufacturer: "QEMU",
			ProductName:  "Standard PC",
			BIOSVendor:   "SeaBIOS",
			PCIOutput:    "pci15ad,1976\n",
		}
		if got != want {
			t.Fatalf("currentIllumosVirtualizationInput() = %#v, want %#v", got, want)
		}
	})
}

func TestCurrentLinuxVirtualizationInputReadsHostSignals(t *testing.T) {
	host := &fakeHostOS{
		runOutputs: map[string]string{
			fakeRunKey("uname", "-r"):  "6.10.0-test\n",
			fakeRunKey("dmidecode"):    "vboxVer_7.0.14\nvboxRev_161095\nAddress: 0xea580\n",
			fakeRunKey("virt-what"):    "kvm\n",
			fakeRunKey("vmware", "-v"): "VMware Fusion\n",
			fakeRunKey("lspci"):        "00:03.0 Ethernet controller: Red Hat, Inc. Virtio network device\n",
		},
		files: map[string][]byte{
			"/proc/1/cgroup":                 []byte("0::/docker/abcdef\n"),
			"/proc/self/status":              []byte("Name:\tcat\n"),
			"/proc/1/environ":                []byte("container=systemd-nspawn\x00PATH=/usr/bin"),
			"/etc/machine-id":                []byte("machine-id\n"),
			"/sys/class/dmi/id/bios_vendor":  []byte("SeaBIOS\n"),
			"/sys/class/dmi/id/product_name": []byte("Standard PC\n"),
			"/sys/class/dmi/id/sys_vendor":   []byte("QEMU\n"),
		},
		stats: map[string]os.FileInfo{
			"/.dockerenv":        fakeFileInfo{name: ".dockerenv"},
			"/run/.containerenv": fakeFileInfo{name: ".containerenv"},
			"/proc/vz":           fakeFileInfo{name: "vz", isDir: true},
			"/proc/lve/list":     fakeFileInfo{name: "list"},
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	got := currentLinuxVirtualizationInput(s)
	// ProcVZEntries is covered through procVZEntryCount below; this collector
	// currently probes that fixed path directly rather than through host IO.
	if got.CGroup != "0::/docker/abcdef\n" || !got.DockerEnv || !got.ContainerEnv || !got.ProcVZ || !got.LVEList {
		t.Fatalf("currentLinuxVirtualizationInput() host markers = %#v", got)
	}
	if got.ProcStatus != "Name:\tcat\n" || got.ContainerRuntime != "systemd-nspawn" || got.MachineID != "machine-id" {
		t.Fatalf("currentLinuxVirtualizationInput() process fields = %#v", got)
	}
	if runtime.GOOS != "plan9" && got.KernelVersion != "6.10.0-test" {
		t.Fatalf("currentLinuxVirtualizationInput() kernel version = %q, want 6.10.0-test", got.KernelVersion)
	}
	if got.DMIBIOSVendor != "SeaBIOS" || got.DMIProductName != "Standard PC" || got.DMISysVendor != "QEMU" {
		t.Fatalf("currentLinuxVirtualizationInput() DMI fields = %#v", got)
	}
	if got.DMIDecodeInfo.VirtualBoxVersion != "7.0.14" || got.DMIDecodeInfo.VirtualBoxRevision != "161095" || got.DMIDecodeInfo.VMwareVersion != "ESXi 6.5" {
		t.Fatalf("currentLinuxVirtualizationInput() dmidecode info = %#v", got.DMIDecodeInfo)
	}
	if got.VirtWhatOutput != "kvm\n" || got.VMwareCommand != "VMware Fusion\n" || got.LspciOutput != "00:03.0 Ethernet controller: Red Hat, Inc. Virtio network device\n" {
		t.Fatalf("currentLinuxVirtualizationInput() command fields = %#v", got)
	}
}

func TestProcVZEntryCountMatchesRubyResolverOffset(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "veinfo"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vestat"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := procVZEntryCount(dir); got != 4 {
		t.Fatalf("procVZEntryCount() = %d, want entries plus resolver offset 4", got)
	}
	emptyDir := t.TempDir()
	if got := procVZEntryCount(emptyDir); got != 2 {
		t.Fatalf("procVZEntryCount(empty) = %d, want resolver offset 2", got)
	}
	if got := procVZEntryCount(filepath.Join(dir, "missing")); got != 0 {
		t.Fatalf("procVZEntryCount(missing) = %d, want 0", got)
	}
}

func TestDetectWindowsVirtualizationMatchesRubyResolver(t *testing.T) {
	tests := []struct {
		name  string
		input windowsVirtualizationInput
		want  virtualization
	}{
		{
			name:  "VirtualBox model",
			input: windowsVirtualizationInput{Model: "VirtualBox"},
			want:  virtualization{Name: "virtualbox", IsVirtual: true},
		},
		{
			name:  "VMware model",
			input: windowsVirtualizationInput{Model: "VMware"},
			want:  virtualization{Name: "vmware", IsVirtual: true},
		},
		{
			name:  "KVM model",
			input: windowsVirtualizationInput{Model: "KVM10"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "OpenStack model",
			input: windowsVirtualizationInput{Model: "OpenStack"},
			want:  virtualization{Name: "openstack", IsVirtual: true},
		},
		{
			name:  "AHV model",
			input: windowsVirtualizationInput{Model: "AHV"},
			want:  virtualization{Name: "ahv", IsVirtual: true},
		},
		{
			name:  "Microsoft Virtual Machine",
			input: windowsVirtualizationInput{Manufacturer: "Microsoft", Model: "Virtual Machine"},
			want:  virtualization{Name: "hyperv", IsVirtual: true},
		},
		{
			name:  "Xen manufacturer",
			input: windowsVirtualizationInput{Manufacturer: "Xen"},
			want:  virtualization{Name: "xen", IsVirtual: true},
		},
		{
			name:  "Amazon EC2 manufacturer",
			input: windowsVirtualizationInput{Manufacturer: "Amazon EC2"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "NetKVM service",
			input: windowsVirtualizationInput{NetKVM: true},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "QEMU manufacturer",
			input: windowsVirtualizationInput{WMIResult: true, Manufacturer: "QEMU", Model: "Standard PC (i440FX + PIIX, 1996)"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "SeaBIOS manufacturer",
			input: windowsVirtualizationInput{WMIResult: true, BIOSManufacturer: "SeaBIOS"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "QEMU casing variant",
			input: windowsVirtualizationInput{WMIResult: true, Manufacturer: "Qemu", Model: "standard pc (i440fx + piix, 1996)"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "SeaBIOS casing variant",
			input: windowsVirtualizationInput{WMIResult: true, BIOSManufacturer: "seabios"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "Google NetKVM service",
			input: windowsVirtualizationInput{BIOSManufacturer: "Google", NetKVM: true},
			want:  virtualization{Name: "gce", IsVirtual: true},
		},
		{
			name:  "physical machine",
			input: windowsVirtualizationInput{WMIResult: true},
			want:  virtualization{Name: "physical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectWindowsVirtualization(tt.input)
			if got != tt.want {
				t.Fatalf("detectWindowsVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCurrentWindowsVirtualizationInputNoWMIResultMatchesRubyResolver(t *testing.T) {
	got := detectWindowsVirtualization(currentWindowsVirtualizationInput("windows", func(name string, args ...string) string {
		return ""
	}))
	want := virtualization{Unknown: true}
	if got != want {
		t.Fatalf("detectWindowsVirtualization(no WMI result) = %#v, want %#v", got, want)
	}
}

func TestVirtualizationFactValuesReturnsNilForUnknownWindowsResolver(t *testing.T) {
	gotVirtual, gotIsVirtual := virtualizationFactValues(virtualization{Unknown: true})
	if gotVirtual != nil || gotIsVirtual != nil {
		t.Fatalf("virtualizationFactValues(unknown) = %v, %v; want nil, nil", gotVirtual, gotIsVirtual)
	}
}

func TestVirtualizationFactValuesMatchLinuxVirtualFacts(t *testing.T) {
	tests := []struct {
		name          string
		virtual       virtualization
		wantVirtual   any
		wantIsVirtual any
	}{
		{
			name:          "physical",
			virtual:       virtualization{Name: "physical", IsVirtual: false},
			wantVirtual:   "physical",
			wantIsVirtual: false,
		},
		{
			name:          "virtual",
			virtual:       virtualization{Name: "aws", IsVirtual: true},
			wantVirtual:   "aws",
			wantIsVirtual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVirtual, gotIsVirtual := virtualizationFactValues(tt.virtual)
			if gotVirtual != tt.wantVirtual || gotIsVirtual != tt.wantIsVirtual {
				t.Fatalf("virtualizationFactValues() = %#v, %#v; want %#v, %#v", gotVirtual, gotIsVirtual, tt.wantVirtual, tt.wantIsVirtual)
			}
		})
	}
}

func TestCurrentWindowsVirtualizationInputQueriesWMIC(t *testing.T) {
	got := currentWindowsVirtualizationInput("windows", func(name string, args ...string) string {
		switch name {
		case "wmic":
			return "Manufacturer=Oracle Corporation\r\nModel=VirtualBox\r\nOEMStringArray=vboxVer_6.0.4\r\n\r\nManufacturer=Oracle Corporation\r\nModel=VirtualBox\r\nOEMStringArray=vboxRev_128413\r\n"
		case "reg":
			return "HKEY_LOCAL_MACHINE\\SYSTEM\\CurrentControlSet\\Services\r\n    NetKVM\r\n"
		default:
			t.Fatalf("unexpected command = %q", name)
		}
		return ""
	})
	want := windowsVirtualizationInput{
		Manufacturer: "Oracle Corporation",
		Model:        "VirtualBox",
		OEMStrings:   []string{"vboxVer_6.0.4", "vboxRev_128413"},
		NetKVM:       true,
	}
	if got.Manufacturer != want.Manufacturer || got.Model != want.Model || got.NetKVM != want.NetKVM || len(got.OEMStrings) != len(want.OEMStrings) {
		t.Fatalf("currentWindowsVirtualizationInput() = %#v, want %#v", got, want)
	}
	for i := range want.OEMStrings {
		if got.OEMStrings[i] != want.OEMStrings[i] {
			t.Fatalf("currentWindowsVirtualizationInput().OEMStrings = %#v, want %#v", got.OEMStrings, want.OEMStrings)
		}
	}
}

func TestCurrentWindowsVirtualizationInputSkipsNonWindows(t *testing.T) {
	got := currentWindowsVirtualizationInput("linux", func(string, ...string) string {
		t.Fatal("currentWindowsVirtualizationInput(non-windows) ran command")
		return ""
	})
	if got.Manufacturer != "" || got.Model != "" || len(got.OEMStrings) != 0 {
		t.Fatalf("currentWindowsVirtualizationInput(non-windows) = %#v, want empty", got)
	}
}

func TestWindowsHypervisorFactsMatchRubyFacts(t *testing.T) {
	tests := []struct {
		name  string
		input windowsVirtualizationInput
		want  map[string]any
	}{
		{
			name: "VirtualBox OEM metadata",
			input: windowsVirtualizationInput{
				Model:      "VirtualBox",
				OEMStrings: []string{"vboxVer_ 13.4", "vboxRev_ 13.4"},
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"virtualbox": map[string]any{"version": " 13.4", "revision": " 13.4"},
				},
			},
		},
		{
			name:  "OpenStack maps to KVM metadata",
			input: windowsVirtualizationInput{Model: "OpenStack"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"kvm": map[string]any{"openstack": true},
				},
			},
		},
		{
			name:  "Google NetKVM maps to KVM metadata",
			input: windowsVirtualizationInput{BIOSManufacturer: "Google", NetKVM: true},
			want: map[string]any{
				"hypervisors": map[string]any{
					"kvm": map[string]any{"google": true},
				},
			},
		},
		{
			name:  "Hyper-V metadata",
			input: windowsVirtualizationInput{Manufacturer: "Microsoft", Model: "Virtual Machine"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"hyperv": map[string]any{},
				},
			},
		},
		{
			name:  "Xen context from model",
			input: windowsVirtualizationInput{Manufacturer: "Xen", Model: "HVM domU"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"xen": map[string]any{"context": "hvm"},
				},
			},
		},
		{
			name:  "Xen paravirtualized context by default",
			input: windowsVirtualizationInput{Manufacturer: "Xen", Model: "PV domU"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"xen": map[string]any{"context": "pv"},
				},
			},
		},
		{
			name:  "physical host",
			input: windowsVirtualizationInput{},
			want:  map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Collection(windowsHypervisorFacts(tt.input))
			if !mapsEqual(got, tt.want) {
				t.Fatalf("windowsHypervisorFacts() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCurrentWindowsHypervisorFactsUsesSessionHost(t *testing.T) {
	host := &fakeHostOS{
		platform: "windows",
		runOutputs: map[string]string{
			fakeRunKey("wmic", "computersystem", "get", "Manufacturer,Model,OEMStringArray", "/value"):                            "Manufacturer=Microsoft Corporation\r\nModel=Virtual Machine\r\n",
			fakeRunKey("wmic", "bios", "get", "Manufacturer", "/value"):                                                           "",
			fakeRunKey("powershell", "-NoProfile", "-NonInteractive", "-Command", windowsCIMScript("Win32_BIOS", "Manufacturer")): "",
			fakeRunKey("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Services`):                                                  "",
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	got := Collection(currentWindowsHypervisorFacts(s))
	want := map[string]any{"hypervisors": map[string]any{"hyperv": map[string]any{}}}
	if !mapsEqual(got, want) {
		t.Fatalf("currentWindowsHypervisorFacts() = %#v, want %#v", got, want)
	}
}

func TestDetectOpenBSDVirtualization(t *testing.T) {
	tests := []struct {
		name  string
		input openBSDVirtualizationInput
		want  virtualization
	}{
		{
			name:  "bare metal",
			input: openBSDVirtualizationInput{},
			want:  virtualization{Name: "physical"},
		},
		{
			name:  "vmm product name",
			input: openBSDVirtualizationInput{ProductName: "VMM"},
			want:  virtualization{Name: "vmm", IsVirtual: true},
		},
		{
			name:  "qemu vendor",
			input: openBSDVirtualizationInput{Vendor: "QEMU", ProductName: "Standard PC (i440FX + PIIX, 1996)"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "known none product stays physical",
			input: openBSDVirtualizationInput{Vendor: "QEMU", ProductName: "none"},
			want:  virtualization{Name: "physical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectOpenBSDVirtualization(tt.input)
			if got != tt.want {
				t.Fatalf("detectOpenBSDVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDetectDMIHostVirtualization(t *testing.T) {
	tests := []struct {
		name  string
		input dmiVirtualizationInput
		want  virtualization
	}{
		{
			name:  "qemu manufacturer",
			input: dmiVirtualizationInput{Manufacturer: "QEMU", ProductName: "Standard PC (i440FX + PIIX, 1996)"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "qemu product",
			input: dmiVirtualizationInput{ProductName: "QEMU Virtual Machine"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "qemu manufacturer beats generic virtual machine product",
			input: dmiVirtualizationInput{Manufacturer: "QEMU", ProductName: "Virtual Machine"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "seabios",
			input: dmiVirtualizationInput{BIOSVendor: "SeaBIOS"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "seabios beats generic virtual machine product",
			input: dmiVirtualizationInput{ProductName: "Virtual Machine", BIOSVendor: "SeaBIOS"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "qemu casing variant",
			input: dmiVirtualizationInput{Manufacturer: "Qemu", ProductName: "standard pc (i440fx + piix, 1996)"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "seabios casing variant",
			input: dmiVirtualizationInput{BIOSVendor: "seabios"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "virtualbox casing variant",
			input: dmiVirtualizationInput{ProductName: "virtualbox"},
			want:  virtualization{Name: "virtualbox", IsVirtual: true},
		},
		{
			name:  "specific product beats seabios fallback",
			input: dmiVirtualizationInput{ProductName: "Bochs Machine", BIOSVendor: "SeaBIOS"},
			want:  virtualization{Name: "bochs", IsVirtual: true},
		},
		{
			name:  "virtio pci",
			input: dmiVirtualizationInput{PCIOutput: "00:03.0 Ethernet controller: Red Hat, Inc. Virtio network device\n"},
			want:  virtualization{Name: "kvm", IsVirtual: true},
		},
		{
			name:  "vmware pci",
			input: dmiVirtualizationInput{PCIOutput: "00:0f.0 VGA compatible controller: VMware SVGA II Adapter\n"},
			want:  virtualization{Name: "vmware", IsVirtual: true},
		},
		{
			name:  "physical host",
			input: dmiVirtualizationInput{Manufacturer: "Apple Inc.", ProductName: "Mac16,10"},
			want:  virtualization{Name: "physical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectDMIHostVirtualization(tt.input)
			if got != tt.want {
				t.Fatalf("detectDMIHostVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLinuxHypervisorFacts(t *testing.T) {
	tests := []struct {
		name  string
		input linuxVirtualizationInput
		want  map[string]any
	}{
		{
			name: "OpenVZ host",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				ProcVZEntries: 3,
				ProcStatus:    "envID: 0\n",
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"openvz": map[string]any{"id": 0, "host": true},
				},
			},
		},
		{
			name: "OpenVZ container",
			input: linuxVirtualizationInput{
				ProcVZ:        true,
				ProcVZEntries: 3,
				ProcStatus:    "envID: 101\n",
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"openvz": map[string]any{"id": 101, "host": false},
				},
			},
		},
		{
			name:  "docker cgroup",
			input: linuxVirtualizationInput{CGroup: "0::/docker/abcdef\n"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"docker": map[string]any{"id": "abcdef"},
				},
			},
		},
		{
			name:  "containerd cgroup",
			input: linuxVirtualizationInput{CGroup: "0::/system.slice/containerd.service/kubepods/burstable/pod123/0123456789abcdef\n"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"docker": map[string]any{"id": "0123456789abcdef"},
				},
			},
		},
		{
			name:  "lxc cgroup",
			input: linuxVirtualizationInput{CGroup: "11:name=systemd:/lxc/test_name\n"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"lxc": map[string]any{"name": "test_name"},
				},
			},
		},
		{
			name:  "systemd-nspawn with machine id",
			input: linuxVirtualizationInput{ContainerRuntime: "systemd-nspawn", MachineID: "testid00"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"systemd_nspawn": map[string]any{"id": "testid00"},
				},
			},
		},
		{
			name:  "systemd-nspawn without machine id",
			input: linuxVirtualizationInput{ContainerRuntime: "systemd-nspawn"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"systemd_nspawn": map[string]any{},
				},
			},
		},
		{
			name:  "lxc environment",
			input: linuxVirtualizationInput{ContainerRuntime: "lxc"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"lxc": map[string]any{},
				},
			},
		},
		{
			name:  "lxc-virtwhat environment",
			input: linuxVirtualizationInput{ContainerRuntime: "lxc-virtwhat"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"lxc-virtwhat": map[string]any{},
				},
			},
		},
		{
			name: "VirtualBox DMI product with dmidecode metadata",
			input: linuxVirtualizationInput{
				DMIProductName: "VirtualBox",
				DMIDecodeInfo: dmiDecodeHypervisorInfo{
					VirtualBoxVersion:  "6.1.4",
					VirtualBoxRevision: "136177",
				},
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"virtualbox": map[string]any{"version": "6.1.4", "revision": "136177"},
				},
			},
		},
		{
			name: "VirtualBox virt-what fallback",
			input: linuxVirtualizationInput{
				VirtWhatOutput: "virtualbox",
				DMIDecodeInfo: dmiDecodeHypervisorInfo{
					VirtualBoxVersion:  "6.1.4",
					VirtualBoxRevision: "136177",
				},
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"virtualbox": map[string]any{"version": "6.1.4", "revision": "136177"},
				},
			},
		},
		{
			name: "VMware DMI vendor with dmidecode metadata",
			input: linuxVirtualizationInput{
				DMISysVendor:  "VMware, Inc.",
				DMIDecodeInfo: dmiDecodeHypervisorInfo{VMwareVersion: "ESXi 6.7"},
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"vmware": map[string]any{"version": "ESXi 6.7"},
				},
			},
		},
		{
			name: "VMware virt-what fallback",
			input: linuxVirtualizationInput{
				VirtWhatOutput: "vmware",
				DMIDecodeInfo:  dmiDecodeHypervisorInfo{VMwareVersion: "ESXi 6.7"},
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"vmware": map[string]any{"version": "ESXi 6.7"},
				},
			},
		},
		{
			name: "VMware PCI fallback",
			input: linuxVirtualizationInput{
				LspciOutput: "00:0f.0 VGA compatible controller: VMware SVGA II Adapter\n",
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"vmware": map[string]any{},
				},
			},
		},
		{
			name: "VirtualBox PCI fallback",
			input: linuxVirtualizationInput{
				LspciOutput: "00:02.0 VGA compatible controller: InnoTek Systemberatung GmbH VirtualBox Graphics Adapter\n",
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"virtualbox": map[string]any{},
				},
			},
		},
		{
			name: "KVM PCI fallback",
			input: linuxVirtualizationInput{
				LspciOutput: "00:04.0 Ethernet controller: Red Hat, Inc. Virtio network device\n",
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"kvm": map[string]any{},
				},
			},
		},
		{
			name:  "Xen HVM virt-what fallback",
			input: linuxVirtualizationInput{VirtWhatOutput: "xen\nxen-hvm"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"xen": map[string]any{"context": "hvm", "privileged": false},
				},
			},
		},
		{
			name:  "Xen dom0 virt-what fallback",
			input: linuxVirtualizationInput{VirtWhatOutput: "xen\nxen-dom0"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"xen": map[string]any{"context": "pv", "privileged": true},
				},
			},
		},
		{
			name:  "KVM virt-what fallback",
			input: linuxVirtualizationInput{VirtWhatOutput: "kvm"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"kvm": map[string]any{},
				},
			},
		},
		{
			name: "Xen PCI fallback",
			input: linuxVirtualizationInput{
				LspciOutput: "00:03.0 Non-VGA unclassified device: XenSource, Inc. Xen Platform Device\n",
			},
			want: map[string]any{
				"hypervisors": map[string]any{
					"xen": map[string]any{},
				},
			},
		},
		{
			name:  "Hyper-V DMI sys vendor",
			input: linuxVirtualizationInput{DMISysVendor: "Microsoft Corporation"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"hyperv": map[string]any{},
				},
			},
		},
		{
			name:  "Hyper-V DMI product name",
			input: linuxVirtualizationInput{DMIProductName: "Virtual Machine"},
			want: map[string]any{
				"hypervisors": map[string]any{
					"hyperv": map[string]any{},
				},
			},
		},
		{
			name:  "physical host",
			input: linuxVirtualizationInput{},
			want:  map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Collection(linuxHypervisorFacts(tt.input))
			if !mapsEqual(got, tt.want) {
				t.Fatalf("linuxHypervisorFacts() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLinuxHypervisorFactsReturnsNilOpenVZFactWhenAbsent(t *testing.T) {
	got := linuxHypervisorFacts(linuxVirtualizationInput{})
	want := []ResolvedFact{
		{Name: "hypervisors.openvz", Value: nil},
		{Name: "hypervisors.docker", Value: nil},
		{Name: "hypervisors.lxc", Value: nil},
		{Name: "hypervisors.systemd_nspawn", Value: nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linuxHypervisorFacts() = %#v, want %#v", got, want)
	}
}

func TestCurrentLinuxHypervisorFactsUsesSessionHost(t *testing.T) {
	host := &fakeHostOS{
		platform: "linux",
		stats: map[string]os.FileInfo{
			"/.dockerenv": fakeFileInfo{name: ".dockerenv"},
		},
		runOutputs: map[string]string{
			fakeRunKey("uname", "-r"):  "",
			fakeRunKey("dmidecode"):    "",
			fakeRunKey("virt-what"):    "",
			fakeRunKey("vmware", "-v"): "",
			fakeRunKey("lspci"):        "",
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	got := currentLinuxHypervisorFacts(s)
	want := []ResolvedFact{
		{Name: "hypervisors.docker", Value: map[string]any{}},
		{Name: "hypervisors.lxc", Value: nil},
		{Name: "hypervisors.systemd_nspawn", Value: nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxHypervisorFacts() = %#v, want %#v", got, want)
	}
}

func TestLinuxHypervisorFactsReturnsNilDockerFactWhenDockerAbsent(t *testing.T) {
	tests := []struct {
		name  string
		input linuxVirtualizationInput
		want  []ResolvedFact
	}{
		{
			name:  "lxc detected",
			input: linuxVirtualizationInput{CGroup: "11:name=systemd:/lxc/test_name\n"},
			want: []ResolvedFact{
				{Name: "hypervisors.lxc", Value: map[string]any{"name": "test_name"}},
				{Name: "hypervisors.docker", Value: nil},
			},
		},
		{
			name:  "no container resolver result",
			input: linuxVirtualizationInput{},
			want: []ResolvedFact{
				{Name: "hypervisors.openvz", Value: nil},
				{Name: "hypervisors.docker", Value: nil},
				{Name: "hypervisors.lxc", Value: nil},
				{Name: "hypervisors.systemd_nspawn", Value: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linuxHypervisorFacts(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("linuxHypervisorFacts() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLinuxHypervisorFactsReturnsNilContainerFactsWhenAbsent(t *testing.T) {
	tests := []struct {
		name  string
		input linuxVirtualizationInput
		want  []ResolvedFact
	}{
		{
			name:  "docker detected",
			input: linuxVirtualizationInput{DockerEnv: true},
			want: []ResolvedFact{
				{Name: "hypervisors.docker", Value: map[string]any{}},
				{Name: "hypervisors.lxc", Value: nil},
				{Name: "hypervisors.systemd_nspawn", Value: nil},
			},
		},
		{
			name:  "no container resolver result",
			input: linuxVirtualizationInput{},
			want: []ResolvedFact{
				{Name: "hypervisors.openvz", Value: nil},
				{Name: "hypervisors.docker", Value: nil},
				{Name: "hypervisors.lxc", Value: nil},
				{Name: "hypervisors.systemd_nspawn", Value: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linuxHypervisorFacts(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("linuxHypervisorFacts() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCurrentLinuxVirtualizationInputReadsDMIDecodeMetadata(t *testing.T) {
	got := currentLinuxVirtualizationInputWithCommands(testSession, func(name string, args ...string) string {
		switch name {
		case "dmidecode":
			return `BIOS Information
	Vendor: innotek GmbH
	Version: VirtualBox
	Address: 0xE0000

OEM Strings
	String 1: vboxVer_6.1.4
	String 2: vboxRev_136177
`
		case "lspci":
			return ""
		case "virt-what":
			return "kvm"
		case "vmware":
			if len(args) != 1 || args[0] != "-v" {
				t.Fatalf("vmware args = %#v, want [-v]", args)
			}
			return ""
		default:
			t.Fatalf("unexpected command = %q", name)
			return ""
		}
	})

	want := dmiDecodeHypervisorInfo{VirtualBoxVersion: "6.1.4", VirtualBoxRevision: "136177"}
	if got.DMIDecodeInfo != want {
		t.Fatalf("currentLinuxVirtualizationInput(testSession).DMIDecodeInfo = %#v, want %#v", got.DMIDecodeInfo, want)
	}
	if got.VMwareCommand != "" {
		t.Fatalf("currentLinuxVirtualizationInput(testSession).VMwareCommand = %q, want empty", got.VMwareCommand)
	}
	if got.VirtWhatOutput != "kvm" {
		t.Fatalf("currentLinuxVirtualizationInput(testSession).VirtWhatOutput = %q, want kvm", got.VirtWhatOutput)
	}
}

func TestParseDMIDecodeHypervisorInfo(t *testing.T) {
	virtualboxOutput := `BIOS Information
	Vendor: innotek GmbH
	Version: VirtualBox
	Address: 0xE0000

OEM Strings
	String 1: vboxVer_6.1.4
	String 2: vboxRev_136177
`
	got := parseDMIDecodeHypervisorInfo(virtualboxOutput)
	want := dmiDecodeHypervisorInfo{VirtualBoxVersion: "6.1.4", VirtualBoxRevision: "136177"}
	if got != want {
		t.Fatalf("parseDMIDecodeHypervisorInfo() = %#v, want %#v", got, want)
	}

	vmwareOutput := `BIOS Information
	Vendor: Phoenix Technologies LTD
	Version: 6.00
	Address: 0xEA490
`
	got = parseDMIDecodeHypervisorInfo(vmwareOutput)
	want = dmiDecodeHypervisorInfo{VMwareVersion: "ESXi 6.7"}
	if got != want {
		t.Fatalf("parseDMIDecodeHypervisorInfo() = %#v, want %#v", got, want)
	}
}

func TestDetectMacOSVirtualization(t *testing.T) {
	tests := []struct {
		name     string
		hardware macOSSystemProfilerHardware
		want     virtualization
	}{
		{
			name: "physical Mac",
			hardware: macOSSystemProfilerHardware{
				ModelIdentifier:   "MacBookPro11,4",
				BootROMVersion:    "1037.60.58.0.0 (iBridge: 17.16.12551.0.0,0)",
				SubsystemVendorID: "0x123",
			},
			want: virtualization{Name: "physical"},
		},
		{
			name:     "VMware model identifier",
			hardware: macOSSystemProfilerHardware{ModelIdentifier: "VMware"},
			want:     virtualization{Name: "vmware", IsVirtual: true},
		},
		{
			name:     "VMware model identifier must be prefix",
			hardware: macOSSystemProfilerHardware{ModelIdentifier: "MacVMware"},
			want:     virtualization{Name: "physical"},
		},
		{
			name:     "VirtualBox boot ROM",
			hardware: macOSSystemProfilerHardware{BootROMVersion: "VirtualBox"},
			want:     virtualization{Name: "virtualbox", IsVirtual: true},
		},
		{
			name:     "VirtualBox boot ROM must be prefix",
			hardware: macOSSystemProfilerHardware{BootROMVersion: "Apple VirtualBox"},
			want:     virtualization{Name: "physical"},
		},
		{
			name:     "Parallels subsystem vendor",
			hardware: macOSSystemProfilerHardware{SubsystemVendorID: "0x1ab8"},
			want:     virtualization{Name: "parallels", IsVirtual: true},
		},
		{
			name:     "Parallels subsystem vendor prefix",
			hardware: macOSSystemProfilerHardware{SubsystemVendorID: "0x1ab8,0x0400"},
			want:     virtualization{Name: "parallels", IsVirtual: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMacOSVirtualization(tt.hardware)
			if got != tt.want {
				t.Fatalf("detectMacOSVirtualization() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func mapsEqual(got, want map[string]any) bool {
	if len(got) != len(want) {
		return false
	}
	for key, wantValue := range want {
		gotValue, ok := got[key]
		if !ok {
			return false
		}
		gotMap, gotOK := gotValue.(map[string]any)
		wantMap, wantOK := wantValue.(map[string]any)
		if gotOK || wantOK {
			if !gotOK || !wantOK || !mapsEqual(gotMap, wantMap) {
				return false
			}
			continue
		}
		if gotValue != wantValue {
			return false
		}
	}
	return true
}
