#!/bin/sh
# DragonFly release gate: verifies the DragonFly candidate fact set through a
# built facts binary. Local wrappers and CI/asserted native gates run this same
# script so the checked fact set stays aligned.
#
# Usage: dragonfly-release-gate.sh [path-to-facts-binary]
set -eu

PATH=/sbin:/usr/sbin:/bin:/usr/bin:/usr/local/sbin:/usr/local/bin
export PATH

FACTS_BIN="${1:-./facts}"

if [ "$(uname -s)" != "DragonFly" ]; then
    echo "dragonfly-release-gate.sh must run on DragonFly" >&2
    exit 1
fi

FACT_SET="os.name os.family os.release os.architecture os.hardware kernel.name \
kernel.release.full kernel.version.full kernel.release.major virtual is_virtual \
networking memory memory.system.total processors processors.count system_uptime \
load_averages mountpoints disks partitions path"

# shellcheck disable=SC2086
out="$("$FACTS_BIN" --json $FACT_SET)"
printf '%s\n' "$out"

fail() {
    echo "dragonfly-release-gate: $1" >&2
    exit 1
}

escape_ere() {
    printf '%s\n' "$1" | sed 's/[][\\.^$*+?{}()|]/\\&/g'
}

printf '%s\n' "$out" | grep -Eq '"os.name"[[:space:]]*:[[:space:]]*"DragonFly"' \
    || fail 'os.name != "DragonFly"'
printf '%s\n' "$out" | grep -Eq '"kernel.name"[[:space:]]*:[[:space:]]*"DragonFly"' \
    || fail 'kernel.name != "DragonFly"'
printf '%s\n' "$out" | grep -Eq '"os.family"[[:space:]]*:[[:space:]]*"DragonFly"' \
    || fail 'os.family != "DragonFly"'

for key in os.release os.architecture os.hardware kernel.release.full kernel.version.full \
    kernel.release.major virtual is_virtual networking memory memory.system.total \
    processors processors.count system_uptime load_averages mountpoints disks partitions path; do
    escaped_key=$(escape_ere "$key")
    printf '%s\n' "$out" | grep -Eq "\"$escaped_key\"[[:space:]]*:" \
        || fail "missing fact $key"
    printf '%s\n' "$out" | grep -Eq "\"$escaped_key\"[[:space:]]*:[[:space:]]*null" \
        && fail "fact $key is null"
done

echo "dragonfly-release-gate: all facts present and non-null"
