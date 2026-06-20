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

	got := discoverPartitions(root)
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

	got := currentLinuxPartitions(root, run)
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

	got := discoverPartitions(root)
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

func TestCurrentOpenBSDPartitionsReadsEveryDiskName(t *testing.T) {
	got := currentOpenBSDPartitions(func(name string, args ...string) string {
		switch strings.Join(append([]string{name}, args...), " ") {
		case "sysctl -n hw.disknames":
			return "sd0:942d2f143e47054f,sd1:1111111111111111\n"
		case "disklabel sd0":
			return openBSDDisklabelSD0
		case "disklabel sd1":
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
			})
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
	})
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

	collection := Collection(CoreFacts(testSession))
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

	got := disksFact(dir)
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

	got := currentLinuxDisks(dir, run)
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

	collection := Collection(CoreFacts(testSession))
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
