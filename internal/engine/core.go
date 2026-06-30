package engine

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Version is overridden at release time via
// -ldflags "-X github.com/ncode/facts/internal/engine.Version=<git tag>".
// The literal below is the dev-build fallback when no tag is injected.
var Version = "dev" // ponytail: var, not const, so the build pipeline can inject the git tag

// CoreFacts returns the small cross-platform fact set used by the initial Go
// CLI. disabled is the resolution-gating set: single-output categories whose
// top-level fact name it contains skip resolution entirely (ADR-0015). The
// result is memoized per Session, keyed by a fingerprint of the disabled set, so
// a repeat call with the same set reuses the build while a call with a different
// set rebuilds (the gating depends on disabled, so a stale memo would misgate).
func CoreFacts(s *Session, disabled map[string]bool) []ResolvedFact {
	fingerprint := disabledFingerprint(disabled)
	s.coreFacts.mu.Lock()
	defer s.coreFacts.mu.Unlock()
	if !s.coreFacts.built || s.coreFacts.fingerprint != fingerprint {
		s.coreFacts.facts = buildCoreFacts(s, disabled)
		s.coreFacts.fingerprint = fingerprint
		s.coreFacts.built = true
	}
	return append([]ResolvedFact(nil), s.coreFacts.facts...)
}

// disabledFingerprint is a stable identity for a disabled set — its enabled keys
// sorted and comma-joined. CoreFacts keys its per-Session memo on this so two
// calls gate identically iff their fingerprints match. Only true-valued keys
// gate (buildCoreFacts checks disabled[fact]), so a false-valued key is omitted.
func disabledFingerprint(disabled map[string]bool) string {
	if len(disabled) == 0 {
		return ""
	}
	keys := make([]string, 0, len(disabled))
	for name, on := range disabled {
		if on {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// buildCoreFacts composes the resolved core-fact set as the ordered union of
// each fact category's package-internal assembly function plus the cross-cutting
// facts (facterversion, path) and the virtualization-derived facts the cloud
// resolvers depend on. Facts are collected into a name-keyed canonical tree, so
// the composition order does not affect the resolved output; it mirrors the
// historical assembly order for reviewability.
func buildCoreFacts(s *Session, disabled map[string]bool) []ResolvedFact {
	virtualization := detectVirtualization(s)
	virtualFact, isVirtualFact := virtualizationFactValues(virtualization)
	dmi := s.cachedDMI()
	facts := []ResolvedFact{
		{Name: "facterversion", Value: Version},
		{Name: "is_virtual", Value: isVirtualFact},
		{Name: "path", Value: currentPathEntries(runtime.GOOS, os.Getenv)},
		{Name: "virtual", Value: virtualFact},
	}
	// gate skips a single-output category whose only top-level fact name is
	// disabled, so the resolver never runs (resolution-gating, ADR-0015). Only
	// categories whose entire output shares one root and whose probes are not
	// needed by a kept fact are gated; multi-output (os/disks/uptime),
	// shared-probe (identity, dmi), inline, and cloud/hypervisor facts below
	// stay eager and rely on FilterDisabledFacts resolve-then-prune. selinux is
	// likewise eager: it emits as os.selinux.* descendants, so a name-level
	// disable of "selinux" is a no-op — it is disabled via os.selinux.
	gate := func(fact string, resolve func(*Session) []ResolvedFact) {
		if disabled[fact] {
			return
		}
		facts = append(facts, resolve(s)...)
	}
	gate("networking", networkingCoreFacts)
	gate("processors", processorsCoreFacts)
	gate("memory", memoryCoreFacts)
	facts = append(facts, osCoreFacts(s)...)
	facts = append(facts, dmiCoreFacts(s)...)
	facts = append(facts, disksCoreFacts(s)...)
	gate("ssh", sshCoreFacts)
	facts = append(facts, identityCoreFacts(s)...)
	facts = append(facts, uptimeCoreFacts(s)...)
	facts = append(facts, selinuxCoreFacts(s)...)
	gate("fips_enabled", fipsCoreFacts)
	gate("timezone", timezoneCoreFacts)
	gate("augeas", augeasCoreFacts)
	gate("xen", xenCoreFacts)
	facts = append(facts, currentLinuxHypervisorFacts(s)...)
	facts = append(facts, currentWindowsHypervisorFacts(s)...)
	facts = append(facts, azureFacts(s.Context(), newAzureClient(azureMetadataBaseURL, nil), virtualization)...)
	facts = append(facts, ec2Facts(s, newEC2Client(ec2MetadataBaseURL, nil), virtualization)...)
	facts = append(facts, platformGCEFacts(s.Context(), runtime.GOOS, virtualization, dmiBIOSVendor(dmi), newGCEClient(gceMetadataBaseURL, nil))...)
	return facts
}

func currentPathEntries(goos string, getenv func(string) string) []string {
	key := "PATH"
	separator := string(os.PathListSeparator)
	if goos == "plan9" {
		key = "path"
		separator = "\x00"
	}
	return pathEntriesWithSeparator(getenv(key), separator)
}

// pathEntries splits a raw PATH value into an ordered array of entries on the
// host platform's path-list separator, preserving entry order and dropping
// empty entries.
func pathEntries(raw string) []string {
	return pathEntriesWithSeparator(raw, string(os.PathListSeparator))
}

func pathEntriesWithSeparator(raw, separator string) []string {
	entries := make([]string, 0)
	for _, entry := range strings.Split(raw, separator) {
		if entry == "" {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func virtualizationFactValues(v virtualization) (any, any) {
	if v.Unknown {
		return nil, nil
	}
	return v.Name, v.IsVirtual
}

func readText(path string, readFile fileReader) string {
	data, err := readFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func readOptionalText(path string, readFile fileReader) any {
	data, err := readFile(path)
	if err != nil {
		return nil
	}
	return strings.TrimSpace(string(data))
}

func readFileString(path string, readFiles ...fileReader) string {
	readFile := osHost{}.readFile
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	data, err := readFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func isSymlink(path string, lstats ...func(string) (os.FileInfo, error)) bool {
	lstat := osHost{}.lstat
	if len(lstats) > 0 && lstats[0] != nil {
		lstat = lstats[0]
	}
	info, err := lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

func readSysfsString(root, device, name string, readFiles ...fileReader) string {
	readFile := osHost{}.readFile
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	data, err := readFile(filepath.Join(root, device, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readDMIString(root, name string, readFiles ...fileReader) string {
	readFile := osHost{}.readFile
	if len(readFiles) > 0 && readFiles[0] != nil {
		readFile = readFiles[0]
	}
	data, err := readFile(filepath.Join(root, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ToValidUTF8(string(data), "\uFFFD"))
}

type commandRunner func(name string, args ...string) string

func missingFileReader(string) ([]byte, error) {
	return nil, os.ErrNotExist
}

func rootedPath(root, path string) string {
	if root == "/" {
		return "/" + strings.TrimPrefix(path, "/")
	}
	return filepath.Join(root, path)
}

func (s *Session) commandOutput(name string, args ...string) string {
	return s.host.run(s.ctx, name, args...)
}

func bytesToMB(value any) any {
	number, ok := numericValue(value)
	if !ok {
		return nil
	}
	if number <= 0 {
		return 0.0
	}
	return number / (1024.0 * 1024.0)
}

func bytesToHumanReadable(value any) any {
	number, original, ok := byteValue(value)
	if !ok {
		return nil
	}
	if number < 0 {
		return ""
	}
	if number < 1024 {
		return formatByteNumber(number, original) + " bytes"
	}

	prefixes := [...]string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	exp := int(math.Floor(math.Log2(number) / 10.0))
	converted := math.Round(100.0*(number/math.Pow(1024.0, float64(exp)))) / 100.0
	if converted == 1024.0 {
		exp++
		converted = 1.0
	}
	if exp < 1 || exp > len(prefixes) {
		return formatByteNumber(number, original) + " bytes"
	}
	return strconv.FormatFloat(converted, 'f', 2, 64) + " " + prefixes[exp-1]
}

func numericValue(value any) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case string:
		return rubyToI(v), true
	default:
		return 0, false
	}
}

func rubyToI(value string) float64 {
	value = strings.TrimLeft(value, " \t\n\v\f\r")
	if value == "" {
		return 0
	}

	end := 0
	if value[0] == '+' || value[0] == '-' {
		end = 1
	}
	startDigits := end
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == startDigits {
		return 0
	}

	n, err := strconv.ParseInt(value[:end], 10, 64)
	if err != nil {
		return 0
	}
	return float64(n)
}

func byteValue(value any) (float64, string, bool) {
	switch v := value.(type) {
	case nil:
		return 0, "", false
	case int:
		return float64(v), strconv.Itoa(v), true
	case int64:
		return float64(v), strconv.FormatInt(v, 10), true
	case float64:
		return v, strconv.FormatFloat(v, 'f', -1, 64), true
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, v, err == nil
	default:
		return 0, "", false
	}
}

func formatByteNumber(number float64, original string) string {
	if original != "" && math.Abs(number) >= 1e18 {
		return original
	}
	return strconv.FormatFloat(number, 'f', -1, 64)
}

func memoryCapacity(used, total int) string {
	if total <= 0 || used <= 0 {
		return "0.00%"
	}
	percent := 100.0 * float64(used) / float64(total)
	return strconv.FormatFloat(percent, 'f', 2, 64) + "%"
}

type memorySwapFactValues struct {
	available      any
	availableBytes any
	capacity       any
	total          any
	totalBytes     any
	used           any
	usedBytes      any
}

// memorySwapValues omits the whole swap subtree when there is no swap
// (zero total bytes), matching Ruby Facter on every platform.
func memorySwapValues(totalBytes, availableBytes int) memorySwapFactValues {
	if totalBytes <= 0 {
		return memorySwapFactValues{}
	}
	usedBytes := max(0, totalBytes-availableBytes)
	return memorySwapFactValues{
		available:      bytesToHumanReadable(availableBytes),
		availableBytes: availableBytes,
		capacity:       memoryCapacity(usedBytes, totalBytes),
		total:          bytesToHumanReadable(totalBytes),
		totalBytes:     totalBytes,
		used:           bytesToHumanReadable(usedBytes),
		usedBytes:      usedBytes,
	}
}

// filesystemCapacity computes the df/Facter capacity percentage as
// used/(used+available). The denominator is the space visible to an
// unprivileged writer (used + available), so root-reserved blocks read the
// same percentage df reports and a fully used mount (available == 0) reads
// 100%, not 0%. An empty or unknown mount (used <= 0) reads 0%.
func filesystemCapacity(used, available int) string {
	if available <= 0 {
		// No space available — full. Covers full read-only mounts and the
		// zero-size special filesystems (/dev/pts, hugepages) that Facter
		// reports as 100%. Checked before used so a 0-used/0-available mount
		// is 100%, not 0%.
		return "100%"
	}
	if used <= 0 {
		return "0%"
	}
	total := used + available
	percent := 100.0 * float64(used) / float64(total)
	return strconv.FormatFloat(percent, 'f', 2, 64) + "%"
}

type fileReader func(string) ([]byte, error)
