package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAzureFactsFetchMetadataAndCloudProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "/metadata/instance?api-version=2020-09-01" {
			t.Fatalf("request URL = %q, want Azure instance metadata endpoint", r.URL.String())
		}
		if got := r.Header.Get("Metadata"); got != "true" {
			t.Fatalf("Metadata header = %q, want true", got)
		}
		_, _ = w.Write([]byte(`{"compute":{"location":"westus2","vmSize":"Standard_B1s"},"network":{"interface":[{"ipv4":{"ipAddress":[{"privateIpAddress":"10.0.0.4"}]}}]}}`))
	}))
	t.Cleanup(server.Close)

	facts := azureFacts(context.Background(), newAzureClient(server.URL, server.Client()), virtualization{Name: "hyperv", IsVirtual: true})
	assertDescriptorDeclaresFacts(t, "az_metadata", facts)
	got := factValues(facts)

	metadata, ok := got["az_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("az_metadata = %#v, want metadata map", got["az_metadata"])
	}
	compute := metadata["compute"].(map[string]any)
	if got, want := compute["location"], "westus2"; got != want {
		t.Fatalf("compute.location = %#v, want %#v", got, want)
	}
	if got, want := got["cloud.provider"], "azure"; got != want {
		t.Fatalf("cloud.provider = %#v, want %#v", got, want)
	}
}

func TestAzureFactsRequireHyperVVirtualization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("Azure metadata endpoint should not be called outside Hyper-V")
	}))
	t.Cleanup(server.Close)

	facts := azureFacts(context.Background(), newAzureClient(server.URL, server.Client()), virtualization{Name: "kvm", IsVirtual: true})
	got := factValues(facts)

	if value, ok := got["az_metadata"]; !ok || value != nil {
		t.Fatalf("az_metadata = %#v, present %t; want nil fact outside Hyper-V", value, ok)
	}
}

func TestAzureFactsReturnNilMetadataForEmptyAzureResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	facts := azureFacts(context.Background(), newAzureClient(server.URL, server.Client()), virtualization{Name: "hyperv", IsVirtual: true})
	got := factValues(facts)

	if value, ok := got["az_metadata"]; !ok || value != nil {
		t.Fatalf("az_metadata = %#v, present %t; want nil fact for empty metadata", value, ok)
	}
	if provider := got["cloud.provider"]; provider != nil {
		t.Fatalf("cloud.provider = %#v, want omitted or nil for empty metadata", provider)
	}
}

func TestAzureFactsSkipInvalidMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	t.Cleanup(server.Close)

	facts := azureFacts(context.Background(), newAzureClient(server.URL, server.Client()), virtualization{Name: "hyperv", IsVirtual: true})
	got := factValues(facts)
	if value, ok := got["az_metadata"]; !ok || value != nil {
		t.Fatalf("az_metadata = %#v, present %t; want nil fact for invalid metadata", value, ok)
	}
	if provider := got["cloud.provider"]; provider != nil {
		t.Fatalf("cloud.provider = %#v, want omitted or nil for invalid metadata", provider)
	}
}
