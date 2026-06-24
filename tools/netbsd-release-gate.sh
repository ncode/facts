#!/bin/sh
# NetBSD release gate: verifies the NetBSD release-gate fact set through a
# built facts binary. CI and local NetBSD smoke tests run this same script.
#
# Usage: netbsd-release-gate.sh [path-to-facts-binary]
set -eu

PATH=/sbin:/usr/sbin:/bin:/usr/bin:/usr/pkg/sbin:/usr/pkg/bin
export PATH

FACTS_BIN="${1:-./facts}"

if [ "$(uname -s)" != "NetBSD" ]; then
    echo "netbsd-release-gate.sh must run on NetBSD" >&2
    exit 1
fi

FACT_SET="os.name os.family os.release os.architecture os.hardware kernel.name \
kernel.release.full kernel.version.full kernel.release.major virtual is_virtual networking \
memory memory.system.total processors processors.count dmi system_uptime \
load_averages mountpoints disks partitions"

# shellcheck disable=SC2086
out="$("$FACTS_BIN" --json $FACT_SET)"
printf '%s\n' "$out"

fail() {
    echo "netbsd-release-gate: $1" >&2
    exit 1
}

printf '%s\n' "$out" | grep -Eq '"os.name"[[:space:]]*:[[:space:]]*"NetBSD"' \
    || fail 'os.name != "NetBSD"'
printf '%s\n' "$out" | grep -Eq '"kernel.name"[[:space:]]*:[[:space:]]*"NetBSD"' \
    || fail 'kernel.name != "NetBSD"'
printf '%s\n' "$out" | grep -Eq '"os.family"[[:space:]]*:[[:space:]]*"NetBSD"' \
    || fail 'os.family != "NetBSD"'

for key in os.release os.architecture os.hardware kernel.release.full kernel.version.full \
    kernel.release.major virtual is_virtual networking memory memory.system.total \
    processors processors.count dmi system_uptime load_averages \
    mountpoints disks partitions; do
    key_re=$(printf '%s\n' "$key" | sed 's/\./\\./g')
    printf '%s\n' "$out" | grep -Eq "\"$key_re\"[[:space:]]*:" \
        || fail "missing fact $key"
    printf '%s\n' "$out" | grep -Eq "\"$key_re\"[[:space:]]*:[[:space:]]*null" \
        && fail "fact $key is null"
done

echo "netbsd-release-gate: all facts present and non-null"
