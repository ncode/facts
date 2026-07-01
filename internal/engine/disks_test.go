package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestPartitionsFactAddsFirstMountpointForDevice(t *testing.T) {
	partitions := map[string]any{
		"/dev/sda2": map[string]any{
			"filesystem": "btrfs",
			"size":       "13.09 GiB",
			"size_bytes": 14050918400,
			"uuid":       "bbc18fba-8191-48c8-b8bd-30373654bb3e",
		},
	}
	mountpoints := map[string]any{
		"/": map[string]any{
			"device": "/dev/sda2",
		},
		"/boot/grub2/x86_64-efi": map[string]any{
			"device": "/dev/sda2",
		},
	}

	got := partitionsFact(partitions, mountpoints)
	want := map[string]any{
		"/dev/sda2": map[string]any{
			"filesystem": "btrfs",
			"mount":      "/",
			"size":       "13.09 GiB",
			"size_bytes": 14050918400,
			"uuid":       "bbc18fba-8191-48c8-b8bd-30373654bb3e",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionsFact() = %#v, want %#v", got, want)
	}
}

func TestPartitionsFactWithMountEntriesUsesResolverOrderForDuplicateDeviceLikeRuby(t *testing.T) {
	partitions := map[string]any{
		"/dev/sda2": map[string]any{
			"filesystem": "btrfs",
			"size":       "13.09 GiB",
			"size_bytes": 14050918400,
		},
	}
	mountEntries := []mountEntry{
		{Device: "/dev/sda2", Path: "/z-first", Filesystem: "btrfs"},
		{Device: "/dev/sda2", Path: "/a-second", Filesystem: "btrfs"},
	}
	mountpoints := map[string]any{
		"/a-second": map[string]any{"device": "/dev/sda2"},
		"/z-first":  map[string]any{"device": "/dev/sda2"},
	}

	got := partitionsFactWithMountEntries(partitions, mountEntries, mountpoints)
	partition, ok := got["/dev/sda2"].(map[string]any)
	if !ok {
		t.Fatalf("partitionsFactWithMountEntries() = %#v, want /dev/sda2 partition", got)
	}
	if partition["mount"] != "/z-first" {
		t.Fatalf("partition mount = %#v, want first resolver mountpoint /z-first", partition["mount"])
	}
}

func TestPartitionsFactMatchesFreeBSDGPTMountsByPartlabel(t *testing.T) {
	partitions := map[string]any{
		"vtbd0p2": map[string]any{"partlabel": "efiesp"},
		"vtbd0p5": map[string]any{"partlabel": "rootfs"},
	}
	mountpoints := map[string]any{
		"/":         map[string]any{"device": "/dev/gpt/rootfs", "filesystem": "ufs"},
		"/boot/efi": map[string]any{"device": "/dev/gpt/efiesp", "filesystem": "msdosfs"},
	}

	got := partitionsFact(partitions, mountpoints)
	root := got["vtbd0p5"].(map[string]any)
	if root["mount"] != "/" || root["filesystem"] != "ufs" {
		t.Fatalf("root partition = %#v, want mount and filesystem from mountpoint", root)
	}
	efi := got["vtbd0p2"].(map[string]any)
	if efi["mount"] != "/boot/efi" || efi["filesystem"] != "msdosfs" {
		t.Fatalf("efi partition = %#v, want mount and filesystem from mountpoint", efi)
	}
}

func TestPartitionForMountDeviceMatchesNamesLabelsAndUUIDs(t *testing.T) {
	t.Parallel()

	partitions := map[string]any{
		"ada0p2":  map[string]any{"partuuid": "7f6ec6ec-2e4e-11ef-9a8a-0800276f7822"},
		"vtbd0p2": map[string]any{"partlabel": "rootfs"},
		"sda1":    map[string]any{"filesystem": "ext4"},
		"ignored": "not a partition map",
	}

	tests := []struct {
		name   string
		device string
		want   string
	}{
		{name: "direct key", device: "sda1", want: "sda1"},
		{name: "dev prefix", device: "/dev/sda1", want: "sda1"},
		{name: "gpt label", device: "/dev/gpt/rootfs", want: "vtbd0p2"},
		{name: "gpt uuid", device: "/dev/gptid/7f6ec6ec-2e4e-11ef-9a8a-0800276f7822", want: "ada0p2"},
		{name: "missing", device: "/dev/gpt/missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := partitionForMountDevice(partitions, tt.device)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("partitionForMountDevice(%q) = %#v, want nil", tt.device, got)
				}
				return
			}
			want := partitions[tt.want].(map[string]any)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("partitionForMountDevice(%q) = %#v, want partition %q", tt.device, got, tt.want)
			}
		})
	}
}

func TestPartitionsFactReturnsPartitionsWithoutMountpoints(t *testing.T) {
	partitions := map[string]any{
		"/dev/sda1": map[string]any{"filesystem": "ext3"},
	}

	got := partitionsFact(partitions, nil)
	if !reflect.DeepEqual(got, partitions) {
		t.Fatalf("partitionsFact() = %#v, want %#v", got, partitions)
	}
}

func TestPartitionsFactReturnsNilForEmptyPartitions(t *testing.T) {
	if got := partitionsFact(map[string]any{}, map[string]any{"/": map[string]any{"device": "/dev/sda1"}}); got != nil {
		t.Fatalf("partitionsFact() = %#v, want nil", got)
	}
}

func TestParseLinuxLSBLKPropertyLineHandlesQuotedAndUnquotedValues(t *testing.T) {
	t.Parallel()

	line := `NAME="sda1" FSTYPE=ext4 LABEL="data \"disk\"" EMPTY="" BROKEN="unterminated`
	got := parseLinuxLSBLKPropertyLine(line)
	want := map[string]string{
		"NAME":   "sda1",
		"FSTYPE": "ext4",
		"LABEL":  `data "disk"`,
		"BROKEN": "unterminated",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxLSBLKPropertyLine(%q) = %#v, want %#v", line, got, want)
	}
}

func TestParseLinuxLSBLKPropertiesSkipsRowsWithoutValues(t *testing.T) {
	t.Parallel()

	input := "NAME=\"sda1\" FSTYPE=\"ext4\"\nNAME=\"sda2\"\nMISSING=\"name\"\n"
	got := parseLinuxLSBLKProperties(input)
	want := map[string]map[string]string{
		"sda1": {"FSTYPE": "ext4"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxLSBLKProperties(%q) = %#v, want %#v", input, got, want)
	}
}

func TestLinuxLSBLKVersionParsesUtilLinuxVersion(t *testing.T) {
	t.Parallel()

	major, minor, ok := linuxLSBLKVersion("lsblk from util-linux 2.39.3\n")
	if !ok || major != 2 || minor != 39 {
		t.Fatalf("linuxLSBLKVersion() = %d, %d, %v; want 2, 39, true", major, minor, ok)
	}
	if major, minor, ok := linuxLSBLKVersion("lsblk unknown\n"); ok || major != 0 || minor != 0 {
		t.Fatalf("linuxLSBLKVersion(no version) = %d, %d, %v; want 0, 0, false", major, minor, ok)
	}
}

func TestDiscoverPartitionsReadsSysfsPartitionEntries(t *testing.T) {
	root := t.TempDir()
	partitionDir := filepath.Join(root, "sda1")
	diskDir := filepath.Join(root, "sda")
	if err := os.Mkdir(partitionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(diskDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partitionDir, "partition"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partitionDir, "size"), []byte("4096\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(diskDir, "size"), []byte("8192\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := discoverPartitions(root, osHost{})
	want := map[string]any{
		"/dev/sda1": map[string]any{"size": "2.00 MiB", "size_bytes": 2097152},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverPartitions() = %#v, want %#v", got, want)
	}
}

func TestCurrentLinuxPartitionsAddsLSBLKParttypeLikeRubyResolver(t *testing.T) {
	root := t.TempDir()
	partitionDir := filepath.Join(root, "sda1")
	if err := os.Mkdir(partitionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(partitionDir, "partition"), "1\n")
	writeFile(t, filepath.Join(partitionDir, "size"), "234\n")

	run := func(name string, args ...string) string {
		if name != "lsblk" {
			t.Fatalf("run(%q, %#v), want lsblk", name, args)
		}
		switch strings.Join(args, " ") {
		case "--version":
			return "lsblk from util-linux 2.25\n"
		case "-p -P -o NAME,FSTYPE,UUID,LABEL,PARTUUID,PARTLABEL,PARTTYPE":
			return `NAME="/dev/sda1" FSTYPE="ext3" UUID="88077904-4fd4-476f-9af2-0f7a806ca25e" LABEL="/boot" PARTUUID="00061fe0-01" PARTLABEL="" PARTTYPE="21686148-6449-6E6F-744E-656564454649"` + "\n"
		default:
			t.Fatalf("unexpected lsblk args %#v", args)
			return ""
		}
	}

	got := currentLinuxPartitions(root, run, osHost{})
	want := map[string]any{
		"/dev/sda1": map[string]any{
			"filesystem": "ext3",
			"label":      "/boot",
			"parttype":   "21686148-6449-6E6F-744E-656564454649",
			"partuuid":   "00061fe0-01",
			"size":       "117.00 KiB",
			"size_bytes": 119808,
			"uuid":       "88077904-4fd4-476f-9af2-0f7a806ca25e",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxPartitions() = %#v, want %#v", got, want)
	}
}

func TestDiscoverPartitionsHandlesDMAndLoopDevicesLikeRubyResolver(t *testing.T) {
	root := t.TempDir()
	dmDir := filepath.Join(root, "dm-0")
	loopDir := filepath.Join(root, "loop0")
	if err := os.MkdirAll(filepath.Join(dmDir, "dm"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(loopDir, "loop"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dmDir, "dm", "name"), "VolGroup00-LogVol00\n")
	writeFile(t, filepath.Join(dmDir, "size"), "201213\n")
	writeFile(t, filepath.Join(loopDir, "loop", "backing_file"), "some_path\n")
	writeFile(t, filepath.Join(loopDir, "size"), "234\n")

	got := discoverPartitions(root, osHost{})
	want := map[string]any{
		"/dev/mapper/VolGroup00-LogVol00": map[string]any{"size": "98.25 MiB", "size_bytes": 103021056},
		"/dev/loop0":                      map[string]any{"backing_file": "some_path", "size": "117.00 KiB", "size_bytes": 119808},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverPartitions() = %#v, want %#v", got, want)
	}
}

func TestParseFreeBSDGeomPartitions_returnsRubyCompatiblePartitionFacts(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("testdata", "kern.geom.confxml"))
	if err != nil {
		t.Fatal(err)
	}

	got := parseFreeBSDGeomPartitions(string(input))
	want := map[string]any{
		"ada0p1": map[string]any{
			"partlabel":  "gptboot0",
			"parttype":   "freebsd-boot",
			"partuuid":   "503d3458-c135-11e8-bd11-7d7cd061b26f",
			"size":       "512.00 KiB",
			"size_bytes": 524288,
		},
		"ada0p2": map[string]any{
			"partlabel":  "swap0",
			"parttype":   "freebsd-swap",
			"partuuid":   "5048d40d-c135-11e8-bd11-7d7cd061b26f",
			"size":       "2.00 GiB",
			"size_bytes": 2147483648,
		},
		"ada0p3": map[string]any{
			"partlabel":  "zfs0",
			"parttype":   "freebsd-zfs",
			"partuuid":   "504f1547-c135-11e8-bd11-7d7cd061b26f",
			"size":       "474.94 GiB",
			"size_bytes": 509961306112,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFreeBSDGeomPartitions() = %#v, want %#v", got, want)
	}
}

func TestFreeBSDPartitionTypePrefersTypeThenRawType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config freeBSDGeomConfig
		want   string
	}{
		{name: "type", config: freeBSDGeomConfig{Type: " freebsd-zfs ", RawType: "raw"}, want: "freebsd-zfs"},
		{name: "raw type fallback", config: freeBSDGeomConfig{RawType: " efi "}, want: "efi"},
		{name: "empty", config: freeBSDGeomConfig{Type: " ", RawType: "\t"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := freeBSDPartitionType(tt.config); got != tt.want {
				t.Fatalf("freeBSDPartitionType(%#v) = %q, want %q", tt.config, got, tt.want)
			}
		})
	}
}

func TestParseFreeBSDGeomDisks_returnsRubyCompatibleDiskFacts(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("testdata", "kern.geom.confxml"))
	if err != nil {
		t.Fatal(err)
	}

	got := parseFreeBSDGeomDisks(string(input))
	want := map[string]any{
		"ada0": map[string]any{
			"model":         "Samsung SSD 850 PRO 512GB",
			"serial_number": "S250NXAG959927J",
			"size":          "476.94 GiB",
			"size_bytes":    512110190592,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseFreeBSDGeomDisks() = %#v, want %#v", got, want)
	}
}

func TestParseFreeBSDGeomDisks_omitsTypeWithoutRotationRate(t *testing.T) {
	input := `<mesh><class><name>DISK</name><geom><provider><name>ada1</name><config><descr>Unknown Disk</descr></config></provider></geom></class></mesh>`

	got := parseFreeBSDGeomDisks(input)
	disk := got["ada1"].(map[string]any)
	if _, ok := disk["type"]; ok {
		t.Fatalf("disk type = %#v, want omitted", disk["type"])
	}
}

func TestFreeBSDDiskTypeMapsRotationRates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"unknown", "0", ""},
		{"non_rotating", "1", "ssd"},
		{"rotational", "7200", "hdd"},
		{"missing", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := freeBSDDiskType(tt.in); got != tt.want {
				t.Fatalf("freeBSDDiskType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

const openBSDDisklabelSD0 = `# /dev/rsd0c:
type: SCSI
disk: SCSI disk
label: Block Device
duid: 942d2f143e47054f
flags:
bytes/sector: 512
sectors/track: 63
tracks/cylinder: 255
sectors/cylinder: 16065
cylinders: 1305
total sectors: 20971520
boundstart: 565248
boundend: 20971520
16 partitions:
#                size           offset  fstype [fsize bsize   cpg]
  a:          2409248           565248  4.2BSD   2048 16384 12960 # /
  b:           524288          2974496    swap                    # none
  c:         20971520                0  unused
  d:          6291456          3498784  4.2BSD   2048 16384 12960 # /usr
  e:          4194304          9790240  4.2BSD   2048 16384 12960 # /home
  i:           532480            32768   MSDOS
`

const netBSDDisklabelLD4 = `# /dev/rld4:
type: ld
disk: ld4
label: fictitious
flags:
bytes/sector: 512
sectors/track: 63
tracks/cylinder: 16
sectors/cylinder: 1008
cylinders: 16383
total sectors: 20971520
rpm: 3600
interleave: 1
trackskew: 0
cylinderskew: 0
headswitch: 0		# microseconds
track-to-track seek: 0	# microseconds
drivedata: 0
6 partitions:
#        size    offset     fstype [fsize bsize cpg/sgs]
 c:  20971520         0     unused      0     0        # (Cyl.      0 -  20805*)
 e:    163840     32768      MSDOS                     # (Cyl.     32*-    195*)
 f:     32767         1    unknown                     # (Cyl.      0*-     32*)
disklabel: boot block size 0
disklabel: super block size 0
`

const netBSDWedgesLD4 = `/dev/rld4: 2 wedges:
dk0: EFI, 163840 blocks at 32768, type: msdos
dk1: netbsd-root, 20766720 blocks at 196608, type: ffs
`

const dragonFlyDisklabelDA0S1 = `# /dev/da0s1:
#
# Calculated informational fields for the slice:
#
# boot space:    1012224 bytes
# data space:  134213632 blocks	# 131068.00 MB (137434759168 bytes)
#
# NOTE: The partition data base and stop are physically
#       aligned instead of slice-relative aligned.
#
# All byte equivalent offsets must be aligned.
#
diskid: 206f7902-abb6-11ee-8d16-010000000000
label:
boot2 data base:      0x000000001000
partitions data base: 0x0000000f8200
partitions data stop: 0x001fffcf8200
backup label:         0x001fffd57c00
total size:           0x001fffd58c00	# 131069.35 MB
alignment: 4096
display block size: 1024	# for partition display and edit only
16 partitions:
#          size     offset    fstype   fsuuid
  a:     786432          0    4.2BSD	#     768.000MB
  b:    2097152     786432      swap	#    2048.000MB
  d:  131330048    2883584   HAMMER2	#  128252.000MB
  a-stor_uuid: 20778a7a-abb6-11ee-8d16-010000000000
  b-stor_uuid: 20778b27-abb6-11ee-8d16-010000000000
  d-stor_uuid: 20778bb3-abb6-11ee-8d16-010000000000
`

func TestParseBSDDisklabelDisk_returnsSizeFacts(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
	}{
		{name: "openbsd", input: openBSDDisklabelSD0},
		{name: "netbsd", input: netBSDDisklabelLD4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBSDDisklabelDisk(tt.input)
			want := map[string]any{"size": "10.00 GiB", "size_bytes": 10_737_418_240}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("parseBSDDisklabelDisk() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestParseDragonFlyDiskInfo_returnsSizeFacts(t *testing.T) {
	got := parseDragonFlyDiskInfo("/dev/da0         blksize=512  offset=0x000000000000 size=0x002000000000  128.00 GB\n")
	want := map[string]any{"size": "128.00 GiB", "size_bytes": 137_438_953_472}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDragonFlyDiskInfo() = %#v, want %#v", got, want)
	}
}

func TestParseOpenBSDDisklabelPartitions_returnsDevicePartitions(t *testing.T) {
	got := parseBSDDisklabelPartitions("sd0", openBSDDisklabelSD0)
	want := map[string]any{
		"/dev/sd0a": map[string]any{"filesystem": "4.2BSD", "size": "1.15 GiB", "size_bytes": 1_233_534_976},
		"/dev/sd0b": map[string]any{"filesystem": "swap", "size": "256.00 MiB", "size_bytes": 268_435_456},
		"/dev/sd0d": map[string]any{"filesystem": "4.2BSD", "size": "3.00 GiB", "size_bytes": 3_221_225_472},
		"/dev/sd0e": map[string]any{"filesystem": "4.2BSD", "size": "2.00 GiB", "size_bytes": 2_147_483_648},
		"/dev/sd0i": map[string]any{"filesystem": "MSDOS", "size": "260.00 MiB", "size_bytes": 272_629_760},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseBSDDisklabelPartitions() = %#v, want %#v", got, want)
	}
}

func TestParseBSDDiskNamesSplitsOpenBSDCommaSeparatedNames(t *testing.T) {
	got := parseBSDDiskNames("openbsd", "hw.disknames=sd0:942d2f143e47054f,sd1:1111111111111111,cd0:\n")
	want := []string{"sd0", "sd1", "cd0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseBSDDiskNames() = %#v, want %#v", got, want)
	}
}

func TestIsBSDDiskNameAllowsOnlyAlnumDeviceNames(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"":        false,
		"sd0":     true,
		"nvme0":   true,
		"da0p1":   true,
		"sd0:":    false,
		"sd0-":    false,
		"sd0.eli": false,
	}
	for name, want := range tests {
		if got := isBSDDiskName(name); got != want {
			t.Fatalf("isBSDDiskName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestCurrentBSDDisksOmitWhenNoRunnableDiskData(t *testing.T) {
	t.Parallel()

	if got := currentBSDDisks("openbsd", nil); got != nil {
		t.Fatalf("currentBSDDisks(nil run) = %#v, want nil", got)
	}
	if got := currentBSDDisks("openbsd", func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("currentBSDDisks(no devices) = %#v, want nil", got)
	}
	got := currentBSDDisks("openbsd", func(name string, args ...string) string {
		switch fakeRunKey(name, args...) {
		case fakeRunKey("sysctl", "-n", "hw.disknames"):
			return "sd0:942d2f143e47054f\n"
		case fakeRunKey("disklabel", "sd0"):
			return ""
		default:
			t.Fatalf("unexpected command %q %#v", name, args)
			return ""
		}
	})
	if got != nil {
		t.Fatalf("currentBSDDisks(empty disklabel) = %#v, want nil", got)
	}
}

func TestCurrentOpenBSDPartitionsReadsEveryDiskName(t *testing.T) {
	got := currentOpenBSDPartitions(func(name string, args ...string) string {
		switch fakeRunKey(name, args...) {
		case fakeRunKey("sysctl", "-n", "hw.disknames"):
			return "sd0:942d2f143e47054f,sd1:1111111111111111\n"
		case fakeRunKey("disklabel", "sd0"):
			return openBSDDisklabelSD0
		case fakeRunKey("disklabel", "sd1"):
			return strings.ReplaceAll(openBSDDisklabelSD0, "sd0", "sd1")
		default:
			t.Fatalf("unexpected command %q %#v", name, args)
			return ""
		}
	})
	for _, key := range []string{"/dev/sd0a", "/dev/sd1a"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("currentOpenBSDPartitions() = %#v, want key %q", got, key)
		}
	}
}

func TestCurrentOpenBSDPartitionsOmitWhenNoRunnableDiskData(t *testing.T) {
	t.Parallel()

	if got := currentOpenBSDPartitions(nil); got != nil {
		t.Fatalf("currentOpenBSDPartitions(nil run) = %#v, want nil", got)
	}
	if got := currentOpenBSDPartitions(func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("currentOpenBSDPartitions(no devices) = %#v, want nil", got)
	}
}

func TestCurrentNetBSDPartitionsReadsDkctlWedges(t *testing.T) {
	disklabelCalled := false
	got := currentNetBSDPartitions(func(name string, args ...string) string {
		switch fakeRunKey(name, args...) {
		case fakeRunKey("sysctl", "-n", "hw.disknames"):
			return "ld4\n"
		case fakeRunKey("sh", "-c", "disklabel ld4 2>/dev/null || true"):
			disklabelCalled = true
			return strings.Replace(netBSDDisklabelLD4, "bytes/sector: 512", "bytes/sector: 4096", 1)
		case fakeRunKey("dkctl", "ld4", "listwedges"):
			if !disklabelCalled {
				t.Fatal("currentNetBSDPartitions() read wedges before disklabel")
			}
			return netBSDWedgesLD4
		default:
			t.Fatalf("unexpected command %q %#v", name, args)
			return ""
		}
	})

	want := map[string]any{
		"/dev/dk0": map[string]any{"filesystem": "msdos", "partlabel": "EFI", "size": "640.00 MiB", "size_bytes": 671_088_640},
		"/dev/dk1": map[string]any{"filesystem": "ffs", "partlabel": "netbsd-root", "size": "79.22 GiB", "size_bytes": 85_060_485_120},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentNetBSDPartitions() = %#v, want %#v", got, want)
	}
	if !disklabelCalled {
		t.Fatal("currentNetBSDPartitions() did not read disklabel before parsing wedges")
	}
}

func TestCurrentNetBSDPartitionsFallsBackToDisklabelWhenNoWedges(t *testing.T) {
	t.Parallel()

	got := currentNetBSDPartitions(func(name string, args ...string) string {
		switch fakeRunKey(name, args...) {
		case fakeRunKey("sysctl", "-n", "hw.disknames"):
			return "ld4\n"
		case fakeRunKey("sh", "-c", "disklabel ld4 2>/dev/null || true"):
			return netBSDDisklabelLD4
		case fakeRunKey("dkctl", "ld4", "listwedges"):
			return ""
		default:
			t.Fatalf("unexpected command %q %#v", name, args)
			return ""
		}
	})
	if _, ok := got["/dev/ld4e"]; !ok {
		t.Fatalf("currentNetBSDPartitions() = %#v, want fallback disklabel partition /dev/ld4e", got)
	}
}

func TestCurrentNetBSDPartitionsOmitWhenNoRunnableDiskData(t *testing.T) {
	t.Parallel()

	if got := currentNetBSDPartitions(nil); got != nil {
		t.Fatalf("currentNetBSDPartitions(nil run) = %#v, want nil", got)
	}
	if got := currentNetBSDPartitions(func(string, ...string) string { return "" }); got != nil {
		t.Fatalf("currentNetBSDPartitions(no devices) = %#v, want nil", got)
	}
}

func TestParseNetBSDDkctlWedges_returnsDevicePartitions(t *testing.T) {
	got := parseNetBSDDkctlWedges(netBSDWedgesLD4, 512)
	want := map[string]any{
		"/dev/dk0": map[string]any{"filesystem": "msdos", "partlabel": "EFI", "size": "80.00 MiB", "size_bytes": 83_886_080},
		"/dev/dk1": map[string]any{"filesystem": "ffs", "partlabel": "netbsd-root", "size": "9.90 GiB", "size_bytes": 10_632_560_640},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNetBSDDkctlWedges() = %#v, want %#v", got, want)
	}
}

func TestPartitionsFactJoinsAllOpenBSDMountpointsFromMountpointDevices(t *testing.T) {
	partitions := parseBSDDisklabelPartitions("sd0", openBSDDisklabelSD0)
	mountpoints := openBSDMountpointsFact(`/dev/sd0a on / type ffs (local)
/dev/sd0d on /usr type ffs (local, nodev)
/dev/sd0e on /home type ffs (local, nodev, nosuid)
`, "")

	got := partitionsFact(partitions, mountpoints)
	for _, tt := range []struct {
		device string
		mount  string
	}{
		{"/dev/sd0a", "/"},
		{"/dev/sd0d", "/usr"},
		{"/dev/sd0e", "/home"},
	} {
		partition, ok := got[tt.device].(map[string]any)
		if !ok {
			t.Fatalf("partitionsFact() = %#v, want partition %q", got, tt.device)
		}
		if partition["mount"] != tt.mount {
			t.Fatalf("%s mount = %#v, want %#v", tt.device, partition["mount"], tt.mount)
		}
	}
}

func TestCurrentDisksUsesBSDDisklabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos         string
		disknames    string
		device       string
		disklabelCmd string
		disklabel    string
		wantDiskKey  string
	}{
		{
			goos:         "openbsd",
			disknames:    "sd0:942d2f143e47054f\n",
			device:       "sd0",
			disklabelCmd: "disklabel sd0",
			disklabel:    openBSDDisklabelSD0,
			wantDiskKey:  "sd0",
		},
		{
			goos:         "netbsd",
			disknames:    "ld4 dk0 dk1\n",
			device:       "ld4",
			disklabelCmd: "sh -c disklabel ld4 2>/dev/null || true",
			disklabel:    netBSDDisklabelLD4,
			wantDiskKey:  "ld4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			t.Parallel()

			got := currentDisks(tt.goos, func(name string, args ...string) string {
				switch strings.Join(append([]string{name}, args...), " ") {
				case "sysctl -n hw.disknames":
					return tt.disknames
				case tt.disklabelCmd:
					return tt.disklabel
				default:
					t.Fatalf("unexpected command %q %#v", name, args)
					return ""
				}
			}, osHost{})
			want := map[string]any{
				tt.wantDiskKey: map[string]any{"size": "10.00 GiB", "size_bytes": 10_737_418_240},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("currentDisks(%q) = %#v, want %#v", tt.goos, got, want)
			}
		})
	}
}

func TestCurrentDisksUsesDragonFlyDiskinfo(t *testing.T) {
	got := currentDisks("dragonfly", func(name string, args ...string) string {
		switch strings.Join(append([]string{name}, args...), " ") {
		case "sysctl -n kern.disks":
			return "da0 vn3 vn2\n"
		case "diskinfo /dev/da0":
			return "/dev/da0         blksize=512  offset=0x000000000000 size=0x002000000000  128.00 GB\n"
		case "diskinfo /dev/vn3", "diskinfo /dev/vn2":
			return "No such file or directory\n"
		default:
			t.Fatalf("unexpected command %q %#v", name, args)
			return ""
		}
	}, osHost{})
	want := map[string]any{
		"da0": map[string]any{"size": "128.00 GiB", "size_bytes": 137_438_953_472},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentDisks(dragonfly) = %#v, want %#v", got, want)
	}
}

func TestParseDragonFlyDisklabelPartitions_returnsDevicePartitions(t *testing.T) {
	got := parseDragonFlyDisklabelPartitions("da0s1", dragonFlyDisklabelDA0S1)
	want := map[string]any{
		"/dev/da0s1a": map[string]any{"filesystem": "4.2BSD", "size": "768.00 MiB", "size_bytes": 805_306_368},
		"/dev/da0s1b": map[string]any{"filesystem": "swap", "size": "2.00 GiB", "size_bytes": 2_147_483_648},
		"/dev/da0s1d": map[string]any{"filesystem": "HAMMER2", "size": "125.25 GiB", "size_bytes": 134_481_969_152},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDragonFlyDisklabelPartitions() = %#v, want %#v", got, want)
	}
}

func TestCurrentDragonFlyPartitionsReadsKernelDiskNames(t *testing.T) {
	got := currentDragonFlyPartitions(func(name string, args ...string) string {
		switch strings.Join(append([]string{name}, args...), " ") {
		case "sysctl -n kern.disks":
			return "da0 vn3\n"
		case "disklabel da0":
			return "disklabel: Operation not supported by device\n"
		case "disklabel da0s1":
			return dragonFlyDisklabelDA0S1
		case "disklabel da0s2", "disklabel da0s3", "disklabel da0s4",
			"disklabel vn3", "disklabel vn3s1", "disklabel vn3s2", "disklabel vn3s3", "disklabel vn3s4":
			return "disklabel: Operation not supported by device\n"
		default:
			t.Fatalf("unexpected command %q %#v", name, args)
			return ""
		}
	})
	if _, ok := got["/dev/da0s1d"]; !ok {
		t.Fatalf("currentDragonFlyPartitions() = %#v, want /dev/da0s1d", got)
	}
}

func TestCurrentDragonFlyPartitionsTriesOtherSlices(t *testing.T) {
	got := currentDragonFlyPartitions(func(name string, args ...string) string {
		switch strings.Join(append([]string{name}, args...), " ") {
		case "sysctl -n kern.disks":
			return "da0\n"
		case "disklabel da0", "disklabel da0s1":
			return "disklabel: Operation not supported by device\n"
		case "disklabel da0s2":
			return strings.ReplaceAll(dragonFlyDisklabelDA0S1, "/dev/da0s1", "/dev/da0s2")
		case "disklabel da0s3", "disklabel da0s4":
			return ""
		default:
			t.Fatalf("unexpected command %q %#v", name, args)
			return ""
		}
	})
	if _, ok := got["/dev/da0s2d"]; !ok {
		t.Fatalf("currentDragonFlyPartitions() = %#v, want /dev/da0s2d", got)
	}
}

func TestCurrentIllumosPartitionsReadsVTOCSlices(t *testing.T) {
	vtoc := `* /dev/rdsk/c9t0d0s2 EFI partition map
*
* Dimensions:
*         512 bytes/sector
*
*                            First       Sector      Last
* Partition  Tag  Flags      Sector       Count      Sector  Mount Directory
       0     12    00          256        2048        2303
       1      4    00         2304        4096        6399
       2      5    01            0        8192        8191
`
	run := func(name string, args ...string) string {
		switch strings.Join(append([]string{name}, args...), " ") {
		case "prtvtoc /dev/rdsk/c9t0d0s2":
			return vtoc
		case "fstyp /dev/rdsk/c9t0d0s0":
			return "pcfs\n"
		case "fstyp /dev/rdsk/c9t0d0s1":
			return "zfs\n"
		default:
			t.Fatalf("unexpected command %q %#v", name, args)
			return ""
		}
	}
	glob := func(pattern string) ([]string, error) {
		if pattern != "/dev/rdsk/*s2" {
			t.Fatalf("glob pattern = %q, want /dev/rdsk/*s2", pattern)
		}
		return []string{"/dev/rdsk/c9t0d0s2"}, nil
	}

	got := currentIllumosPartitions(run, glob)
	want := map[string]any{
		"/dev/dsk/c9t0d0s0": map[string]any{"filesystem": "pcfs", "size": "1.00 MiB", "size_bytes": 1_048_576},
		"/dev/dsk/c9t0d0s1": map[string]any{"filesystem": "zfs", "size": "2.00 MiB", "size_bytes": 2_097_152},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentIllumosPartitions() = %#v, want %#v", got, want)
	}
}

func TestCurrentIllumosPartitionsOmitWhenNoRunnableDiskData(t *testing.T) {
	t.Parallel()

	run := func(string, ...string) string {
		t.Fatal("currentIllumosPartitions() ran command without whole slices")
		return ""
	}
	if got := currentIllumosPartitions(nil, func(string) ([]string, error) { return nil, nil }); got != nil {
		t.Fatalf("currentIllumosPartitions(nil run) = %#v, want nil", got)
	}
	if got := currentIllumosPartitions(run, nil); got != nil {
		t.Fatalf("currentIllumosPartitions(nil glob) = %#v, want nil", got)
	}
	if got := currentIllumosPartitions(run, func(string) ([]string, error) { return nil, os.ErrNotExist }); got != nil {
		t.Fatalf("currentIllumosPartitions(glob error) = %#v, want nil", got)
	}
	if got := currentIllumosPartitions(run, func(string) ([]string, error) { return []string{"/dev/rdsk/not-a-whole-slice"}, nil }); got != nil {
		t.Fatalf("currentIllumosPartitions(no whole slices) = %#v, want nil", got)
	}
}

func TestIllumosPartitionFilesystemSkipsUnknownAndDeviceErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "valid", input: "zfs\n", want: "zfs"},
		{name: "empty", input: "", want: ""},
		{name: "unknown", input: "unknown_fstyp\n", want: ""},
		{name: "device error", input: "/dev/rdsk/c9t0d0s0: I/O error\n", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := illumosPartitionFilesystem(tt.input); got != tt.want {
				t.Fatalf("illumosPartitionFilesystem(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDisksFacts_omittedWhenNoDevicesEnumerate(t *testing.T) {
	t.Parallel()

	if got := disksFacts(nil); got != nil {
		t.Fatalf("disksFacts(nil) = %#v, want nil", got)
	}
	if got := disksFacts(map[string]any{}); got != nil {
		t.Fatalf("disksFacts(empty) = %#v, want nil", got)
	}

	disks := map[string]any{"sda": map[string]any{"size": "8.00 GiB"}}
	want := []ResolvedFact{{Name: "disks", Value: disks}}
	if got := disksFacts(disks); !reflect.DeepEqual(got, want) {
		t.Fatalf("disksFacts() = %#v, want %#v", got, want)
	}
}

func TestPartitionsFacts_omittedWhenNoDevicesEnumerate(t *testing.T) {
	t.Parallel()

	if got := partitionsFacts(nil); got != nil {
		t.Fatalf("partitionsFacts(nil) = %#v, want nil", got)
	}
	if got := partitionsFacts(map[string]any{}); got != nil {
		t.Fatalf("partitionsFacts(empty) = %#v, want nil", got)
	}

	partitions := map[string]any{"/dev/sda1": map[string]any{"size": "8.00 GiB"}}
	want := []ResolvedFact{{Name: "partitions", Value: partitions}}
	if got := partitionsFacts(partitions); !reflect.DeepEqual(got, want) {
		t.Fatalf("partitionsFacts() = %#v, want %#v", got, want)
	}
}

func TestMountpointsFacts_omittedWhenUnresolved(t *testing.T) {
	t.Parallel()

	if got := mountpointsFacts(nil); got != nil {
		t.Fatalf("mountpointsFacts(nil) = %#v, want nil", got)
	}
	if got := mountpointsFacts(map[string]any{}); got != nil {
		t.Fatalf("mountpointsFacts(empty) = %#v, want nil", got)
	}

	mountpoints := map[string]any{"/": map[string]any{"size": "8.00 GiB"}}
	want := []ResolvedFact{{Name: "mountpoints", Value: mountpoints}}
	if got := mountpointsFacts(mountpoints); !reflect.DeepEqual(got, want) {
		t.Fatalf("mountpointsFacts() = %#v, want %#v", got, want)
	}
}

func TestParseZFSPoolFacts_matchesRubyFacterFixtures(t *testing.T) {
	zfsOutput, err := os.ReadFile(filepath.Join("testdata", "zfs"))
	if err != nil {
		t.Fatal(err)
	}
	zpoolOutput, err := os.ReadFile(filepath.Join("testdata", "zpool"))
	if err != nil {
		t.Fatal(err)
	}
	zpoolFeatureOutput, err := os.ReadFile(filepath.Join("testdata", "zpool-with-featureflags"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		facts []ResolvedFact
		want  map[string]any
	}{
		{
			name:  "zfs versions",
			facts: zfsFactsFromUpgradeOutput(string(zfsOutput)),
			want: map[string]any{
				"zfs.feature_numbers": []string{"1", "2", "3", "4", "5", "6"},
				"zfs.version":         "6",
			},
		},
		{
			name:  "zpool legacy versions",
			facts: zpoolFactsFromUpgradeOutput(string(zpoolOutput)),
			want: map[string]any{
				"zpool.feature_numbers": []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "30", "31", "32", "33", "34"},
				"zpool.version":         "34",
			},
		},
		{
			name:  "zpool feature flags",
			facts: zpoolFactsFromUpgradeOutput(string(zpoolFeatureOutput)),
			want: map[string]any{
				"zpool.feature_numbers": []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28"},
				"zpool.feature_flags":   []string{"async_destroy", "empty_bpobj", "lz4_compress", "multi_vdev_crash_dump", "spacemap_histogram", "enabled_txg", "hole_birth", "extensible_dataset", "embedded_data", "bookmarks", "filesystem_limits", "large_blocks", "large_dnode", "sha512", "skein", "device_removal", "obsolete_counts", "zpool_checkpoint", "spacemap_v2"},
				"zpool.version":         "5000",
			},
		},
		{
			name:  "zfs invalid output omitted",
			facts: zfsFactsFromUpgradeOutput("internal error: failed to initialize ZFS library\n"),
			want:  map[string]any{},
		},
		{
			name:  "zpool invalid output omitted",
			facts: zpoolFactsFromUpgradeOutput("internal error: failed to initialize ZFS library\n"),
			want:  map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := factsByName(tt.facts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("facts = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func factsByName(facts []ResolvedFact) map[string]any {
	values := make(map[string]any, len(facts))
	for _, fact := range facts {
		values[fact.Name] = fact.Value
	}
	return values
}

func TestFilesystemsFacts_omittedWhenUnresolved(t *testing.T) {
	t.Parallel()

	if got := filesystemsFacts(nil); got != nil {
		t.Fatalf("filesystemsFacts(nil) = %#v, want nil", got)
	}
	if got := filesystemsFacts([]string{}); got != nil {
		t.Fatalf("filesystemsFacts([]) = %#v, want nil", got)
	}
	want := []ResolvedFact{{Name: "filesystems", Value: []string{"apfs", "autofs", "devfs"}}}
	if got := filesystemsFacts([]string{"apfs", "autofs", "devfs"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("filesystemsFacts() = %#v, want %#v", got, want)
	}
}

func TestCoreFacts_includeFilesystems(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("filesystems resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession, nil))
	got, ok := collection["filesystems"].([]string)
	if !ok || len(got) == 0 {
		t.Fatalf("filesystems = %#v, want non-empty array of filesystem names", collection["filesystems"])
	}
}

func TestDisksFact_readsLinuxSysfsBlockDevices(t *testing.T) {
	dir := t.TempDir()
	disk := filepath.Join(dir, "sda")
	for _, subdir := range []string{"device", "queue"} {
		if err := os.MkdirAll(filepath.Join(disk, subdir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"device/model":     "FastDisk\n",
		"device/vendor":    "Acme\n",
		"queue/rotational": "0\n",
		"size":             "2048\n",
	}
	for name, value := range files {
		if err := os.WriteFile(filepath.Join(disk, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got := disksFact(dir, osHost{})
	want := map[string]any{
		"sda": map[string]any{
			"model":      "FastDisk",
			"vendor":     "Acme",
			"type":       "ssd",
			"size":       "1.00 MiB",
			"size_bytes": 1_048_576,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("disksFact() = %#v, want %#v", got, want)
	}
}

func TestCurrentLinuxDisksAddsSerialNumberAndWWN(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"sda", "sr0"} {
		disk := filepath.Join(dir, name)
		for _, subdir := range []string{"device", "queue"} {
			if err := os.MkdirAll(filepath.Join(disk, subdir), 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
	files := map[string]string{
		"sda/device/model":     "model2\n",
		"sda/device/vendor":    "vendor2\n",
		"sda/queue/rotational": "1\n",
		"sda/size":             "231\n",
		"sr0/device/model":     "model1\n",
		"sr0/device/vendor":    "vendor1\n",
		"sr0/queue/rotational": "0\n",
		"sr0/size":             "12\n",
	}
	for name, value := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(name string, args ...string) string {
		if name != "lsblk" || len(args) != 4 || args[0] != "-dn" || args[1] != "-o" {
			t.Fatalf("run(%q, %#v), want lsblk -dn -o <field> /dev/<disk>", name, args)
		}
		switch strings.Join(args[2:], " ") {
		case "serial /dev/sda":
			return "B2EI34F1AL\n"
		case "wwn /dev/sda":
			return "29429191.0\n"
		case "serial /dev/sr0", "wwn /dev/sr0":
			return ""
		default:
			t.Fatalf("unexpected lsblk args %#v", args)
			return ""
		}
	}

	got := currentLinuxDisks(dir, run, osHost{})
	want := map[string]any{
		"sda": map[string]any{
			"model":         "model2",
			"serial_number": "B2EI34F1AL",
			"size":          "115.50 KiB",
			"size_bytes":    118_272,
			"type":          "hdd",
			"vendor":        "vendor2",
			"wwn":           "29429191.0",
		},
		"sr0": map[string]any{
			"model":      "model1",
			"size":       "6.00 KiB",
			"size_bytes": 6144,
			"type":       "ssd",
			"vendor":     "vendor1",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentLinuxDisks() = %#v, want %#v", got, want)
	}
}

func TestDisksCoreFactsUsesSessionHostForLinuxDiskPartitionAndMountpointFacts(t *testing.T) {
	host := &fakeHostOS{
		platform: "linux",
		dirs: map[string][]os.DirEntry{
			"/sys/block":       fakeDirEntries("sdz"),
			"/sys/class/block": fakeDirEntries("sdz1"),
		},
		files: map[string][]byte{
			"/sys/block/sdz/device/model":     []byte("FastDisk\n"),
			"/sys/block/sdz/device/vendor":    []byte("Acme\n"),
			"/sys/block/sdz/queue/rotational": []byte("0\n"),
			"/sys/block/sdz/size":             []byte("2048\n"),
			"/sys/class/block/sdz1/size":      []byte("4096\n"),
			"/proc/self/mounts":               []byte("/dev/sdz1 / ext4 rw,noatime 0 0\n"),
			"/proc/cmdline":                   []byte("root=/dev/sdz1\n"),
		},
		stats: map[string]os.FileInfo{
			"/sys/block/sdz/device":           fakeFileInfo{name: "device", mode: os.ModeDir, isDir: true},
			"/sys/class/block/sdz1/partition": fakeFileInfo{name: "partition"},
		},
		mountStats: map[string]mountStat{
			"/": {SizeBytes: 4096, AvailableBytes: 1024, UsedBytes: 3072},
		},
		runOutputs: map[string]string{
			fakeRunKey("lsblk", "-dn", "-o", "serial", "/dev/sdz"):                                      "SERIAL42\n",
			fakeRunKey("lsblk", "-dn", "-o", "wwn", "/dev/sdz"):                                         "wwn-42\n",
			fakeRunKey("lsblk", "--version"):                                                            "lsblk from util-linux 2.25\n",
			fakeRunKey("lsblk", "-p", "-P", "-o", "NAME,FSTYPE,UUID,LABEL,PARTUUID,PARTLABEL,PARTTYPE"): `NAME="/dev/sdz1" FSTYPE="ext4" UUID="uuid-root" LABEL="rootfs" PARTUUID="partuuid-root" PARTLABEL="root" PARTTYPE="linux"` + "\n",
			fakeRunKey("zfs", "upgrade", "-v"):                                                          "",
			fakeRunKey("zpool", "upgrade", "-v"):                                                        "",
		},
	}
	s := NewSessionContext(t.Context())
	s.host = host

	got := factsByName(disksCoreFacts(s))
	wantDisks := map[string]any{
		"sdz": map[string]any{
			"model":         "FastDisk",
			"serial_number": "SERIAL42",
			"size":          "1.00 MiB",
			"size_bytes":    1_048_576,
			"type":          "ssd",
			"vendor":        "Acme",
			"wwn":           "wwn-42",
		},
	}
	if !reflect.DeepEqual(got["disks"], wantDisks) {
		t.Fatalf("disks = %#v, want %#v", got["disks"], wantDisks)
	}
	wantMountpoints := map[string]any{
		"/": map[string]any{
			"available":       "1.00 KiB",
			"available_bytes": 1024,
			"capacity":        "75.00%",
			"device":          "/dev/sdz1",
			"filesystem":      "ext4",
			"options":         []string{"rw", "noatime"},
			"size":            "4.00 KiB",
			"size_bytes":      4096,
			"used":            "3.00 KiB",
			"used_bytes":      3072,
		},
	}
	if !reflect.DeepEqual(got["mountpoints"], wantMountpoints) {
		t.Fatalf("mountpoints = %#v, want %#v", got["mountpoints"], wantMountpoints)
	}
	wantPartitions := map[string]any{
		"/dev/sdz1": map[string]any{
			"filesystem": "ext4",
			"label":      "rootfs",
			"mount":      "/",
			"partlabel":  "root",
			"parttype":   "linux",
			"partuuid":   "partuuid-root",
			"size":       "2.00 MiB",
			"size_bytes": 2_097_152,
			"uuid":       "uuid-root",
		},
	}
	if !reflect.DeepEqual(got["partitions"], wantPartitions) {
		t.Fatalf("partitions = %#v, want %#v", got["partitions"], wantPartitions)
	}
	if want := []string{"/sys/block", "/sys/class/block"}; !reflect.DeepEqual(host.readDirCalls, want) {
		t.Fatalf("readDir calls = %#v, want %#v", host.readDirCalls, want)
	}
	if want := []string{"/"}; !reflect.DeepEqual(host.statMountpointCalls, want) {
		t.Fatalf("statMountpoint calls = %#v, want %#v", host.statMountpointCalls, want)
	}
}

func TestDisksFactOmitsOverflowingLinuxSectorSize(t *testing.T) {
	host := &fakeHostOS{
		dirs: map[string][]os.DirEntry{
			"/sys/block": fakeDirEntries("sdz"),
		},
		files: map[string][]byte{
			"/sys/block/sdz/device/model": []byte("OverflowDisk\n"),
			"/sys/block/sdz/size":         []byte("18014398509481984\n"),
		},
		stats: map[string]os.FileInfo{
			"/sys/block/sdz/device": fakeFileInfo{name: "device", mode: os.ModeDir, isDir: true},
		},
	}

	got := disksFact("/sys/block", host)
	disk := got["sdz"].(map[string]any)
	if _, ok := disk["size_bytes"]; ok {
		t.Fatalf("size_bytes = %#v, want omitted for overflowing sector count", disk["size_bytes"])
	}
	if _, ok := disk["size"]; ok {
		t.Fatalf("size = %#v, want omitted for overflowing sector count", disk["size"])
	}
}

func TestCurrentPartitionsUsesSessionHostGlobForIllumos(t *testing.T) {
	const vtoc = `* Dimensions:
*         512 bytes/sector
*
       0     12    00          256        2048        2303
       2      5    01            0        8192        8191
`
	host := &fakeHostOS{
		platform: "illumos",
		globs: map[string][]string{
			"/dev/rdsk/*s2": {"/dev/rdsk/c0t0d0s2"},
		},
		runOutputs: map[string]string{
			fakeRunKey("prtvtoc", "/dev/rdsk/c0t0d0s2"): vtoc,
			fakeRunKey("fstyp", "/dev/rdsk/c0t0d0s0"):   "ufs\n",
		},
	}
	s := NewSessionContext(t.Context())
	s.host = host

	got := currentPartitions(s)
	want := map[string]any{
		"/dev/dsk/c0t0d0s0": map[string]any{
			"filesystem": "ufs",
			"size":       "1.00 MiB",
			"size_bytes": 1_048_576,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentPartitions() = %#v, want %#v", got, want)
	}
	if want := []string{"/dev/rdsk/*s2"}; !reflect.DeepEqual(host.globCalls, want) {
		t.Fatalf("glob calls = %#v, want %#v", host.globCalls, want)
	}
}

func TestCurrentPartitionsDispatchesBySessionPlatform(t *testing.T) {
	freeBSDGeom, err := os.ReadFile(filepath.Join("testdata", "kern.geom.confxml"))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		host      *fakeHostOS
		wantKey   string
		wantCalls []fakeHostRunCall
	}{
		{
			name: "freebsd",
			host: &fakeHostOS{
				platform: "freebsd",
				runOutputs: map[string]string{
					fakeRunKey("sysctl", "-n", "kern.geom.confxml"): string(freeBSDGeom),
				},
			},
			wantKey: "ada0p1",
			wantCalls: []fakeHostRunCall{
				{name: "sysctl", args: []string{"-n", "kern.geom.confxml"}},
			},
		},
		{
			name: "openbsd",
			host: &fakeHostOS{
				platform: "openbsd",
				runOutputs: map[string]string{
					fakeRunKey("sysctl", "-n", "hw.disknames"): "sd0:942d2f143e47054f\n",
					fakeRunKey("disklabel", "sd0"):             openBSDDisklabelSD0,
				},
			},
			wantKey: "/dev/sd0a",
			wantCalls: []fakeHostRunCall{
				{name: "sysctl", args: []string{"-n", "hw.disknames"}},
				{name: "disklabel", args: []string{"sd0"}},
			},
		},
		{
			name: "netbsd",
			host: &fakeHostOS{
				platform: "netbsd",
				runOutputs: map[string]string{
					fakeRunKey("sysctl", "-n", "hw.disknames"):                  "ld4\n",
					fakeRunKey("sh", "-c", "disklabel ld4 2>/dev/null || true"): netBSDDisklabelLD4,
					fakeRunKey("dkctl", "ld4", "listwedges"):                    netBSDWedgesLD4,
				},
			},
			wantKey: "/dev/dk0",
			wantCalls: []fakeHostRunCall{
				{name: "sysctl", args: []string{"-n", "hw.disknames"}},
				{name: "sh", args: []string{"-c", "disklabel ld4 2>/dev/null || true"}},
				{name: "dkctl", args: []string{"ld4", "listwedges"}},
			},
		},
		{
			name: "dragonfly",
			host: &fakeHostOS{
				platform: "dragonfly",
				runOutputs: map[string]string{
					fakeRunKey("sysctl", "-n", "kern.disks"): "da0\n",
					fakeRunKey("disklabel", "da0"):           "disklabel: Operation not supported by device\n",
					fakeRunKey("disklabel", "da0s1"):         dragonFlyDisklabelDA0S1,
					fakeRunKey("disklabel", "da0s2"):         "disklabel: Operation not supported by device\n",
					fakeRunKey("disklabel", "da0s3"):         "disklabel: Operation not supported by device\n",
					fakeRunKey("disklabel", "da0s4"):         "disklabel: Operation not supported by device\n",
				},
			},
			wantKey: "/dev/da0s1a",
			wantCalls: []fakeHostRunCall{
				{name: "sysctl", args: []string{"-n", "kern.disks"}},
				{name: "disklabel", args: []string{"da0"}},
				{name: "disklabel", args: []string{"da0s1"}},
				{name: "disklabel", args: []string{"da0s2"}},
				{name: "disklabel", args: []string{"da0s3"}},
				{name: "disklabel", args: []string{"da0s4"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSessionContext(t.Context())
			s.host = tt.host

			got := currentPartitions(s)
			if _, ok := got[tt.wantKey]; !ok {
				t.Fatalf("currentPartitions(%s) = %#v, want key %q", tt.name, got, tt.wantKey)
			}
			if !reflect.DeepEqual(tt.host.runCalls, tt.wantCalls) {
				t.Fatalf("run calls = %#v, want %#v", tt.host.runCalls, tt.wantCalls)
			}
		})
	}
}

func TestParseLinuxFilesystems_sortsAndSkipsPseudoEntries(t *testing.T) {
	input := "nodev\tsysfs\nnodev\tproc\next4\nfuseblk\nxfs\n"

	if got, want := parseLinuxFilesystems(input), []string{"ext4", "xfs"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLinuxFilesystems() = %#v, want %#v", got, want)
	}
}

func TestCurrentLinuxFilesystemsUnreadableProcMatchesRubyResolver(t *testing.T) {
	readFile := func(path string) ([]byte, error) {
		if path != "/proc/filesystems" {
			t.Fatalf("path = %q, want /proc/filesystems", path)
		}
		return nil, os.ErrPermission
	}

	if got := currentFilesystems("linux", readFile, nil); got != nil {
		t.Fatalf("currentFilesystems(linux) = %#v, want nil", got)
	}
}

func TestCurrentDarwinFilesystemsReadsMountOutput(t *testing.T) {
	t.Parallel()

	got := currentFilesystems("darwin", nil, func(name string, args ...string) string {
		if name != "mount" || len(args) != 0 {
			t.Fatalf("run(%q, %#v), want mount", name, args)
		}
		return "/dev/disk3s1s1 on / (apfs, local)\nmap auto_home on /System/Volumes/Data/home (autofs, automounted)\n"
	})
	want := []string{"apfs", "autofs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentFilesystems(darwin) = %#v, want %#v", got, want)
	}
	if got := currentFilesystems("darwin", nil, nil); got != nil {
		t.Fatalf("currentFilesystems(darwin nil runner) = %#v, want nil", got)
	}
}

func TestCurrentFilesystemsHonorsTargetCapabilityPolicy(t *testing.T) {
	called := false
	readFile := func(path string) ([]byte, error) {
		called = true
		return []byte("ext4\n"), nil
	}
	run := func(name string, args ...string) string {
		called = true
		return "/dev/disk on / (apfs, local)\n"
	}

	if got := currentFilesystems("freebsd", readFile, run); got != nil {
		t.Fatalf("currentFilesystems(freebsd) = %#v, want nil", got)
	}
	if called {
		t.Fatal("currentFilesystems(freebsd) touched probes despite target policy")
	}
}

func TestCurrentZFSFactsHonorsTargetCapabilityPolicy(t *testing.T) {
	called := false
	run := func(name string, args ...string) string {
		called = true
		return " 1 initial version\n"
	}

	if got := currentZFSFacts("openbsd", run); got != nil {
		t.Fatalf("currentZFSFacts(openbsd) = %#v, want nil", got)
	}
	if called {
		t.Fatal("currentZFSFacts(openbsd) touched probes despite target policy")
	}
	if got := currentZFSFacts("freebsd", nil); got != nil {
		t.Fatalf("currentZFSFacts(freebsd, nil) = %#v, want nil", got)
	}
}

func TestCurrentZFSFactsRunsZFSAndZpoolUpgradeCommands(t *testing.T) {
	calls := []string{}
	run := func(name string, args ...string) string {
		key := fakeRunKey(name, args...)
		calls = append(calls, key)
		switch key {
		case fakeRunKey("zfs", "upgrade", "-v"):
			return "1 initial version\n2 snapshot version\n"
		case fakeRunKey("zpool", "upgrade", "-v"):
			return "1 initial version\n2 mirror version\n"
		default:
			t.Fatalf("unexpected command %q %#v", name, args)
			return ""
		}
	}

	got := factsByName(currentZFSFacts("freebsd", run))
	want := map[string]any{
		"zfs.feature_numbers":   []string{"1", "2"},
		"zfs.version":           "2",
		"zpool.feature_numbers": []string{"1", "2"},
		"zpool.version":         "2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentZFSFacts() = %#v, want %#v", got, want)
	}
	wantCalls := []string{fakeRunKey("zfs", "upgrade", "-v"), fakeRunKey("zpool", "upgrade", "-v")}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestParseDarwinFilesystems_sortsUniqueFilesystemTypes(t *testing.T) {
	input := "/dev/disk3s1 on / (apfs, local, read-only)\nmap auto_home on /System/Volumes/Data/home (autofs, automounted)\n/dev/disk3s2 on /System/Volumes/Preboot (apfs, local)\n"

	if got, want := parseDarwinFilesystems(input), []string{"apfs", "autofs"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDarwinFilesystems() = %#v, want %#v", got, want)
	}
}

func TestParseDarwinFilesystems_matchesRubyMacOSFixture(t *testing.T) {
	input := strings.Join([]string{
		"/dev/disk1s5 on / (apfs, local, read-only, journaled)",
		"devfs on /dev (devfs, local, nobrowse)",
		"/dev/disk1s1 on /System/Volumes/Data (apfs, local, journaled, nobrowse)",
		"/dev/disk1s4 on /private/var/vm (apfs, local, journaled, nobrowse)",
		"map auto_home on /System/Volumes/Data/home (autofs, automounted, nobrowse)",
		".host:/VMware Shared Folders on /Volumes/VMware Shared Folders (vmhgfs)",
	}, "\n")

	if got, want := parseDarwinFilesystems(input), []string{"apfs", "autofs", "devfs", "vmhgfs"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDarwinFilesystems() = %#v, want %#v", got, want)
	}
}

func TestCoreFacts_includeRootMountpoint(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skipf("mountpoints resolution is not implemented on %s", runtime.GOOS)
	}

	collection := Collection(CoreFacts(testSession, nil))
	mountpoints, ok := collection["mountpoints"].(map[string]any)
	if !ok {
		t.Fatalf("mountpoints fact = %#v, want map", collection["mountpoints"])
	}
	root, ok := mountpoints["/"].(map[string]any)
	if !ok {
		t.Fatalf("mountpoints[/] = %#v, want root mountpoint", mountpoints["/"])
	}

	for _, key := range []string{"available", "available_bytes", "capacity", "size", "size_bytes", "used", "used_bytes"} {
		if root[key] == nil {
			t.Fatalf("mountpoints[/] = %#v, want key %q", root, key)
		}
	}
	if got := ValueForQuery(ResolvedFact{Name: "mountpoints", Value: mountpoints, UserQuery: "mountpoints./.available.something"}); got != nil {
		t.Fatalf("mountpoints./.available.something = %#v, want nil", got)
	}
}

func TestMountpointsFactIncludesDeviceFilesystemAndOptions(t *testing.T) {
	entries := []mountEntry{
		{Device: "/dev/disk1", Path: "/", Filesystem: "apfs", Options: []string{"rw", "local"}},
		{Device: "proc", Path: "/proc", Filesystem: "proc", Options: []string{"rw"}},
		{Device: "tmpfs", Path: "/proc/acpi", Filesystem: "tmpfs", Options: []string{"rw"}},
		{Device: "auto_home", Path: "/home", Filesystem: "autofs", Options: []string{"automounted"}},
	}
	stats := func(path string) (mountStat, bool) {
		if path != "/" && path != "/proc/acpi" {
			return mountStat{}, false
		}
		return mountStat{SizeBytes: 100, AvailableBytes: 25, UsedBytes: 75}, true
	}

	got := mountpointsFact(entries, stats)
	want := map[string]any{
		"/": map[string]any{
			"available":       "25 bytes",
			"available_bytes": 25,
			"capacity":        "75.00%",
			"device":          "/dev/disk1",
			"filesystem":      "apfs",
			"options":         []string{"rw", "local"},
			"size":            "100 bytes",
			"size_bytes":      100,
			"used":            "75 bytes",
			"used_bytes":      75,
		},
		"/proc/acpi": map[string]any{
			"available":       "25 bytes",
			"available_bytes": 25,
			"capacity":        "75.00%",
			"device":          "tmpfs",
			"filesystem":      "tmpfs",
			"options":         []string{"rw"},
			"size":            "100 bytes",
			"size_bytes":      100,
			"used":            "75 bytes",
			"used_bytes":      75,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mountpointsFact() = %#v, want %#v", got, want)
	}
}

func TestMountpointsFactIncludesEmptyOptionsForParsedMountEntries(t *testing.T) {
	got := mountpointsFact([]mountEntry{{Device: "devfs", Path: "/dev", Filesystem: "devfs"}}, func(string) (mountStat, bool) {
		return mountStat{}, false
	})
	mountpoint := got["/dev"].(map[string]any)
	options, ok := mountpoint["options"].([]string)
	if !ok {
		t.Fatalf("options = %#v, want empty []string", mountpoint["options"])
	}
	if len(options) != 0 {
		t.Fatalf("options = %#v, want empty", options)
	}
}

func TestMountpointsFactOmitsEmptyEntries(t *testing.T) {
	t.Parallel()

	got := mountpointsFact([]mountEntry{{Path: "/"}}, func(string) (mountStat, bool) {
		return mountStat{}, false
	})
	if got != nil {
		t.Fatalf("mountpointsFact(empty entry) = %#v, want nil", got)
	}
}

func TestMountpointsFactCapacityMatchesRubyFilesystemHelper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stats mountStat
		want  string
	}{
		{
			name:  "full",
			stats: mountStat{SizeBytes: 100, UsedBytes: 100},
			want:  "100%",
		},
		{
			name:  "empty",
			stats: mountStat{SizeBytes: 100, UsedBytes: 0, AvailableBytes: 100},
			want:  "0%",
		},
		{
			name:  "partial",
			stats: mountStat{SizeBytes: 10_000, UsedBytes: 421, AvailableBytes: 9_579},
			want:  "4.21%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mountpointsFact([]mountEntry{{Path: "/data"}}, func(string) (mountStat, bool) {
				return tt.stats, true
			})
			mountpoint := got["/data"].(map[string]any)
			if mountpoint["capacity"] != tt.want {
				t.Fatalf("capacity = %#v, want %#v", mountpoint["capacity"], tt.want)
			}
		})
	}
}

func TestResolveLinuxRootMountDeviceMatchesRubyResolver(t *testing.T) {
	tests := []struct {
		name    string
		cmdline string
		blkid   string
		want    string
	}{
		{
			name:    "device path",
			cmdline: "console=ttyAMA0 root=/dev/mmcblk0p2 rootfstype=ext4",
			want:    "/dev/mmcblk0p2",
		},
		{
			name:    "missing cmdline root",
			cmdline: "",
			want:    "",
		},
		{
			name:    "partuuid maps through blkid",
			cmdline: "console=tty0 root=PARTUUID=a2f52878-01 rw",
			blkid:   `/dev/xvda1: UUID="f3d" PARTUUID="a2f52878-01"`,
			want:    "/dev/xvda1",
		},
		{
			name:    "partuuid matches exact blkid field",
			cmdline: "console=tty0 root=PARTUUID=a2f52878-01 rw",
			blkid: strings.Join([]string{
				`/dev/xvda1: UUID="not-a2f52878-01" PARTUUID="other"`,
				`/dev/xvdb1: UUID="uuid-root" PARTUUID="a2f52878-01"`,
			}, "\n"),
			want: "/dev/xvdb1",
		},
		{
			name:    "partuuid remains when blkid cannot map",
			cmdline: "console=tty0 root=PARTUUID=a2f52878-01 rw",
			blkid:   "blkid: command not found",
			want:    "PARTUUID=a2f52878-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readFile := func(path string) ([]byte, error) {
				if path != "/proc/cmdline" {
					t.Fatalf("readFile path = %q, want /proc/cmdline", path)
				}
				return []byte(tt.cmdline), nil
			}
			run := func(name string, args ...string) string {
				if name != "blkid" || len(args) != 0 {
					t.Fatalf("run = %q %#v, want blkid", name, args)
				}
				return tt.blkid
			}

			got := resolveLinuxRootMountDevice(readFile, run)
			if got != tt.want {
				t.Fatalf("resolveLinuxRootMountDevice() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLinuxMountEntriesReplaceDevRootLikeRubyResolver(t *testing.T) {
	entries := []mountEntry{{Device: "/dev/root", Path: "/", Filesystem: "ext4", Options: []string{"rw", "noatime"}}}
	readFile := func(path string) ([]byte, error) {
		return []byte("console=ttyAMA0 root=/dev/mmcblk0p2 rootfstype=ext4"), nil
	}
	run := func(name string, args ...string) string { return "" }

	got := linuxMountEntriesWithRootDevice(entries, readFile, run)
	if got[0].Device != "/dev/mmcblk0p2" {
		t.Fatalf("device = %q, want /dev/mmcblk0p2", got[0].Device)
	}
}

func TestCurrentMountEntriesLinuxReadsProcMountsAndResolvesDevRoot(t *testing.T) {
	host := &fakeHostOS{
		platform: "linux",
		files: map[string][]byte{
			"/proc/self/mounts": []byte("/dev/root / ext4 rw,noatime 0 0\n"),
			"/proc/cmdline":     []byte("console=ttyAMA0 root=/dev/mmcblk0p2 rootfstype=ext4"),
		},
	}
	s := NewSession()
	s.host = host

	got := currentMountEntries(s)
	want := []mountEntry{{Device: "/dev/mmcblk0p2", Path: "/", Filesystem: "ext4", Options: []string{"rw", "noatime"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentMountEntries() = %#v, want %#v", got, want)
	}
	if len(host.runCalls) != 0 {
		t.Fatalf("run calls = %#v, want none", host.runCalls)
	}
}

func TestCurrentMountEntriesParsesPlatformMountOutput(t *testing.T) {
	tests := []struct {
		goos      string
		outputs   map[string]string
		want      []mountEntry
		wantCalls []fakeHostRunCall
	}{
		{
			goos: "darwin",
			outputs: map[string]string{
				fakeRunKey("mount"): "/dev/disk3s1s1 on / (apfs, sealed, local, read-only, journaled)\n",
			},
			want:      []mountEntry{{Device: "/dev/disk3s1s1", Path: "/", Filesystem: "apfs", Options: []string{"sealed", "local", "readonly", "journaled"}}},
			wantCalls: []fakeHostRunCall{{name: "mount"}},
		},
		{
			goos: "freebsd",
			outputs: map[string]string{
				fakeRunKey("mount"): "/dev/ada0p2 on / (ufs, local, journaled soft-updates)\n",
			},
			want:      []mountEntry{{Device: "/dev/ada0p2", Path: "/", Filesystem: "ufs", Options: []string{"local", "journaled soft-updates"}}},
			wantCalls: []fakeHostRunCall{{name: "mount"}},
		},
		{
			goos: "netbsd",
			outputs: map[string]string{
				fakeRunKey("mount"): "/dev/dk1 on / type ffs (noatime, local)\n",
			},
			want:      []mountEntry{{Device: "/dev/dk1", Path: "/", Filesystem: "ffs", Options: []string{"noatime", "local"}}},
			wantCalls: []fakeHostRunCall{{name: "mount"}},
		},
		{
			goos: "plan9",
			want: []mountEntry{{Path: "/"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			s := NewSession()
			host := &fakeHostOS{platform: tt.goos, runOutputs: tt.outputs}
			s.host = host

			got := currentMountEntries(s)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("currentMountEntries(%s) = %#v, want %#v", tt.goos, got, tt.want)
			}
			if !reflect.DeepEqual(host.runCalls, tt.wantCalls) {
				t.Fatalf("run calls = %#v, want %#v", host.runCalls, tt.wantCalls)
			}
		})
	}
}

func TestCurrentMountEntriesOmitWhenPlatformSourceUnavailable(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"darwin", "freebsd", "netbsd", "linux"} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()

			s := NewSession()
			host := &fakeHostOS{platform: goos}
			switch goos {
			case "darwin", "freebsd", "netbsd":
				host.runOutputs = map[string]string{fakeRunKey("mount"): ""}
			}
			s.host = host
			if got := currentMountEntries(s); got != nil {
				t.Fatalf("currentMountEntries(%s) = %#v, want nil", goos, got)
			}
		})
	}
}

func TestRootMountpointUsesPlatformSpecificMountCommands(t *testing.T) {
	tests := []struct {
		goos           string
		outputs        map[string]string
		wantDevice     string
		wantFilesystem string
		wantSizeBytes  int
		wantUsedBytes  int
		wantAvailBytes int
		wantCapacity   string
		wantOptions    []string
		wantCalls      []fakeHostRunCall
	}{
		{
			goos: "openbsd",
			outputs: map[string]string{
				fakeRunKey("mount"):    "/dev/sd0a on / type ffs (local)\n",
				fakeRunKey("df", "-P"): "Filesystem 512-blocks Used Available Capacity Mounted on\n/dev/sd0a 2000 1000 1000 50% /\n",
			},
			wantDevice:     "/dev/sd0a",
			wantFilesystem: "ffs",
			wantSizeBytes:  1_024_000,
			wantUsedBytes:  512_000,
			wantAvailBytes: 512_000,
			wantCapacity:   "50.00%",
			wantOptions:    []string{"local"},
			wantCalls: []fakeHostRunCall{
				{name: "mount"},
				{name: "df", args: []string{"-P"}},
			},
		},
		{
			goos: "netbsd",
			outputs: map[string]string{
				fakeRunKey("mount"):    "/dev/dk1 on / type ffs (noatime, local)\n",
				fakeRunKey("df", "-P"): "Filesystem 512-blocks Used Avail Capacity Mounted on\n/dev/dk1 4000 1000 3000 25% /\n",
			},
			wantDevice:     "/dev/dk1",
			wantFilesystem: "ffs",
			wantSizeBytes:  2_048_000,
			wantUsedBytes:  512_000,
			wantAvailBytes: 1_536_000,
			wantCapacity:   "25.00%",
			wantOptions:    []string{"noatime", "local"},
			wantCalls: []fakeHostRunCall{
				{name: "mount"},
				{name: "df", args: []string{"-P"}},
			},
		},
		{
			goos: "dragonfly",
			outputs: map[string]string{
				fakeRunKey("mount"):    "da0s1d on / (hammer2, local)\n",
				fakeRunKey("df", "-P"): "Filesystem 512-blocks Used Avail Capacity Mounted on\nda0s1d 8000 2000 6000 25% /\n",
			},
			wantDevice:     "/dev/da0s1d",
			wantFilesystem: "hammer2",
			wantSizeBytes:  4_096_000,
			wantUsedBytes:  1_024_000,
			wantAvailBytes: 3_072_000,
			wantCapacity:   "25.00%",
			wantOptions:    []string{"local"},
			wantCalls: []fakeHostRunCall{
				{name: "mount"},
				{name: "df", args: []string{"-P"}},
			},
		},
		{
			goos: "illumos",
			outputs: map[string]string{
				fakeRunKey("mount", "-v"): "rpool/ROOT/test on / type zfs read/write/setuid/devices/dev=4310002 on Thu Jan  1 00:00:00 1970\n",
				fakeRunKey("df", "-P"):    "Filesystem 512-blocks Used Available Capacity Mounted on\nrpool/ROOT/test 16000 4000 12000 25% /\n",
			},
			wantDevice:     "rpool/ROOT/test",
			wantFilesystem: "zfs",
			wantSizeBytes:  8_192_000,
			wantUsedBytes:  2_048_000,
			wantAvailBytes: 6_144_000,
			wantCapacity:   "25.00%",
			wantOptions:    []string{"read", "write", "setuid", "devices", "dev=4310002"},
			wantCalls: []fakeHostRunCall{
				{name: "mount", args: []string{"-v"}},
				{name: "df", args: []string{"-P"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			host := &fakeHostOS{platform: tt.goos, runOutputs: tt.outputs}
			s := NewSession()
			s.host = host

			got := rootMountpoint(s)
			root, ok := got["/"].(map[string]any)
			if !ok {
				t.Fatalf("rootMountpoint(%s) = %#v, want / map", tt.goos, got)
			}
			if root["device"] != tt.wantDevice || root["filesystem"] != tt.wantFilesystem || root["size_bytes"] != tt.wantSizeBytes {
				t.Fatalf("root mountpoint = %#v, want device %q filesystem %q size_bytes %d", root, tt.wantDevice, tt.wantFilesystem, tt.wantSizeBytes)
			}
			if root["used_bytes"] != tt.wantUsedBytes || root["available_bytes"] != tt.wantAvailBytes || root["capacity"] != tt.wantCapacity {
				t.Fatalf("root mountpoint df fields = %#v, want used_bytes %d available_bytes %d capacity %q", root, tt.wantUsedBytes, tt.wantAvailBytes, tt.wantCapacity)
			}
			if !reflect.DeepEqual(root["options"], tt.wantOptions) {
				t.Fatalf("root mountpoint options = %#v, want %#v", root["options"], tt.wantOptions)
			}
			if !reflect.DeepEqual(host.runCalls, tt.wantCalls) {
				t.Fatalf("run calls = %#v, want %#v", host.runCalls, tt.wantCalls)
			}
		})
	}
}

func TestCurrentBSDAndIllumosMountpointsFallbackToRootStatWhenMountOutputMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*Session) map[string]any
	}{
		{name: "dragonfly", run: currentDragonFlyMountpoints},
		{name: "illumos", run: currentIllumosMountpoints},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host := &fakeHostOS{
				platform: tt.name,
				runOutputs: map[string]string{
					fakeRunKey("mount"):       "",
					fakeRunKey("mount", "-v"): "",
				},
				mountStats: map[string]mountStat{
					"/": {SizeBytes: 2048, UsedBytes: 1024, AvailableBytes: 1024},
				},
			}
			s := NewSession()
			s.host = host

			got := tt.run(s)
			root, ok := got["/"].(map[string]any)
			if !ok {
				t.Fatalf("%s mountpoints = %#v, want / mountpoint", tt.name, got)
			}
			if root["size_bytes"] != 2048 || root["used_bytes"] != 1024 || root["available_bytes"] != 1024 || root["capacity"] != "50.00%" {
				t.Fatalf("%s root mountpoint = %#v, want stat-derived bytes and capacity", tt.name, root)
			}
			if want := []string{"/"}; !reflect.DeepEqual(host.statMountpointCalls, want) {
				t.Fatalf("%s statMountpoint calls = %#v, want %#v", tt.name, host.statMountpointCalls, want)
			}
		})
	}
}

func TestDarwinMountpointsFactUsesZeroDefaultsWhenStatFails(t *testing.T) {
	entries := []mountEntry{{Device: "/dev/root", Path: "/", Filesystem: "ext4", Options: []string{"rw", "noatime"}}}
	stats := func(string) (mountStat, bool) { return mountStat{}, false }

	got := darwinMountpointsFact(entries, stats)
	want := map[string]any{
		"/": map[string]any{
			"available":       "0 bytes",
			"available_bytes": 0,
			"capacity":        "100%",
			"device":          "/dev/root",
			"filesystem":      "ext4",
			"options":         []string{"rw", "noatime"},
			"size":            "0 bytes",
			"size_bytes":      0,
			"used":            "0 bytes",
			"used_bytes":      0,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("darwinMountpointsFact() = %#v, want %#v", got, want)
	}
}

func TestFreeBSDMountpointsFactParsesMountOutput(t *testing.T) {
	mountOutput := `/dev/ada0p2 on / (ufs, local, journaled soft-updates)
devfs on /dev (devfs)
tmpfs on /tmp/example path (tmpfs, local, nosuid)
`
	stats := func(path string) (mountStat, bool) {
		if path != "/" {
			return mountStat{}, false
		}
		return mountStat{SizeBytes: 466_449_743_872, AvailableBytes: 67_979_685_888, UsedBytes: 374_704_357_376}, true
	}

	got := mountpointsFact(parseFreeBSDMountEntries(mountOutput), stats)
	want := map[string]any{
		"/": map[string]any{
			"available":       "63.31 GiB",
			"available_bytes": 67_979_685_888,
			"capacity":        "84.64%",
			"device":          "/dev/ada0p2",
			"filesystem":      "ufs",
			"options":         []string{"local", "journaled soft-updates"},
			"size":            "434.42 GiB",
			"size_bytes":      466_449_743_872,
			"used":            "348.97 GiB",
			"used_bytes":      374_704_357_376,
		},
		"/dev": map[string]any{
			"device":     "devfs",
			"filesystem": "devfs",
			"options":    []string{},
		},
		"/tmp/example path": map[string]any{
			"device":     "tmpfs",
			"filesystem": "tmpfs",
			"options":    []string{"local", "nosuid"},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mountpointsFact(parseFreeBSDMountEntries()) = %#v, want %#v", got, want)
	}
}

func TestOpenBSDMountpointsFactParsesMountAndDFOutput(t *testing.T) {
	mountOutput := `/dev/sd0a on / type ffs (local)
/dev/sd0d on /usr type ffs (local, nodev)
/dev/sd0e on /usr/local type ffs (local, nodev, wxallowed)
`
	dfOutput := `Filesystem   512-blocks       Used   Available Capacity Mounted on
/dev/sd0a       2018844     404488     1513416    21%   /
/dev/sd0d       2018844    1595216      322688    83%   /usr
/dev/sd0e       6082908    3477752     2301012    60%   /usr/local
`

	got := openBSDMountpointsFact(mountOutput, dfOutput)
	want := map[string]any{
		"/": map[string]any{
			"available":       "738.97 MiB",
			"available_bytes": 774_868_992,
			"capacity":        "21.09%",
			"device":          "/dev/sd0a",
			"filesystem":      "ffs",
			"options":         []string{"local"},
			"size":            "985.76 MiB",
			"size_bytes":      1_033_648_128,
			"used":            "197.50 MiB",
			"used_bytes":      207_097_856,
		},
		"/usr": map[string]any{
			"available":       "157.56 MiB",
			"available_bytes": 165_216_256,
			"capacity":        "83.17%",
			"device":          "/dev/sd0d",
			"filesystem":      "ffs",
			"options":         []string{"local", "nodev"},
			"size":            "985.76 MiB",
			"size_bytes":      1_033_648_128,
			"used":            "778.91 MiB",
			"used_bytes":      816_750_592,
		},
		"/usr/local": map[string]any{
			"available":       "1.10 GiB",
			"available_bytes": 1_178_118_144,
			"capacity":        "60.18%",
			"device":          "/dev/sd0e",
			"filesystem":      "ffs",
			"options":         []string{"local", "nodev", "wxallowed"},
			"size":            "2.90 GiB",
			"size_bytes":      3_114_448_896,
			"used":            "1.66 GiB",
			"used_bytes":      1_780_609_024,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("openBSDMountpointsFact() = %#v, want %#v", got, want)
	}
}

func TestNetBSDMountpointsFactParsesMountAndDFOutput(t *testing.T) {
	mountOutput := `/dev/dk1 on / type ffs (noatime, local)
/dev/dk0 on /boot type msdos (local)
ptyfs on /dev/pts type ptyfs (local)
`
	dfOutput := `Filesystem  512-blocks      Used     Avail Capacity Mounted on
/dev/dk1       40891628   5550832  33296216    14%   /
/dev/dk0         261120     13568    247552     5%   /boot
`

	got := netBSDMountpointsFact(mountOutput, dfOutput)
	root := got["/"].(map[string]any)
	for key, want := range map[string]any{
		"available":       "15.88 GiB",
		"available_bytes": 17_047_662_592,
		"capacity":        "14.29%",
		"size":            "19.50 GiB",
		"size_bytes":      20_936_513_536,
		"used":            "2.65 GiB",
		"used_bytes":      2_842_025_984,
	} {
		if root[key] != want {
			t.Fatalf("root[%s] = %#v, want %#v", key, root[key], want)
		}
	}
	devpts := got["/dev/pts"].(map[string]any)
	if _, ok := devpts["size_bytes"]; ok {
		t.Fatalf("/dev/pts size_bytes = %#v, want omitted without df row", devpts["size_bytes"])
	}
}

func TestDragonFlyMountpointsFactParsesMountAndDFOutput(t *testing.T) {
	mountOutput := `da0s1d on / (hammer2, local)
devfs on /dev (devfs, nosymfollow, local)
/dev/da0s1a on /boot (ufs, local)`
	dfOutput := `Filesystem  512-blocks    Used     Avail Capacity  Mounted on
da0s1d       247916160 2892416 245023744     1%    /
devfs                2       2         0   100%    /dev
/dev/da0s1a    1548188 1315440    108896    92%    /boot`

	got := dragonFlyMountpointsFact(mountOutput, dfOutput)
	root := got["/"].(map[string]any)
	if root["device"] != "/dev/da0s1d" || root["filesystem"] != "hammer2" || root["size_bytes"] != 126_933_073_920 {
		t.Fatalf("dragonFlyMountpointsFact()[/] = %#v", root)
	}
}

func TestCurrentDragonFlyMountpointsUsesMountAndDFOutput(t *testing.T) {
	t.Parallel()

	s := NewSession()
	s.host = &fakeHostOS{runOutputs: map[string]string{
		fakeRunKey("mount"): "da0s1d on / (hammer2, local)\n",
		fakeRunKey("df", "-P"): `Filesystem  512-blocks    Used     Avail Capacity  Mounted on
da0s1d       247916160 2892416 245023744     1%    /
`,
	}}

	got := currentDragonFlyMountpoints(s)
	root := got["/"].(map[string]any)
	if root["device"] != "/dev/da0s1d" || root["filesystem"] != "hammer2" || root["size_bytes"] != 126_933_073_920 {
		t.Fatalf("currentDragonFlyMountpoints()[/] = %#v", root)
	}
}

func TestDragonFlyMountDeviceOnlyNormalizesDiskPartitions(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "da0s1d", want: "/dev/da0s1d"},
		{in: "nvme0s1a", want: "/dev/nvme0s1a"},
		{in: "/dev/da0s1a", want: "/dev/da0s1a"},
		{in: "devfs", want: "devfs"},
		{in: "host10s1:/export", want: "host10s1:/export"},
	}

	for _, tt := range tests {
		if got := dragonFlyMountDevice(tt.in); got != tt.want {
			t.Fatalf("dragonFlyMountDevice(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIllumosMountpointsFactParsesMountAndDFOutput(t *testing.T) {
	mountOutput := `rpool/ROOT/omnios-r151058 on / type zfs read/write/setuid/devices/dev=4310002 on Thu Jan  1 00:00:00 1970
swap on /tmp type tmpfs read/write/setuid/devices/xattr/dev=8b80002 on Fri Jun 19 19:05:19 2026`
	dfOutput := `Filesystem            512-blocks        Used   Available Capacity  Mounted on
rpool/ROOT/omnios-r151058    59932672     1902744    57629848     4%    /
swap                     5046424      133048     4913376     3%    /tmp`

	got := illumosMountpointsFact(mountOutput, dfOutput)
	root := got["/"].(map[string]any)
	if root["device"] != "rpool/ROOT/omnios-r151058" || root["filesystem"] != "zfs" || root["size_bytes"] != 30_685_528_064 {
		t.Fatalf("illumosMountpointsFact()[/] = %#v", root)
	}
}

func TestCurrentIllumosMountpointsUsesMountAndDFOutput(t *testing.T) {
	t.Parallel()

	s := NewSession()
	s.host = &fakeHostOS{runOutputs: map[string]string{
		fakeRunKey("mount", "-v"): "rpool/ROOT/omnios-r151058 on / type zfs read/write/setuid on Thu Jan  1 00:00:00 1970\n",
		fakeRunKey("df", "-P"): `Filesystem            512-blocks        Used   Available Capacity  Mounted on
rpool/ROOT/omnios-r151058    59932672     1902744    57629848     4%    /
`,
	}}

	got := currentIllumosMountpoints(s)
	root := got["/"].(map[string]any)
	if root["device"] != "rpool/ROOT/omnios-r151058" || root["filesystem"] != "zfs" || root["size_bytes"] != 30_685_528_064 {
		t.Fatalf("currentIllumosMountpoints()[/] = %#v", root)
	}
}

func TestCurrentBSDStyleMountpointsFallbackToRootStat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		outputs map[string]string
		current func(*Session) map[string]any
	}{
		{
			name:    "dragonfly",
			outputs: map[string]string{fakeRunKey("mount"): ""},
			current: currentDragonFlyMountpoints,
		},
		{
			name:    "illumos",
			outputs: map[string]string{fakeRunKey("mount", "-v"): ""},
			current: currentIllumosMountpoints,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			host := &fakeHostOS{
				runOutputs: tt.outputs,
				mountStats: map[string]mountStat{
					"/": {SizeBytes: 100, AvailableBytes: 25, UsedBytes: 75},
				},
			}
			s := NewSession()
			s.host = host

			got := tt.current(s)
			root := got["/"].(map[string]any)
			if root["size_bytes"] != 100 || root["available_bytes"] != 25 || root["used_bytes"] != 75 {
				t.Fatalf("current mountpoints fallback root = %#v", root)
			}
			if want := []string{"/"}; !reflect.DeepEqual(host.statMountpointCalls, want) {
				t.Fatalf("statMountpoint calls = %#v, want %#v", host.statMountpointCalls, want)
			}
		})
	}
}

func TestParseNetBSDMountEntries(t *testing.T) {
	input := `/dev/dk1 on / type ffs (noatime, local)
/dev/dk0 on /boot type msdos (local)
ptyfs on /dev/pts type ptyfs (local)
`

	got := parseBSDMountEntries(input)
	want := []mountEntry{
		{Device: "/dev/dk1", Path: "/", Filesystem: "ffs", Options: []string{"noatime", "local"}},
		{Device: "/dev/dk0", Path: "/boot", Filesystem: "msdos", Options: []string{"local"}},
		{Device: "ptyfs", Path: "/dev/pts", Filesystem: "ptyfs", Options: []string{"local"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseBSDMountEntries() = %#v, want %#v", got, want)
	}
}

func TestParseBSDMountEntriesHandlesFreeBSDStyleFilesystemOptions(t *testing.T) {
	t.Parallel()

	input := `/dev/gpt/rootfs on / (ufs, local, journaled soft-updates)
malformed without separator
/dev/gpt/bad on /bad
`

	got := parseBSDMountEntries(input)
	want := []mountEntry{
		{Device: "/dev/gpt/rootfs", Path: "/", Filesystem: "ufs", Options: []string{"local", "journaled soft-updates"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseBSDMountEntries() = %#v, want %#v", got, want)
	}
}

func TestParseIllumosMountEntriesHandlesLegacyAndTypedFormats(t *testing.T) {
	t.Parallel()

	input := `rpool/ROOT/omnios-r151058 on / type zfs read/write/setuid/devices/dev=4310002 on Thu Jan  1 00:00:00 1970
/ on rpool/ROOT/omnios-r151058 read/write/setuid
bad line
/missing on onlyonefield
`

	got := parseIllumosMountEntries(input)
	want := []mountEntry{
		{Device: "rpool/ROOT/omnios-r151058", Path: "/", Filesystem: "zfs", Options: []string{"read", "write", "setuid", "devices", "dev=4310002"}},
		{Device: "rpool/ROOT/omnios-r151058", Path: "/", Options: []string{"read", "write", "setuid"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseIllumosMountEntries() = %#v, want %#v", got, want)
	}
}

func TestParseDarwinMountEntries(t *testing.T) {
	input := "/dev/disk3s1s1 on / (apfs, sealed, local, read-only, journaled)\nmap auto_home on /System/Volumes/Data/home (autofs, automounted, nobrowse)\nserver:/Shared\\040Data on /Volumes/Shared\\040Data (nfs, nodev, nosuid)\n"

	got := parseDarwinMountEntries(input)
	want := []mountEntry{
		{Device: "/dev/disk3s1s1", Path: "/", Filesystem: "apfs", Options: []string{"sealed", "local", "readonly", "journaled"}},
		{Device: "map auto_home", Path: "/System/Volumes/Data/home", Filesystem: "autofs", Options: []string{"automounted", "nobrowse"}},
		{Device: "server:/Shared Data", Path: "/Volumes/Shared Data", Filesystem: "nfs", Options: []string{"nodev", "nosuid"}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDarwinMountEntries() = %#v, want %#v", got, want)
	}
}

func TestParseDarwinMountEntriesNormalizesRubyOptionAliases(t *testing.T) {
	input := "/dev/disk3s1 on / (apfs, read-only, asynchronous, synchronous, quotas, rootfs, defwrite, nodev)\n"

	got := parseDarwinMountEntries(input)
	want := []mountEntry{{
		Device:     "/dev/disk3s1",
		Path:       "/",
		Filesystem: "apfs",
		Options:    []string{"readonly", "async", "noasync", "quota", "root", "deferwrites", "nodev"},
	}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDarwinMountEntries() = %#v, want %#v", got, want)
	}
}
