package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestGCEFactsFetchRecursiveMetadataAndNormalizeInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "/?recursive=true&alt=json" {
			t.Fatalf("request URL = %q, want recursive JSON metadata endpoint", r.URL.String())
		}
		if got := r.Header.Get("Metadata-Flavor"); got != "Google" {
			t.Fatalf("Metadata-Flavor = %q, want Google", got)
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

func factValues(facts []ResolvedFact) map[string]any {
	values := make(map[string]any, len(facts))
	for _, fact := range facts {
		values[fact.Name] = fact.Value
	}
	return values
}
