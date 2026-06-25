package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGCEFactsFetchRecursiveMetadataAndNormalizeInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "/?recursive=true&alt=json" {
			t.Errorf("request URL = %q, want recursive JSON metadata endpoint", r.URL.String())
			http.Error(w, "unexpected metadata endpoint", http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("Metadata-Flavor"); got != "Google" {
			t.Errorf("Metadata-Flavor = %q, want Google", got)
			http.Error(w, "missing metadata header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Metadata-Flavor", "Google")
		_, _ = w.Write([]byte(`{
			"project": {"attributes": {"sshKeys": "alice:key\nbob:key\n"}},
			"instance": {
				"attributes": {"ssh-keys": "carol:key\n"},
				"image": "projects/debian-cloud/global/images/debian-12-bookworm-v20240110",
				"machineType": "projects/123456789/machineTypes/e2-medium",
				"zone": "projects/123456789/zones/us-central1-a",
				"networkInterfaces": [{"network": "projects/123456789/global/networks/default"}]
			}
		}`))
	}))
	t.Cleanup(server.Close)

	facts := gceFacts(context.Background(), newGCEClient(server.URL, server.Client()))
	got := factValues(facts)

	metadata, ok := got["gce"].(map[string]any)
	if !ok {
		t.Fatalf("gce fact = %#v, want metadata map", got["gce"])
	}
	project := metadata["project"].(map[string]any)
	projectAttributes := project["attributes"].(map[string]any)
	if got, want := projectAttributes["sshKeys"], []any{"alice:key", "bob:key"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("project.attributes.sshKeys = %#v, want %#v", got, want)
	}
	instance := metadata["instance"].(map[string]any)
	for key, want := range map[string]any{
		"image":       "debian-12-bookworm-v20240110",
		"machineType": "e2-medium",
		"zone":        "us-central1-a",
	} {
		if got := instance[key]; got != want {
			t.Fatalf("instance.%s = %#v, want %#v", key, got, want)
		}
	}
	interfaces := instance["networkInterfaces"].([]any)
	primary := interfaces[0].(map[string]any)
	if got, want := primary["network"], "default"; got != want {
		t.Fatalf("networkInterfaces[0].network = %#v, want %#v", got, want)
	}
	attributes := instance["attributes"].(map[string]any)
	if got, want := attributes["ssh-keys"], []any{"carol:key"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("instance.attributes.ssh-keys = %#v, want %#v", got, want)
	}
	if got["cloud.provider"] != "gce" {
		t.Fatalf("cloud.provider = %#v, want gce", got["cloud.provider"])
	}
}

func TestGCEFactsSendsAcceptJSONHeaderLikeRubyResolver(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		w.Header().Set("Metadata-Flavor", "Google")
		_, _ = w.Write([]byte(`{"some":"metadata"}`))
	}))
	t.Cleanup(server.Close)

	got := factValues(gceFacts(context.Background(), newGCEClient(server.URL, server.Client())))
	if got["gce"] == nil {
		t.Fatalf("gce fact = %#v, want metadata", got["gce"])
	}
}

func TestGCEFactsSkipInvalidMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Metadata-Flavor", "Google")
		_, _ = w.Write([]byte(`not json`))
	}))
	t.Cleanup(server.Close)

	if got := gceFacts(context.Background(), newGCEClient(server.URL, server.Client())); len(got) != 0 {
		t.Fatalf("gceFacts(context.Background(), ) = %#v, want no facts for invalid metadata", got)
	}
}

func TestGCEFactsRequireGoogleMetadataFlavor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Metadata-Flavor", "NotGoogle")
		_, _ = w.Write([]byte(`{"some":"metadata"}`))
	}))
	t.Cleanup(server.Close)

	if got := gceFacts(context.Background(), newGCEClient(server.URL, server.Client())); len(got) != 0 {
		t.Fatalf("gceFacts(context.Background(), ) = %#v, want no facts for spoofed metadata flavor", got)
	}
}

func TestGCEFactsSkipNilClient(t *testing.T) {
	if got := gceFacts(context.Background(), nil); got != nil {
		t.Fatalf("gceFacts(nil client) = %#v, want nil", got)
	}
}

func TestLinuxGCEFactsSkipsMetadataWhenBIOSVendorIsNotGoogleLikeRuby(t *testing.T) {
	var requested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requested.Store(true)
	}))
	t.Cleanup(server.Close)

	got := linuxGCEFacts(context.Background(), "linux", "Acme BIOS", newGCEClient(server.URL, server.Client()))
	want := []ResolvedFact{{Name: "gce", Value: nil}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linuxGCEFacts(context.Background(), ) = %#v, want %#v", got, want)
	}
	if requested.Load() {
		t.Fatal("metadata endpoint was queried for non-Google BIOS vendor")
	}
}

func TestLinuxGCEFactsFetchesMetadataForGoogleBIOSVendor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "/?recursive=true&alt=json" {
			t.Errorf("request URL = %q, want recursive JSON metadata endpoint", r.URL.String())
			http.Error(w, "unexpected metadata endpoint", http.StatusBadRequest)
			return
		}
		w.Header().Set("Metadata-Flavor", "Google")
		_, _ = w.Write([]byte(`{"instance":{"machineType":"projects/123/machineTypes/e2-medium"}}`))
	}))
	t.Cleanup(server.Close)

	got := factValues(linuxGCEFacts(context.Background(), "linux", "Google", newGCEClient(server.URL, server.Client())))
	gce, ok := got["gce"].(map[string]any)
	if !ok {
		t.Fatalf("gce fact = %#v, want metadata map", got["gce"])
	}
	instance := gce["instance"].(map[string]any)
	if instance["machineType"] != "e2-medium" || got["cloud.provider"] != "gce" {
		t.Fatalf("linuxGCEFacts() = %#v, want normalized metadata and cloud provider", got)
	}
}

func TestLinuxGCEFactsEmitsNilWhenGoogleBIOSHasNoClient(t *testing.T) {
	got := linuxGCEFacts(context.Background(), "linux", "Google", nil)
	want := []ResolvedFact{{Name: "gce", Value: nil}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linuxGCEFacts(nil client) = %#v, want %#v", got, want)
	}
}

func TestLinuxGCEFactsSkipNonLinuxPlatform(t *testing.T) {
	if got := linuxGCEFacts(context.Background(), "freebsd", "Google", nil); got != nil {
		t.Fatalf("linuxGCEFacts(non-linux) = %#v, want nil", got)
	}
}

func TestPlatformGCEFactsFetchesWindowsMetadataWhenVirtualizationIsGCELikeRuby(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "/?recursive=true&alt=json" {
			t.Fatalf("request URL = %q, want recursive JSON metadata endpoint", r.URL.String())
		}
		w.Header().Set("Metadata-Flavor", "Google")
		_, _ = w.Write([]byte(`{"some":"metadata"}`))
	}))
	t.Cleanup(server.Close)

	got := factValues(platformGCEFacts(context.Background(), "windows", virtualization{Name: "gce", IsVirtual: true}, "", newGCEClient(server.URL, server.Client())))
	if got["gce"] == nil {
		t.Fatalf("gce fact = %#v, want metadata", got["gce"])
	}
	if got["cloud.provider"] != "gce" {
		t.Fatalf("cloud.provider = %#v, want gce", got["cloud.provider"])
	}
}

func TestPlatformGCEFactsEmitsNilForWindowsWithEmptyMetadata(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "http://metadata.test/?recursive=true&alt=json" {
			t.Fatalf("request URL = %q, want recursive JSON metadata endpoint", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Metadata-Flavor": []string{"Google"}},
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}, nil
	})}

	got := platformGCEFacts(context.Background(), "windows", virtualization{Name: "gce", IsVirtual: true}, "", newGCEClient("http://metadata.test", client))
	want := []ResolvedFact{{Name: "gce", Value: nil}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("platformGCEFacts(windows empty metadata) = %#v, want %#v", got, want)
	}
}

func TestPlatformGCEFactsDispatchesLinux(t *testing.T) {
	got := platformGCEFacts(context.Background(), "linux", virtualization{}, "not google", nil)
	want := []ResolvedFact{{Name: "gce", Value: nil}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("platformGCEFacts(linux non-GCE) = %#v, want %#v", got, want)
	}
}

func TestPlatformGCEFactsEmitsNilForWindowsWithoutGCESignal(t *testing.T) {
	got := platformGCEFacts(context.Background(), "windows", virtualization{Name: "kvm", IsVirtual: true}, "", nil)
	want := []ResolvedFact{{Name: "gce", Value: nil}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("platformGCEFacts(windows non-gce) = %#v, want %#v", got, want)
	}
}

func TestPlatformGCEFactsSkipUnsupportedPlatform(t *testing.T) {
	if got := platformGCEFacts(context.Background(), "freebsd", virtualization{Name: "gce", IsVirtual: true}, "Google", nil); got != nil {
		t.Fatalf("platformGCEFacts(unsupported) = %#v, want nil", got)
	}
}

func factValues(facts []ResolvedFact) map[string]any {
	values := make(map[string]any, len(facts))
	for _, fact := range facts {
		values[fact.Name] = fact.Value
	}
	return values
}
