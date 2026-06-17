#!/bin/sh
# FreeBSD release gate: verifies the FreeBSD release-gate fact set through a
# built facts binary. This is the single definition of the FreeBSD fact set;
# both the FreeBSD CI job and `make lima-freebsd-smoke` run this script so the
# two gates cannot drift apart.
#
# Usage: freebsd-release-gate.sh [path-to-facts-binary]
set -eu

FACTS_BIN="${1:-./facts}"

if [ "$(uname -s)" != "FreeBSD" ]; then
    echo "freebsd-release-gate.sh must run on FreeBSD" >&2
    exit 1
fi

FACT_SET="os.name os.family os.release os.architecture os.hardware kernel.name \
kernel.release.full kernel.version.full kernel.release.major virtual is_virtual networking \
memory memory.system.total processors processors.count dmi system_uptime \
load_averages mountpoints"

# shellcheck disable=SC2086
out="$("$FACTS_BIN" --json $FACT_SET)"
printf '%s\n' "$out"

fail() {
    echo "freebsd-release-gate: $1" >&2
    exit 1
}

printf '%s\n' "$out" | grep -Eq '"os.name"[[:space:]]*:[[:space:]]*"FreeBSD"' \
    || fail 'os.name != "FreeBSD"'
printf '%s\n' "$out" | grep -Eq '"kernel.name"[[:space:]]*:[[:space:]]*"FreeBSD"' \
    || fail 'kernel.name != "FreeBSD"'
printf '%s\n' "$out" | grep -Eq '"os.family"[[:space:]]*:[[:space:]]*"FreeBSD"' \
    || fail 'os.family != "FreeBSD"'

for key in os.release os.architecture os.hardware kernel.release.full kernel.version.full \
    kernel.release.major virtual is_virtual networking memory memory.system.total \
    processors processors.count dmi system_uptime load_averages \
    mountpoints; do
    printf '%s\n' "$out" | grep -Eq "\"$key\"[[:space:]]*:" \
        || fail "missing fact $key"
    printf '%s\n' "$out" | grep -Eq "\"$key\"[[:space:]]*:[[:space:]]*null" \
        && fail "fact $key is null"
done

echo "freebsd-release-gate: all facts present and non-null"
