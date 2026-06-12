package engine

import "testing"

func BenchmarkDetectMacOSVirtualization(b *testing.B) {
	tests := []struct {
		name     string
		hardware macOSSystemProfilerHardware
		want     virtualization
	}{
		{
			name: "physical",
			hardware: macOSSystemProfilerHardware{
				ModelIdentifier:   "Mac16,10",
				BootROMVersion:    "18000.120.36",
				SubsystemVendorID: "0x14e4",
			},
			want: virtualization{Name: "physical"},
		},
		{
			name:     "vmware",
			hardware: macOSSystemProfilerHardware{ModelIdentifier: "VMware7,1"},
			want:     virtualization{Name: "vmware", IsVirtual: true},
		},
		{
			name:     "virtualbox",
			hardware: macOSSystemProfilerHardware{BootROMVersion: "VirtualBox 7.0"},
			want:     virtualization{Name: "virtualbox", IsVirtual: true},
		},
		{
			name:     "parallels",
			hardware: macOSSystemProfilerHardware{SubsystemVendorID: "0x1ab8,0x0400"},
			want:     virtualization{Name: "parallels", IsVirtual: true},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				got := detectMacOSVirtualization(tt.hardware)
				if got != tt.want {
					b.Fatalf("detectMacOSVirtualization() = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}
