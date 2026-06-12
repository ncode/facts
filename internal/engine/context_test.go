package engine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestSessionCommandOutputHonorsCancelledContext(t *testing.T) {
	s := NewSessionContext(cancelledContext())

	if got := s.commandOutput("echo", "unreachable"); got != "" {
		t.Fatalf("commandOutput() = %q, want empty for cancelled context", got)
	}
}

func TestLoadExternalCommandFactsHonorsCancelledContext(t *testing.T) {
	ctx := cancelledContext()
	s := NewSessionContext(ctx)

	facts, err := loadExternalCommandFacts(s, "echo", "echo", "unreachable=fact")
	if !errors.Is(err, ctx.Err()) {
		t.Fatalf("loadExternalCommandFacts() err = %v, want %v", err, ctx.Err())
	}
	if facts != nil {
		t.Fatalf("loadExternalCommandFacts() = %#v, want nil for cancelled context", facts)
	}
}

func TestCloudMetadataClientsHonorCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s with cancelled context", r.URL.Path)
	}))
	defer server.Close()
	ctx := cancelledContext()

	if got := newAzureClient(server.URL, server.Client()).metadata(ctx); len(got) != 0 {
		t.Fatalf("azure metadata = %#v, want empty for cancelled context", got)
	}
	if got := newGCEClient(server.URL, server.Client()).metadata(ctx); len(got) != 0 {
		t.Fatalf("gce metadata = %#v, want empty for cancelled context", got)
	}
	if got := newEC2Client(server.URL, server.Client()).metadata(ctx); len(got) != 0 {
		t.Fatalf("ec2 metadata = %#v, want empty for cancelled context", got)
	}
}
