#!/bin/sh
# OpenBSD release gate: verifies the OpenBSD release-gate fact set through a
# built facts binary. CI and local OpenBSD smoke tests run this same script.
#
# Usage: openbsd-release-gate.sh [path-to-facts-binary]
set -eu

PATH=/sbin:/usr/sbin:/bin:/usr/bin:/usr/local/sbin:/usr/local/bin
export PATH

FACTS_BIN="${1:-./facts}"

if [ "$(uname -s)" != "OpenBSD" ]; then
    echo "openbsd-release-gate.sh must run on OpenBSD" >&2
    exit 1
fi

FACT_SET="os.name os.family os.release os.architecture os.hardware kernel \
kernelrelease kernelversion kernelmajversion virtual is_virtual networking \
memory memory.system.total processors processors.count dmi system_uptime \
load_averages mountpoints disks partitions"

# shellcheck disable=SC2086
out="$("$FACTS_BIN" --json $FACT_SET)"
printf '%s\n' "$out"

fail() {
    echo "openbsd-release-gate: $1" >&2
    exit 1
}

printf '%s\n' "$out" | grep -Eq '"os.name"[[:space:]]*:[[:space:]]*"OpenBSD"' \
    || fail 'os.name != "OpenBSD"'
printf '%s\n' "$out" | grep -Eq '"kernel"[[:space:]]*:[[:space:]]*"OpenBSD"' \
    || fail 'kernel != "OpenBSD"'
printf '%s\n' "$out" | grep -Eq '"os.family"[[:space:]]*:[[:space:]]*"OpenBSD"' \
    || fail 'os.family != "OpenBSD"'

for key in os.release os.architecture os.hardware kernelrelease kernelversion \
    kernelmajversion virtual is_virtual networking memory memory.system.total \
    processors processors.count dmi system_uptime load_averages \
    mountpoints disks partitions; do
    printf '%s\n' "$out" | grep -Eq "\"$key\"[[:space:]]*:" \
        || fail "missing fact $key"
    printf '%s\n' "$out" | grep -Eq "\"$key\"[[:space:]]*:[[:space:]]*null" \
        && fail "fact $key is null"
done

echo "openbsd-release-gate: all facts present and non-null"
