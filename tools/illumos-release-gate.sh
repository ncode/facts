#!/bin/sh
# illumos release gate: verifies the illumos candidate fact set through a
# built facts binary. This validates illumos/OmniOS only, not Oracle Solaris.
#
# Usage: illumos-release-gate.sh [path-to-facts-binary]
set -eu

PATH=/sbin:/usr/sbin:/bin:/usr/bin:/usr/sbin:/usr/local/sbin:/usr/local/bin
export PATH

FACTS_BIN="${1:-./facts}"

if [ "$(uname -s)" != "SunOS" ]; then
    echo "illumos-release-gate.sh must run on illumos/SunOS" >&2
    exit 1
fi

FACT_SET="os.name os.family os.release os.architecture os.hardware kernel.name \
kernel.release.full kernel.version.full kernel.release.major virtual is_virtual \
networking memory memory.system.total processors processors.count system_uptime \
load_averages mountpoints path"

# shellcheck disable=SC2086
out="$("$FACTS_BIN" --json $FACT_SET)"
printf '%s\n' "$out"

fail() {
    echo "illumos-release-gate: $1" >&2
    exit 1
}

escape_ere() {
    printf '%s\n' "$1" | sed 's/[][\\.^$*+?{}()|]/\\&/g'
}

printf '%s\n' "$out" | grep -Eq '"os.name"[[:space:]]*:' \
    || fail 'missing fact os.name'
printf '%s\n' "$out" | grep -Eq '"os.name"[[:space:]]*:[[:space:]]*null' \
    && fail 'fact os.name is null'
printf '%s\n' "$out" | grep -Eq '"kernel.name"[[:space:]]*:[[:space:]]*"SunOS"' \
    || fail 'kernel.name != "SunOS"'
printf '%s\n' "$out" | grep -Eq '"os.family"[[:space:]]*:[[:space:]]*"illumos"' \
    || fail 'os.family != "illumos"'

for key in os.release os.architecture os.hardware kernel.release.full kernel.version.full \
    kernel.release.major virtual is_virtual networking memory memory.system.total \
    processors processors.count system_uptime load_averages mountpoints path; do
    escaped_key=$(escape_ere "$key")
    printf '%s\n' "$out" | grep -Eq "\"$escaped_key\"[[:space:]]*:" \
        || fail "missing fact $key"
    printf '%s\n' "$out" | grep -Eq "\"$escaped_key\"[[:space:]]*:[[:space:]]*null" \
        && fail "fact $key is null"
done

echo "illumos-release-gate: all facts present and non-null"
