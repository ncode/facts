package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	azureMetadataBaseURL = "http://169.254.169.254"
	azureAPIVersion      = "2020-09-01"
	azureRequestTimeout  = 5 * time.Second
)

type azureClient struct {
	baseURL    string
	httpClient *http.Client
}

func newAzureClient(baseURL string, httpClient *http.Client) *azureClient {
	if httpClient == nil {
		httpClient = newMetadataHTTPClient(azureRequestTimeout)
	}
	return &azureClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

func azureFacts(ctx context.Context, client *azureClient, virt virtualization) []ResolvedFact {
	if client == nil || !azureHypervisor(virt.Name) {
		return []ResolvedFact{{Name: "az_metadata", Value: nil}}
	}
	metadata := client.metadata(ctx)
	if len(metadata) == 0 {
		return []ResolvedFact{{Name: "az_metadata", Value: nil}}
	}
	return []ResolvedFact{
		{Name: "az_metadata", Value: metadata},
		{Name: "cloud.provider", Value: "azure"},
	}
}

func azureHypervisor(name string) bool {
	return strings.EqualFold(name, "hyperv")
}

func (ac *azureClient) metadata(ctx context.Context) map[string]any {
	body, _, ok := fetchMetadata(ctx, ac.httpClient, http.MethodGet, ac.baseURL+"/metadata/instance?api-version="+azureAPIVersion, map[string]string{
		"Metadata": "true",
	})
	if !ok {
		return map[string]any{}
	}
	metadata := map[string]any{}
	if err := json.Unmarshal([]byte(body), &metadata); err != nil {
		return map[string]any{}
	}
	return metadata
}
