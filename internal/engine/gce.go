package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	gceMetadataBaseURL = "http://metadata.google.internal/computeMetadata/v1"
	gceRequestTimeout  = 100 * time.Millisecond
)

type gceClient struct {
	baseURL    string
	httpClient *http.Client
}

func newGCEClient(baseURL string, httpClient *http.Client) *gceClient {
	if httpClient == nil {
		httpClient = newMetadataHTTPClient(gceRequestTimeout)
	}
	return &gceClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

func linuxGCEFacts(ctx context.Context, goos, biosVendor string, client *gceClient) []ResolvedFact {
	if goos != "linux" {
		return nil
	}
	if !strings.Contains(biosVendor, "Google") || client == nil {
		return []ResolvedFact{{Name: "gce", Value: nil}}
	}
	metadata := client.metadata(ctx)
	if len(metadata) == 0 {
		return []ResolvedFact{{Name: "gce", Value: nil}}
	}
	return []ResolvedFact{
		{Name: "gce", Value: metadata},
		{Name: "cloud.provider", Value: "gce"},
	}
}

func platformGCEFacts(ctx context.Context, goos string, virtual virtualization, biosVendor string, client *gceClient) []ResolvedFact {
	switch goos {
	case "linux":
		return linuxGCEFacts(ctx, goos, biosVendor, client)
	case "windows":
		if !strings.Contains(virtual.Name, "gce") || client == nil {
			return []ResolvedFact{{Name: "gce", Value: nil}}
		}
		metadata := client.metadata(ctx)
		if len(metadata) == 0 {
			return []ResolvedFact{{Name: "gce", Value: nil}}
		}
		return []ResolvedFact{
			{Name: "gce", Value: metadata},
			{Name: "cloud.provider", Value: "gce"},
		}
	default:
		return nil
	}
}

func (gc *gceClient) metadata(ctx context.Context) map[string]any {
	body, ok := gc.get(ctx, "?recursive=true&alt=json")
	if !ok || body == "" {
		return map[string]any{}
	}
	metadata := map[string]any{}
	if err := json.Unmarshal([]byte(body), &metadata); err != nil {
		return map[string]any{}
	}
	normalizeGCEMetadata(metadata)
	return metadata
}

func normalizeGCEMetadata(metadata map[string]any) {
	instance, ok := metadata["instance"].(map[string]any)
	if !ok || len(instance) == 0 {
		return
	}

	normalizeGCESSHKeys(metadata, "project")
	normalizeGCESSHKeys(metadata, "instance")
	for _, key := range []string{"image", "machineType", "zone"} {
		if value, ok := instance[key].(string); ok {
			instance[key] = lastPathSegment(value)
		}
	}
	interfaces, ok := instance["networkInterfaces"].([]any)
	if !ok || len(interfaces) == 0 {
		return
	}
	primary, ok := interfaces[0].(map[string]any)
	if !ok {
		return
	}
	if network, ok := primary["network"].(string); ok {
		primary["network"] = lastPathSegment(network)
	}
}

func normalizeGCESSHKeys(metadata map[string]any, root string) {
	rootMap, ok := metadata[root].(map[string]any)
	if !ok {
		return
	}
	attributes, ok := rootMap["attributes"].(map[string]any)
	if !ok {
		return
	}
	for _, name := range []string{"sshKeys", "ssh-keys"} {
		keys, ok := attributes[name].(string)
		if !ok {
			continue
		}
		attributes[name] = splitMetadataLines(keys)
	}
}

func splitMetadataLines(value string) []any {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	out := make([]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func lastPathSegment(value string) string {
	parts := strings.Split(strings.TrimRight(value, "/"), "/")
	return parts[len(parts)-1]
}

func (gc *gceClient) get(ctx context.Context, path string) (string, bool) {
	body, header, ok := fetchMetadata(ctx, gc.httpClient, http.MethodGet, gc.baseURL+"/"+path, map[string]string{
		"Metadata-Flavor": "Google",
		"Accept":          "application/json",
	})
	if !ok || header.Get("Metadata-Flavor") != "Google" {
		return "", false
	}
	return strings.TrimSpace(body), true
}
