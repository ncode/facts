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

func TestSessionRoutesHostIOThroughHost(t *testing.T) {
	host := &fakeHostOS{
		files:  map[string][]byte{"/proc/data": []byte("file-data")},
		stats:  map[string]os.FileInfo{"/stat": fakeFileInfo{name: "stat"}},
		lstats: map[string]os.FileInfo{"/lstat": fakeFileInfo{name: "lstat"}},
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
