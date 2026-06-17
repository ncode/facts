# OpenBSD and NetBSD VM Probe Fixtures

Captured from local `.local/bsd-vms` QEMU guests on 2026-06-17. The VM images
are local scratch artifacts and are intentionally ignored.

## OpenBSD 7.9 arm64

Command set:

```sh
uname -a
uname -s
uname -r
uname -m
uname -p
sysctl hw.model hw.ncpu hw.vendor hw.product hw.version hw.physmem vm.loadavg kern.boottime
sysctl hw.cpuspeed hw.serialno hw.uuid
vmstat -s
swapctl -sk
mount
df -P
route -n get default
dhcpleasectl -l vio0
```

Output:

```text
OpenBSD openbsd.local 7.9 GENERIC.MP#2503 arm64
OpenBSD
7.9
arm64
aarch64
hw.model=Unknown
hw.ncpu=2
hw.vendor=QEMU
hw.product=QEMU Virtual Machine
hw.version=virt-11.0
hw.physmem=1065725952
vm.loadavg=0.56 0.20 0.08
kern.boottime=Wed Jun 17 21:13:32 2026
sysctl: hw.cpuspeed: value is not available
sysctl: hw.serialno: value is not available
sysctl: hw.uuid: value is not available
       4096 bytes per page
     241757 pages managed
      50327 pages free
       7715 pages active
     117925 pages inactive
          2 pages wired
total: 262144 1K-blocks allocated, 12664 used, 249480 available
/dev/sd0a on / type ffs (local)
/dev/sd0e on /home type ffs (local, nodev, nosuid)
/dev/sd0d on /usr type ffs (local, nodev, wxallowed)
Filesystem  512-blocks    Used   Avail Capacity  Mounted on
/dev/sd0a      4661880  446664 3982128    11%    /
/dev/sd0e      8101752       8 7696664     1%    /home
/dev/sd0d     12165816 5013736 6543792    44%    /usr
   route to: default
destination: default
       mask: default
    gateway: 10.0.2.2
  interface: vio0
    if address: 10.0.2.15
dhcp server 10.0.2.2
```

`df -P` was also captured in 1K-block mode for parser comparison:

```text
Filesystem  1024-blocks    Used   Avail Capacity  Mounted on
/dev/sd0a       2330940  223332 1991064    11%    /
/dev/sd0e       4050876       4 3848332     1%    /home
/dev/sd0d       6082908 2506868 3271896    44%    /usr
```

## NetBSD 10.1 arm64

Command set:

```sh
PATH=/sbin:/usr/sbin:/bin:/usr/bin
uname -a
uname -s
uname -r
uname -m
uname -p
sysctl hw.model hw.ncpu hw.cpuspeed hw.physmem64 hw.physmem vm.loadavg kern.boottime
sysctl machdep.dmi.system-vendor machdep.dmi.system-product machdep.dmi.system-version machdep.dmi.system-serial machdep.dmi.system-uuid
vmstat -s
swapctl -sk
mount
df -P
route -n get default
```

Output:

```text
NetBSD netbsd.local 10.1 NetBSD 10.1 (GENERIC64) #0: Mon Dec 16 13:08:11 UTC 2024  mkrepro@mkrepro.NetBSD.org:/usr/src/sys/arch/evbarm/compile/GENERIC64 evbarm
NetBSD
10.1
evbarm
aarch64
hw.model = netbsd,generic-acpi
hw.ncpu = 2
sysctl: second level name cpuspeed in hw.cpuspeed is invalid
hw.physmem64 = 1049231360
hw.physmem = 1049231360
vm.loadavg = 0.00 0.00 0.00
kern.boottime = 1781723607
machdep.dmi.system-vendor = QEMU
machdep.dmi.system-product = QEMU Virtual Machine
machdep.dmi.system-version = virt-11.0
sysctl: second level name system-serial in machdep.dmi.system-serial is invalid
machdep.dmi.system-uuid = 00000000-0000-0000-0000-000000000000
     4096 bytes per page
   247459 pages managed
   210994 pages free
    19007 pages active
        0 pages inactive
     4424 pages wired
no swap devices configured
/dev/dk1 on / type ffs (noatime, local)
/dev/dk0 on /boot type msdos (local)
ptyfs on /dev/pts type ptyfs (local)
procfs on /proc type procfs (local)
tmpfs on /var/shm type tmpfs (local)
Filesystem  1024-blocks     Used    Avail Capacity  Mounted on
/dev/dk1       20402032  2744672 16637264    14%    /
/dev/dk0         162538    66528    96010    40%    /boot
ptyfs                 2        2        0   100%    /dev/pts
procfs                8        8        0   100%    /proc
tmpfs            512320        8   512312     0%    /var/shm
   route to: default
destination: default
       mask: default
    gateway: 10.0.2.2
  interface: vioif0
 local addr: 10.0.2.15
```

NetBSD DHCP lease data was present only in `dhcpcd` binary lease files in the
stock guest, so the first implementation pass should keep DHCP absent unless a
stable text source is selected in task 1.3.
