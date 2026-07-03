package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewMetadataHTTPClientIsProxyLess(t *testing.T) {
	client := newMetadataHTTPClient(3 * time.Second)
	if client.Timeout != 3*time.Second {
		t.Fatalf("Timeout = %v, want 3s", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T, want *http.Transport", client.Transport)
	}
	// A nil Proxy func means link-local metadata requests cannot be redirected
	// off-host by a configured HTTP(S)_PROXY.
	if transport.Proxy != nil {
		t.Fatal("Transport.Proxy != nil, want proxy-less client")
	}
}

func TestFetchMetadataReturnsBodyAndHeadersOn200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Token"); got != "secret" {
			t.Fatalf("X-Token = %q, want secret (headers not passed through)", got)
		}
		w.Header().Set("Metadata-Flavor", "Google")
		_, _ = w.Write([]byte("  raw-body  "))
	}))
	t.Cleanup(server.Close)

	body, header, ok := fetchMetadata(context.Background(), server.Client(), http.MethodGet, server.URL, map[string]string{"X-Token": "secret"})
	if !ok {
		t.Fatal("ok = false, want true for 200")
	}
	// The body is returned untrimmed; trimming is the caller's contract.
	if body != "  raw-body  " {
		t.Fatalf("body = %q, want untrimmed raw-body", body)
	}
	if header.Get("Metadata-Flavor") != "Google" {
		t.Fatalf("response header Metadata-Flavor = %q, want Google", header.Get("Metadata-Flavor"))
	}
}

func TestFetchMetadataFailsClosedOnNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	body, header, ok := fetchMetadata(context.Background(), server.Client(), http.MethodGet, server.URL, nil)
	if ok || body != "" || header != nil {
		t.Fatalf("fetchMetadata(403) = (%q, %v, %v), want fail-closed", body, header, ok)
	}
}

func TestFetchMetadataFailsClosedOnRequestBuildError(t *testing.T) {
	// A control character in the URL makes http.NewRequestWithContext fail
	// before any network call.
	body, header, ok := fetchMetadata(context.Background(), http.DefaultClient, http.MethodGet, "http://\x7f/bad", nil)
	if ok || body != "" || header != nil {
		t.Fatalf("fetchMetadata(bad url) = (%q, %v, %v), want fail-closed", body, header, ok)
	}
}

func TestFetchMetadataFailsClosedOnTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close() // nothing is listening now

	if _, _, ok := fetchMetadata(context.Background(), server.Client(), http.MethodGet, url, nil); ok {
		t.Fatal("ok = true, want fail-closed on transport error")
	}
}

func TestFetchMetadataCapsBodyAtOneMebibyte(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", metadataMaxBodyBytes+4096)))
	}))
	t.Cleanup(server.Close)

	body, _, ok := fetchMetadata(context.Background(), server.Client(), http.MethodGet, server.URL, nil)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if len(body) != metadataMaxBodyBytes {
		t.Fatalf("body length = %d, want capped at %d", len(body), metadataMaxBodyBytes)
	}
}
