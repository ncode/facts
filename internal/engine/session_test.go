package engine

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// testSession is shared by tests that only read resolved facts, mirroring the
// process-wide memoization the package had before sessions; tests that need a
// cold cache create their own Session.
var testSession = NewSession()

type fakeHostOS struct {
	platform            string
	runCalls            []fakeHostRunCall
	runOutput           string
	runOutputs          map[string]string
	files               map[string][]byte
	dirs                map[string][]os.DirEntry
	stats               map[string]os.FileInfo
	lstats              map[string]os.FileInfo
	globs               map[string][]string
	mountStats          map[string]mountStat
	readDirCalls        []string
	globCalls           []string
	statMountpointCalls []string
}

type fakeHostRunCall struct {
	name string
	args []string
}

func (h *fakeHostOS) run(_ context.Context, name string, args ...string) string {
	h.runCalls = append(h.runCalls, fakeHostRunCall{name: name, args: append([]string(nil), args...)})
	if h.runOutputs != nil {
		if output, ok := h.runOutputs[fakeRunKey(name, args...)]; ok {
			return output
		}
	}
	if h.runOutput != "" {
		return h.runOutput
	}
	return "host-output\n"
}

func fakeRunKey(name string, args ...string) string {
	return strings.Join(append([]string{name}, args...), "\x00")
}

func (h *fakeHostOS) goos() string {
	if h.platform != "" {
		return h.platform
	}
	return runtime.GOOS
}

func (h *fakeHostOS) readFile(path string) ([]byte, error) {
	data, ok := h.files[fakeHostPath(path)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func (h *fakeHostOS) readDir(path string) ([]os.DirEntry, error) {
	h.readDirCalls = append(h.readDirCalls, path)
	entries, ok := h.dirs[fakeHostPath(path)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return entries, nil
}

func (h *fakeHostOS) stat(path string) (os.FileInfo, error) {
	info, ok := h.stats[fakeHostPath(path)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return info, nil
}

func (h *fakeHostOS) lstat(path string) (os.FileInfo, error) {
	info, ok := h.lstats[fakeHostPath(path)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return info, nil
}

func (h *fakeHostOS) glob(pattern string) ([]string, error) {
	h.globCalls = append(h.globCalls, pattern)
	matches, ok := h.globs[fakeHostPath(pattern)]
	if !ok {
		return nil, nil
	}
	return append([]string(nil), matches...), nil
}

func (h *fakeHostOS) statMountpoint(path string) (mountStat, bool) {
	h.statMountpointCalls = append(h.statMountpointCalls, path)
	stat, ok := h.mountStats[fakeHostPath(path)]
	return stat, ok
}

func fakeHostPath(path string) string {
	return filepath.ToSlash(path)
}

type fakeFileInfo struct {
	name  string
	mode  os.FileMode
	isDir bool
}

func (fi fakeFileInfo) Name() string       { return fi.name }
func (fi fakeFileInfo) Size() int64        { return 0 }
func (fi fakeFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fi fakeFileInfo) IsDir() bool        { return fi.isDir }
func (fi fakeFileInfo) Sys() any           { return nil }

type fakeDirEntry struct {
	name  string
	mode  os.FileMode
	isDir bool
}

func (de fakeDirEntry) Name() string      { return de.name }
func (de fakeDirEntry) IsDir() bool       { return de.isDir }
func (de fakeDirEntry) Type() os.FileMode { return de.mode.Type() }
func (de fakeDirEntry) Info() (os.FileInfo, error) {
	return fakeFileInfo{name: de.name, mode: de.mode, isDir: de.isDir}, nil
}

func fakeDirEntries(names ...string) []os.DirEntry {
	entries := make([]os.DirEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, fakeDirEntry{name: name, mode: os.ModeDir, isDir: true})
	}
	return entries
}

func TestNewSessionContextDefaultsNilContext(t *testing.T) {
	s := NewSessionContext(nil)
	if s.Context() == nil {
		t.Fatal("Context() = nil, want background context")
	}
	select {
	case <-s.Context().Done():
		t.Fatal("nil context default is already canceled")
	default:
	}
}

func TestSessionLogrDefaultsNilLogger(t *testing.T) {
	s := &Session{}
	if s.logr() == nil {
		t.Fatal("logr() = nil, want discard logger")
	}
	s.warn("ignored")
	s.debug("ignored")
}

func TestOSHostLstatAndGlobUseFilesystem(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "one.fact")
	if err := os.WriteFile(path, []byte("fact=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	host := osHost{}
	info, err := host.lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "one.fact" {
		t.Fatalf("lstat().Name() = %q, want one.fact", info.Name())
	}

	matches, err := host.glob(filepath.Join(dir, "*.fact"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{path}; !reflect.DeepEqual(matches, want) {
		t.Fatalf("glob() = %#v, want %#v", matches, want)
	}
}

func TestSessionRoutesHostIOThroughHost(t *testing.T) {
	host := &fakeHostOS{
		files:  map[string][]byte{"/proc/data": []byte("file-data")},
		dirs:   map[string][]os.DirEntry{"/dir": fakeDirEntries("one", "two")},
		stats:  map[string]os.FileInfo{"/stat": fakeFileInfo{name: "stat"}},
		lstats: map[string]os.FileInfo{"/lstat": fakeFileInfo{name: "lstat"}},
		globs:  map[string][]string{"/tmp/*.fact": {"/tmp/one.fact", "/tmp/two.fact"}},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	if got := s.commandOutput("cmd", "arg"); got != "host-output\n" {
		t.Fatalf("commandOutput() = %q, want host output", got)
	}
	wantCalls := []fakeHostRunCall{{name: "cmd", args: []string{"arg"}}}
	if !reflect.DeepEqual(host.runCalls, wantCalls) {
		t.Fatalf("run calls = %#v, want %#v", host.runCalls, wantCalls)
	}
	data, err := s.readFile("/proc/data")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "file-data" {
		t.Fatalf("readFile() = %q, want file-data", string(data))
	}
	info, err := s.stat("/stat")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "stat" {
		t.Fatalf("stat().Name() = %q, want stat", info.Name())
	}
	info, err = s.lstat("/lstat")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "lstat" {
		t.Fatalf("lstat().Name() = %q, want lstat", info.Name())
	}
	entries, err := s.readDir("/dir")
	if err != nil {
		t.Fatal(err)
	}
	gotNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		gotNames = append(gotNames, entry.Name())
	}
	if want := []string{"one", "two"}; !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("readDir() names = %#v, want %#v", gotNames, want)
	}
	matches, err := s.glob("/tmp/*.fact")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/tmp/one.fact", "/tmp/two.fact"}; !reflect.DeepEqual(matches, want) {
		t.Fatalf("glob() = %#v, want %#v", matches, want)
	}
	if want := []string{"/dir"}; !reflect.DeepEqual(host.readDirCalls, want) {
		t.Fatalf("readDir calls = %#v, want %#v", host.readDirCalls, want)
	}
	if want := []string{"/tmp/*.fact"}; !reflect.DeepEqual(host.globCalls, want) {
		t.Fatalf("glob calls = %#v, want %#v", host.globCalls, want)
	}
}

func TestSessionBackedProbesUseFakeHost(t *testing.T) {
	host := &fakeHostOS{
		runOutput: "augparse 1.14.1\n",
		files: map[string][]byte{
			"/proc/meminfo": []byte("MemTotal:       1024 kB\n"),
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	if got := probeLinuxMeminfo(s); got != "MemTotal:       1024 kB\n" {
		t.Fatalf("probeLinuxMeminfo() = %q, want fake meminfo", got)
	}
	if got := probeAugeasVersion(s); got != "1.14.1" {
		t.Fatalf("probeAugeasVersion() = %q, want fake host output parsed", got)
	}
	wantCalls := []fakeHostRunCall{{name: "augparse", args: []string{"--version"}}}
	if !reflect.DeepEqual(host.runCalls, wantCalls) {
		t.Fatalf("run calls = %#v, want %#v", host.runCalls, wantCalls)
	}
}

func TestSessionCachedLinuxMeminfoMemoizesFirstRead(t *testing.T) {
	host := &fakeHostOS{
		files: map[string][]byte{
			"/proc/meminfo": []byte("MemTotal:       1024 kB\n"),
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	if got := s.cachedLinuxMeminfo(); got != "MemTotal:       1024 kB\n" {
		t.Fatalf("cachedLinuxMeminfo() = %q, want first fake meminfo", got)
	}
	host.files["/proc/meminfo"] = []byte("MemTotal:       2048 kB\n")
	if got := s.cachedLinuxMeminfo(); got != "MemTotal:       1024 kB\n" {
		t.Fatalf("cachedLinuxMeminfo() after host mutation = %q, want cached first read", got)
	}
}

func TestSessionCachedWindowsOSVersionInputUsesSessionPlatform(t *testing.T) {
	host := &fakeHostOS{
		platform: "windows",
		runOutputs: map[string]string{
			fakeRunKey("wmic", "os", "get", "OtherTypeDescription,ProductType,Version", "/value"):                                                                        "",
			fakeRunKey("powershell", "-NoProfile", "-NonInteractive", "-Command", windowsCIMScript("Win32_OperatingSystem", "OtherTypeDescription,ProductType,Version")): "OtherTypeDescription=\r\nProductType=1\r\nVersion=10.0.22631\r\n",
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	if got := s.cachedWindowsOSVersionInput(); !strings.Contains(got, "Version=10.0.22631") {
		t.Fatalf("cachedWindowsOSVersionInput() = %q, want Windows version input", got)
	}
	host.runOutputs[fakeRunKey("powershell", "-NoProfile", "-NonInteractive", "-Command", windowsCIMScript("Win32_OperatingSystem", "OtherTypeDescription,ProductType,Version"))] = "Version=changed\r\n"
	if got := s.cachedWindowsOSVersionInput(); !strings.Contains(got, "Version=10.0.22631") {
		t.Fatalf("cachedWindowsOSVersionInput() after host mutation = %q, want cached first read", got)
	}
	if len(host.runCalls) != 2 {
		t.Fatalf("run calls = %#v, want wmic and PowerShell fallback", host.runCalls)
	}
}

func TestSessionCachedSwapEncryptedMemoizesFreeBSDProbe(t *testing.T) {
	host := &fakeHostOS{
		platform: "freebsd",
		runOutputs: map[string]string{
			fakeRunKey("sysctl", "-n", "vm.stats.vm.v_page_size"):    "4096\n",
			fakeRunKey("sysctl", "-n", "vm.stats.vm.v_page_count"):   "100\n",
			fakeRunKey("sysctl", "-n", "vm.stats.vm.v_active_count"): "20\n",
			fakeRunKey("sysctl", "-n", "vm.stats.vm.v_wire_count"):   "10\n",
			fakeRunKey("swapinfo", "-k"): strings.Join([]string{
				"Device          1K-blocks     Used    Avail Capacity",
				"/dev/gpt/swap.eli     200       50      150    25%",
			}, "\n"),
		},
	}
	s := NewSessionContext(context.Background())
	s.host = host

	if !s.cachedSwapEncrypted() {
		t.Fatal("cachedSwapEncrypted() = false, want true")
	}
	host.runOutputs[fakeRunKey("swapinfo", "-k")] = ""
	if !s.cachedSwapEncrypted() {
		t.Fatal("cachedSwapEncrypted() after host mutation = false, want cached true")
	}
}

func TestCoreCommandEnvSanitizesPath(t *testing.T) {
	env := coreCommandEnv([]string{"PATH=/tmp/attacker", "HOME=/home/alice", "LD_PRELOAD=/tmp/libhack.so"}, "linux")

	for _, entry := range env {
		if entry == "PATH=/tmp/attacker" {
			t.Fatalf("coreCommandEnv kept attacker PATH: %#v", env)
		}
	}
	if slices.Contains(env, "HOME=/home/alice") || slices.Contains(env, "LD_PRELOAD=/tmp/libhack.so") {
		t.Fatalf("coreCommandEnv kept caller environment entries: %#v", env)
	}
	path := ""
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			path = value
			break
		}
	}
	if path == "" {
		t.Fatalf("coreCommandEnv did not set PATH: %#v", env)
	}
	if strings.Contains(path, "/tmp/attacker") {
		t.Fatalf("PATH = %q, want sanitized path", path)
	}
}

func TestOSHostRunTimesOutWedgedCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the POSIX sleep command")
	}
	prev := coreCommandTimeout
	coreCommandTimeout = 50 * time.Millisecond
	t.Cleanup(func() { coreCommandTimeout = prev })

	start := time.Now()
	got := (osHost{}).run(context.Background(), "sleep", "30")
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("osHost.run did not honor coreCommandTimeout: ran for %s", elapsed)
	}
	if got != "" {
		t.Fatalf("osHost.run() = %q, want empty output on timeout", got)
	}
}

func TestCoreCommandEnvWindowsKeepsEnvButOverridesPathAndRoot(t *testing.T) {
	// coreWindowsRoot() resolves to C:\Windows on non-Windows builds, so this
	// test is deterministic on the Linux/macOS CI runners.
	env := coreCommandEnv([]string{
		`TEMP=C:\Temp`,
		`PATH=C:\attacker`,
		`systemroot=C:\attacker`,
		`WinDir=C:\attacker`,
	}, "windows")

	if !slices.Contains(env, `TEMP=C:\Temp`) {
		t.Fatalf("coreCommandEnv dropped inherited Windows env: %#v", env)
	}
	if !slices.Contains(env, `SystemRoot=C:\Windows`) || !slices.Contains(env, `WINDIR=C:\Windows`) {
		t.Fatalf("coreCommandEnv did not set trusted SystemRoot/WINDIR: %#v", env)
	}
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		if coreWindowsManagedEnv(name) && strings.Contains(strings.ToLower(value), `c:\attacker`) {
			t.Fatalf("coreCommandEnv kept attacker %s: %#v", name, env)
		}
	}
}

func TestCoreCommandSearchPathIgnoresCallerSystemRoot(t *testing.T) {
	path := coreCommandSearchPath("windows", []string{`SYSTEMROOT=C:\attacker`})

	if strings.Contains(strings.ToLower(path), `c:\attacker`) {
		t.Fatalf("windows core command path = %q, want trusted Windows root", path)
	}
}

func TestCoreCommandPathHelpersArePlatformAware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  string
		goos string
		want bool
	}{
		{name: "slash", cmd: "bin/tool", goos: "linux", want: true},
		{name: "backslash", cmd: `bin\tool`, goos: "linux", want: true},
		{name: "windows drive", cmd: `C:tool`, goos: "windows", want: true},
		{name: "posix colon", cmd: "C:tool", goos: "linux"},
		{name: "bare", cmd: "tool", goos: "windows"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandHasPathSeparator(tt.cmd, tt.goos); got != tt.want {
				t.Fatalf("commandHasPathSeparator(%q, %q) = %v, want %v", tt.cmd, tt.goos, got, tt.want)
			}
		})
	}

	if got := coreCommandCandidates("tool", "windows"); !reflect.DeepEqual(got, []string{"tool.exe", "tool.com", "tool.bat", "tool.cmd"}) {
		t.Fatalf("coreCommandCandidates(windows bare) = %#v", got)
	}
	if got := coreCommandCandidates("tool.exe", "windows"); !reflect.DeepEqual(got, []string{"tool.exe"}) {
		t.Fatalf("coreCommandCandidates(windows extension) = %#v", got)
	}
	if got := coreCommandCandidates("tool", "linux"); !reflect.DeepEqual(got, []string{"tool"}) {
		t.Fatalf("coreCommandCandidates(linux) = %#v", got)
	}
	if got, ok := coreCommandExecutable("bin/tool", "linux"); !ok || got != "bin/tool" {
		t.Fatalf("coreCommandExecutable(path) = %q, %v; want bin/tool, true", got, ok)
	}
}

func TestCoreCommandFileExecutableChecksRegularExecutableFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	executable := filepath.Join(dir, "tool")
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plain, []byte("not executable\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !coreCommandFileExecutable(executable, "linux") {
		t.Fatalf("coreCommandFileExecutable(%q, linux) = false, want true", executable)
	}
	if coreCommandFileExecutable(plain, "linux") {
		t.Fatalf("coreCommandFileExecutable(%q, linux) = true, want false", plain)
	}
	if !coreCommandFileExecutable(plain, "windows") {
		t.Fatalf("coreCommandFileExecutable(%q, windows) = false, want regular files accepted", plain)
	}
	if coreCommandFileExecutable(dir, "windows") {
		t.Fatalf("coreCommandFileExecutable(%q, windows) = true, want directory rejected", dir)
	}
}

func TestOSHostRunDoesNotSearchCallerPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script")
	}
	dir := t.TempDir()
	name := "facts-test-attacker-command"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf pwned\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	if got := (osHost{}).run(context.Background(), name); got != "" {
		t.Fatalf("osHost.run() = %q, want no output from caller PATH command", got)
	}
}
