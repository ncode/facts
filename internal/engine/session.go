package engine

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

type hostOS interface {
	run(context.Context, string, ...string) string
	readFile(string) ([]byte, error)
	stat(string) (os.FileInfo, error)
	lstat(string) (os.FileInfo, error)
}

type osHost struct{}

func (osHost) run(ctx context.Context, name string, args ...string) string {
	data, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return string(data)
}

func (osHost) readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (osHost) stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (osHost) lstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

// Session carries the state of one resolution run: memoized host probes and
// resolution-scoped caches. Resolvers share a Session so facts derived from
// the same probe agree within a run; a fresh Session re-reads the host, which
// is how discovery stays current and how independent engines stay isolated.
type Session struct {
	ctx  context.Context
	host hostOS
	// logger receives engine diagnostics when the session belongs to an
	// Engine; when nil, diagnostics fall back to the process-wide handler
	// callbacks that back the CLI and the Ruby-compat API.
	logger *slog.Logger

	coreFacts struct {
		mu    sync.Mutex
		facts []ResolvedFact
	}

	augeasVersion                memo[string]
	architectureName             memo[string]
	kernelRelease                memo[string]
	hardwareModel                memo[string]
	macOSModel                   memo[string]
	oSRelease                    memo[any]
	windowsOSVersionInput        memo[string]
	macOSInfo                    memo[macOSInfo]
	macOSSystemProfilerHardware  memo[macOSSystemProfilerHardware]
	macOSSystemProfilerSoftware  memo[macOSSystemProfilerSoftware]
	macOSSystemProfilerEthernet  memo[macOSSystemProfilerEthernet]
	linuxDistro                  memo[linuxDistro]
	totalPhysicalMemoryBytes     memo[int]
	availablePhysicalMemoryBytes memo[int]
	totalSwapMemoryBytes         memo[int]
	availableSwapMemoryBytes     memo[int]
	swapEncrypted                memo[bool]
	windowsMemory                memo[windowsMemory]
	freeBSDMemoryInfo            memo[freeBSDMemoryInfo]
	darwinSwapUsage              memo[darwinSwapUsage]
	linuxMeminfo                 memo[string]
	processorSpeed               memo[string]
	processorModels              memo[[]string]
	processorTopology            memo2[int, int]
	platformProcessorInfo        memo[processorInfo]
	processorExtensions          memo[[]string]
	uptime                       memo[uptimeInfo]
	loadAverages                 memo[map[string]any]
	filesystems                  memo[any]
}

// NewSession returns an empty Session; probes run on first use.
func NewSession() *Session {
	return NewSessionContext(context.Background())
}

// NewSessionContext returns an empty Session whose command executions and
// metadata requests are bound to ctx.
func NewSessionContext(ctx context.Context) *Session {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Session{ctx: ctx, host: osHost{}}
}

// Context returns the context this session's resolution work runs under.
func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) readFile(path string) ([]byte, error) {
	return s.host.readFile(path)
}

func (s *Session) stat(path string) (os.FileInfo, error) {
	return s.host.stat(path)
}

func (s *Session) lstat(path string) (os.FileInfo, error) {
	return s.host.lstat(path)
}

func (s *Session) warn(message string) {
	if s.logger != nil {
		s.logger.Warn(message)
		return
	}
	warn(message)
}

func (s *Session) debug(message string) {
	if s.logger != nil {
		s.logger.Debug(message)
		return
	}
	debug(message)
}

func (s *Session) reportError(message string) {
	if s.logger != nil {
		s.logger.Error(message)
		return
	}
	reportError(message)
}

type memo[T any] struct {
	once sync.Once
	v    T
}

func (m *memo[T]) get(f func() T) T {
	m.once.Do(func() { m.v = f() })
	return m.v
}

type memo2[T1, T2 any] struct {
	once sync.Once
	v1   T1
	v2   T2
}

func (m *memo2[T1, T2]) get(f func() (T1, T2)) (T1, T2) {
	m.once.Do(func() { m.v1, m.v2 = f() })
	return m.v1, m.v2
}

func (s *Session) cachedAugeasVersion() string {
	return s.augeasVersion.get(func() string { return probeAugeasVersion(s) })
}
func (s *Session) cachedArchitectureName() string {
	return s.architectureName.get(func() string { return probeArchitectureName(s) })
}
func (s *Session) cachedKernelRelease() string {
	return s.kernelRelease.get(func() string { return probeKernelRelease(s) })
}
func (s *Session) cachedHardwareModel() string {
	return s.hardwareModel.get(func() string { return probeHardwareModel(s) })
}
func (s *Session) cachedMacOSModel() string {
	return s.macOSModel.get(func() string { return probeMacOSModel(s) })
}
func (s *Session) cachedOSRelease() any {
	return s.oSRelease.get(func() any { return probeOSRelease(s) })
}
func (s *Session) cachedWindowsOSVersionInput() string {
	return s.windowsOSVersionInput.get(func() string { return probeWindowsOSVersionInput(s) })
}
func (s *Session) cachedMacOSInfo() macOSInfo {
	return s.macOSInfo.get(func() macOSInfo { return probeMacOSInfo(s) })
}
func (s *Session) cachedMacOSSystemProfilerHardware() macOSSystemProfilerHardware {
	return s.macOSSystemProfilerHardware.get(func() macOSSystemProfilerHardware { return probeMacOSSystemProfilerHardware(s) })
}
func (s *Session) cachedMacOSSystemProfilerSoftware() macOSSystemProfilerSoftware {
	return s.macOSSystemProfilerSoftware.get(func() macOSSystemProfilerSoftware { return probeMacOSSystemProfilerSoftware(s) })
}
func (s *Session) cachedMacOSSystemProfilerEthernet() macOSSystemProfilerEthernet {
	return s.macOSSystemProfilerEthernet.get(func() macOSSystemProfilerEthernet { return probeMacOSSystemProfilerEthernet(s) })
}
func (s *Session) cachedLinuxDistro() linuxDistro {
	return s.linuxDistro.get(func() linuxDistro { return probeLinuxDistro(s) })
}
func (s *Session) cachedTotalPhysicalMemoryBytes() int {
	return s.totalPhysicalMemoryBytes.get(func() int { return probeTotalPhysicalMemoryBytes(s) })
}
func (s *Session) cachedAvailablePhysicalMemoryBytes() int {
	return s.availablePhysicalMemoryBytes.get(func() int { return probeAvailablePhysicalMemoryBytes(s) })
}
func (s *Session) cachedTotalSwapMemoryBytes() int {
	return s.totalSwapMemoryBytes.get(func() int { return probeTotalSwapMemoryBytes(s) })
}
func (s *Session) cachedAvailableSwapMemoryBytes() int {
	return s.availableSwapMemoryBytes.get(func() int { return probeAvailableSwapMemoryBytes(s) })
}
func (s *Session) cachedSwapEncrypted() bool {
	return s.swapEncrypted.get(func() bool { return probeSwapEncrypted(s) })
}
func (s *Session) cachedWindowsMemory() windowsMemory {
	return s.windowsMemory.get(func() windowsMemory { return probeWindowsMemory(s) })
}
func (s *Session) cachedFreeBSDMemoryInfo() freeBSDMemoryInfo {
	return s.freeBSDMemoryInfo.get(func() freeBSDMemoryInfo { return probeFreeBSDMemoryInfo(s) })
}
func (s *Session) cachedDarwinSwapUsage() darwinSwapUsage {
	return s.darwinSwapUsage.get(func() darwinSwapUsage { return probeDarwinSwapUsage(s) })
}
func (s *Session) cachedLinuxMeminfo() string {
	return s.linuxMeminfo.get(func() string { return probeLinuxMeminfo(s) })
}
func (s *Session) cachedProcessorSpeed() string {
	return s.processorSpeed.get(func() string { return probeProcessorSpeed(s) })
}
func (s *Session) cachedProcessorModels() []string {
	return s.processorModels.get(func() []string { return probeProcessorModels(s) })
}
func (s *Session) cachedProcessorTopology() (int, int) {
	return s.processorTopology.get(func() (int, int) { return probeProcessorTopology(s) })
}
func (s *Session) cachedPlatformProcessorInfo() processorInfo {
	return s.platformProcessorInfo.get(func() processorInfo { return probePlatformProcessorInfo(s) })
}
func (s *Session) cachedProcessorExtensions() []string {
	return s.processorExtensions.get(func() []string { return probeProcessorExtensions(s) })
}
func (s *Session) cachedUptime() uptimeInfo {
	return s.uptime.get(func() uptimeInfo { return probeUptime(s) })
}
func (s *Session) cachedLoadAverages() map[string]any {
	return s.loadAverages.get(func() map[string]any { return probeLoadAverages(s) })
}
func (s *Session) cachedFilesystems() any {
	return s.filesystems.get(func() any { return probeFilesystems(s) })
}
