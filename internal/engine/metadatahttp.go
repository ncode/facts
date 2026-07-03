package engine

import (
	"context"
	"io"
	"net/http"
	"time"
)

// metadataMaxBodyBytes caps every cloud metadata response read. The link-local
// metadata endpoints return small documents; the cap fails closed against a
// hostile or misrouted endpoint streaming an unbounded body.
const metadataMaxBodyBytes = 1 << 20

// newMetadataHTTPClient builds the proxy-less client every cloud provider uses
// for link-local metadata. Proxy is nil so a configured HTTP(S)_PROXY cannot
// redirect a 169.254.x.x metadata request off-host.
func newMetadataHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{Proxy: nil},
	}
}

// fetchMetadata performs one metadata request and fails closed: any transport
// error, a non-200 status, or a body read error yields ok=false. The body is
// capped at metadataMaxBodyBytes and returned untrimmed (callers trim as their
// contract requires). respHeader is returned so callers can enforce a required
// response header (GCE's Metadata-Flavor echo); it is nil when ok is false.
func fetchMetadata(ctx context.Context, client *http.Client, method, url string, headers map[string]string) (string, http.Header, bool) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return "", nil, false
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, metadataMaxBodyBytes))
	if err != nil {
		return "", nil, false
	}
	return string(data), resp.Header, true
}
