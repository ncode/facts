package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkEC2Metadata(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "PUT /latest/api/token":
			_, _ = w.Write([]byte("v2-token"))
		case "GET /latest/meta-data/":
			_, _ = w.Write([]byte("instance_type\nami_id\nsecurity-groups\nnetwork/"))
		case "GET /latest/meta-data/instance_type":
			_, _ = w.Write([]byte("c1.medium"))
		case "GET /latest/meta-data/ami_id":
			_, _ = w.Write([]byte("ami-5d2dc934"))
		case "GET /latest/meta-data/security-groups":
			_, _ = w.Write([]byte("group1\ngroup2"))
		case "GET /latest/meta-data/network/":
			_, _ = w.Write([]byte("interfaces/"))
		case "GET /latest/meta-data/network/interfaces/":
			_, _ = w.Write([]byte("macs/"))
		case "GET /latest/meta-data/network/interfaces/macs/":
			_, _ = w.Write([]byte("12:34:56:78:9a:bc/"))
		case "GET /latest/meta-data/network/interfaces/macs/12:34:56:78:9a:bc/":
			_, _ = w.Write([]byte("accountId"))
		case "GET /latest/meta-data/network/interfaces/macs/12:34:56:78:9a:bc/accountId":
			_, _ = w.Write([]byte("41234"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	b.ReportAllocs()
	for b.Loop() {
		client := newEC2Client(server.URL+"/latest", server.Client())
		metadata := client.metadata(context.Background())
		if metadata["ami_id"] != "ami-5d2dc934" {
			b.Fatalf("metadata[ami_id] = %#v, want ami-5d2dc934", metadata["ami_id"])
		}
	}
}
