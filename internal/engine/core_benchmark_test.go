package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkCoreFacts(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = CoreFacts(testSession)
	}
}

func BenchmarkParseLinuxMeminfoBytes(b *testing.B) {
	input := "MemTotal:       16384000 kB\nMemAvailable:    4096000 kB\nSwapTotal:       1048576 kB\nSwapFree:         262144 kB\n"
	b.ReportAllocs()
	for b.Loop() {
		_ = parseLinuxMeminfoBytes(input, "SwapFree")
	}
}

func BenchmarkParseLinuxDistroOSRelease(b *testing.B) {
	input := "ID=photon\nPRETTY_NAME=\"VMware Photon OS/Linux 5.0\"\nVERSION_ID=5.0\nVERSION_CODENAME=photon\n"
	b.ReportAllocs()
	for b.Loop() {
		got := parseLinuxDistroOSRelease(input)
		if got.ID != "photon" {
			b.Fatalf("parseLinuxDistroOSRelease().ID = %q, want photon", got.ID)
		}
	}
}

func BenchmarkLinuxRouteSourceBindings(b *testing.B) {
	input := "default via 10.16.112.1 dev ens192 proto dhcp src 10.16.125.217 metric 100\n" +
		"10.16.112.0/20 dev ens192 proto kernel scope link src 10.16.125.217\n" +
		"10.16.112.1 dev ens192 proto dhcp scope link src 10.16.125.217 metric 100\n" +
		"fe80::/64 dev ens160 proto kernel metric 256 pref medium\n"
	b.ReportAllocs()
	for b.Loop() {
		got := linuxRouteSourceBindings(input)
		if len(got) != 1 {
			b.Fatalf("linuxRouteSourceBindings() len = %d, want 1", len(got))
		}
	}
}

func BenchmarkParseLinuxProcessorModels(b *testing.B) {
	input := "processor\t: 0\nmodel name\t: Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz\n" +
		"processor\t: 1\nmodel name\t: Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz\n" +
		"processor\t: 2\nmodel name\t: Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz\n" +
		"processor\t: 3\nmodel name\t: Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz\n"
	b.ReportAllocs()
	for b.Loop() {
		got := parseLinuxProcessorModels(input)
		if len(got) != 4 {
			b.Fatalf("parseLinuxProcessorModels() len = %d, want 4", len(got))
		}
	}
}

func BenchmarkParseDarwinVMStatAvailableBytes(b *testing.B) {
	input := `Mach Virtual Memory Statistics: (page size of 4096 bytes)
Pages free:                             1364873.
Pages active:                           2653007.
Pages inactive:                         1583485.
Pages speculative:                      1061442.
Pages wired down:                        986842.
`
	b.ReportAllocs()
	for b.Loop() {
		_ = parseDarwinVMStatAvailableBytes(input)
	}
}

func BenchmarkParseDarwinSwapUsage(b *testing.B) {
	input := "total = 3072.00M  used = 1422.75M  free = 1649.25M  (encrypted)"
	b.ReportAllocs()
	for b.Loop() {
		_ = parseDarwinSwapUsage(input)
	}
}

func BenchmarkParseMacOSSystemProfilerHardware(b *testing.B) {
	input := `Hardware:

    Hardware Overview:

      Model Name: MacBook Pro
      Model Identifier: Mac14,6
      Processor Name: Apple M2 Max
      Processor Speed: 3.68 GHz
      Number of Processors: 1
      Total Number of Cores: 12
      L2 Cache (per Core): 4 MB
      L3 Cache: 24 MB
      Memory: 32 GB
      System Firmware Version: 11881.121.1
      SMC Version (system): 1.16f8
      Serial Number (system): C02TEST1234
      Hardware UUID: 11111111-2222-3333-4444-555555555555
`
	b.ReportAllocs()
	for b.Loop() {
		got := parseMacOSSystemProfilerHardware(input)
		if got.ModelIdentifier != "Mac14,6" {
			b.Fatalf("parseMacOSSystemProfilerHardware().ModelIdentifier = %q, want Mac14,6", got.ModelIdentifier)
		}
	}
}

func BenchmarkUnitFormatting(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = bytesToMB(256_586_343)
		_ = bytesToHumanReadable(1_048_575)
		_ = hertzToHumanReadable(2_365_000_000)
	}
}

func BenchmarkParseSSHHostPublicKey(b *testing.B) {
	line := "ssh-rsa YWJj root@example"
	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseSSHHostPublicKey(line)
	}
}

func BenchmarkDisksFact(b *testing.B) {
	dir := b.TempDir()
	for _, name := range []string{"sda", "vdb", "nvme0n1"} {
		disk := filepath.Join(dir, name)
		for _, subdir := range []string{"device", "queue"} {
			if err := os.MkdirAll(filepath.Join(disk, subdir), 0o700); err != nil {
				b.Fatal(err)
			}
		}
		files := map[string]string{
			"device/model":     "FastDisk\n",
			"device/vendor":    "Acme\n",
			"queue/rotational": "0\n",
			"size":             "2048\n",
		}
		for filename, value := range files {
			if err := os.WriteFile(filepath.Join(disk, filename), []byte(value), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = disksFact(dir, osHost{})
	}
}
