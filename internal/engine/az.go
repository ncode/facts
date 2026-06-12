package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	azureMetadataBaseURL = "http://169.254.169.254"
	azureAPIVersion      = "2020-09-01"
	azureRequestTimeout  = 5 * time.Second
	azureMaxBodyBytes    = 1 << 20
)

type azureClient struct {
	baseURL    string
	httpClient *http.Client
}

func newAzureClient(baseURL string, httpClient *http.Client) *azureClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: azureRequestTimeout,
			Transport: &http.Transport{
				Proxy: nil,
			},
		}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ac.baseURL+"/metadata/instance?api-version="+azureAPIVersion, nil)
	if err != nil {
		return map[string]any{}
	}
	req.Header.Set("Metadata", "true")
	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return map[string]any{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return map[string]any{}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, azureMaxBodyBytes))
	if err != nil {
		return map[string]any{}
	}
	metadata := map[string]any{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return map[string]any{}
	}
	return metadata
}
