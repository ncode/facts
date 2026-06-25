package engine

import (
	"context"
	"os"
	"reflect"
	"testing"
)

func TestXenFacts_includePrivilegedDomains(t *testing.T) {
	got := Collection(xenFacts("xen0", []string{"win", "linux"}))

	wantXen := map[string]any{"domains": []string{"win", "linux"}}
	if !reflect.DeepEqual(got["xen"], wantXen) {
		t.Fatalf("xen = %#v, want %#v", got["xen"], wantXen)
	}
}

func TestXenFacts_skipUnprivilegedXen(t *testing.T) {
	got := xenFacts("xenu", []string{"win"})

	if len(got) != 1 || got[0].Name != "xen" || got[0].Value != nil {
		t.Fatalf("xenFacts(xenu) = %#v, want nil xen fact only", got)
	}
}

func TestDetectXenVMFromSignalsMatchesRubyResolver(t *testing.T) {
	tests := []struct {
		name      string
		evtchn    bool
		procXen   bool
		xvda1     bool
		xvda1Link bool
		want      string
	}{
		{name: "privileged evtchn", evtchn: true, procXen: true, xvda1: true, want: "xen0"},
		{name: "proc xen unprivileged", procXen: true, want: "xenu"},
		{name: "xvda unprivileged", xvda1: true, want: "xenu"},
		{name: "xvda symlink ignored", xvda1: true, xvda1Link: true, want: ""},
		{name: "not xen", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectXenVMFromSignals(tt.evtchn, tt.procXen, tt.xvda1, tt.xvda1Link); got != tt.want {
				t.Fatalf("detectXenVMFromSignals() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectXenCommandMatchesRubyResolver(t *testing.T) {
	tests := []struct {
		name   string
		exists map[string]bool
		want   string
	}{
		{
			name: "both stacks prefer xen toolstack",
			exists: map[string]bool{
				"/usr/lib/xen-common/bin/xen-toolstack": true,
				"/usr/sbin/xl":                          true,
				"/usr/sbin/xm":                          true,
			},
			want: "/usr/lib/xen-common/bin/xen-toolstack",
		},
		{name: "xl first", exists: map[string]bool{"/usr/sbin/xl": true}, want: "/usr/sbin/xl"},
		{name: "xm fallback", exists: map[string]bool{"/usr/sbin/xm": true}, want: "/usr/sbin/xm"},
		{name: "no command", exists: map[string]bool{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectXenCommand(func(path string) bool { return tt.exists[path] })
			if got != tt.want {
				t.Fatalf("selectXenCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectXenDomainsWithCommandRunsSelectedToolstack(t *testing.T) {
	exists := map[string]bool{
		"/usr/sbin/xl": true,
	}
	got := detectXenDomainsWithCommand(func(path string) bool { return exists[path] }, func(name string, args ...string) string {
		if name != "/usr/sbin/xl" || !reflect.DeepEqual(args, []string{"list"}) {
			t.Fatalf("run(%q, %#v), want xl list", name, args)
		}
		return "Name ID Mem VCPUs State Time(s)\nDomain-0 0 4096 4 r----- 100.0\nguest-web 1 2048 2 -b---- 10.0\n"
	})
	want := []string{"guest-web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detectXenDomainsWithCommand() = %#v, want %#v", got, want)
	}

	if got := detectXenDomainsWithCommand(func(string) bool { return false }, func(string, ...string) string {
		t.Fatal("detectXenDomainsWithCommand ran command without Xen toolstack")
		return ""
	}); got != nil {
		t.Fatalf("detectXenDomainsWithCommand(no command) = %#v, want nil", got)
	}

	if got := detectXenDomainsWithCommand(func(path string) bool { return path == "/usr/sbin/xl" }, func(string, ...string) string {
		return ""
	}); got != nil {
		t.Fatalf("detectXenDomainsWithCommand(no output) = %#v, want nil", got)
	}
}

func TestCurrentXenFactsUsesSessionPlatformAndHostToolstack(t *testing.T) {
	s := NewSessionContext(context.Background())
	s.host = &fakeHostOS{
		platform: "linux",
		files: map[string][]byte{
			"/proc/xen/capabilities": []byte("control_d\n"),
		},
		stats: map[string]os.FileInfo{
			"/usr/sbin/xl": fakeFileInfo{name: "xl"},
		},
		runOutputs: map[string]string{
			fakeRunKey("/usr/sbin/xl", "list"): "Name ID Mem VCPUs State Time(s)\nDomain-0 0 4096 4 r----- 100.0\nguest-web 1 2048 2 -b---- 10.0\n",
		},
	}

	got := Collection(currentXenFacts(s))
	want := map[string]any{"xen": map[string]any{"domains": []string{"guest-web"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("currentXenFacts() = %#v, want %#v", got, want)
	}
}
