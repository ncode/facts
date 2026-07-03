package engine

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ec2MetadataBaseURL = "http://169.254.169.254/latest"
	ec2RequestTimeout  = 100 * time.Millisecond
	ec2MaxDepth        = 16
	ec2TokenTTL        = 21600 * time.Second
)

type ec2Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	tokenUntil time.Time
	tokenTTL   time.Duration
	now        func() time.Time
}

func newEC2Client(baseURL string, httpClient *http.Client) *ec2Client {
	if httpClient == nil {
		httpClient = newMetadataHTTPClient(ec2RequestTimeout)
	}
	return &ec2Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		tokenTTL:   ec2TokenTTL,
		now:        time.Now,
	}
}

func ec2Facts(s *Session, client *ec2Client, virt virtualization) []ResolvedFact {
	if client == nil || !awsHypervisor(virt.Name) {
		return []ResolvedFact{
			{Name: "ec2_metadata", Value: nil},
			{Name: "ec2_userdata", Value: nil},
		}
	}
	facts := make([]ResolvedFact, 0, 3)
	metadata := client.metadata(s.Context())
	if len(metadata) == 0 {
		facts = append(facts, ResolvedFact{Name: "ec2_metadata", Value: nil})
	} else {
		facts = append(facts, ResolvedFact{Name: "ec2_metadata", Value: metadata})
	}
	if provider := cloudProviderFact(s, virt, metadata); provider != nil {
		facts = append(facts, *provider)
	}
	userdata := client.userdata(s.Context())
	if userdata == "" {
		facts = append(facts, ResolvedFact{Name: "ec2_userdata", Value: nil})
	} else {
		facts = append(facts, ResolvedFact{Name: "ec2_userdata", Value: userdata})
	}
	return facts
}

func cloudProviderFact(s *Session, virt virtualization, ec2Metadata map[string]any) *ResolvedFact {
	executable := func(path string) bool { return fileExecutable(s.host, path) }
	return cloudProviderFactForPlatform(s.goos(), virt, ec2Metadata, os.Geteuid(), executable, s.commandOutput)
}

func cloudProviderFactForPlatform(goos string, virt virtualization, ec2Metadata map[string]any, euid int, executable func(string) bool, run func(string, ...string) string) *ResolvedFact {
	if goos == "linux" {
		if !linuxAWSCloudProvider(virt.Name, ec2Metadata, euid, executable, run) {
			return nil
		}
		return &ResolvedFact{Name: "cloud.provider", Value: "aws"}
	}
	if !awsHypervisor(virt.Name) || len(ec2Metadata) == 0 {
		return nil
	}
	return &ResolvedFact{Name: "cloud.provider", Value: "aws"}
}

func linuxAWSCloudProvider(name string, ec2Metadata map[string]any, euid int, executable func(string) bool, run func(string, ...string) string) bool {
	if !awsHypervisor(name) || len(ec2Metadata) == 0 {
		return false
	}
	if strings.EqualFold(name, "aws") || euid != 0 || !executable("/opt/puppetlabs/puppet/bin/virt-what") {
		return true
	}
	for field := range strings.FieldsSeq(run("/opt/puppetlabs/puppet/bin/virt-what")) {
		if strings.EqualFold(field, "aws") {
			return true
		}
	}
	return false
}

func fileExecutable(host hostOS, path string) bool {
	info, err := host.stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

func awsHypervisor(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "kvm") || strings.Contains(name, "xen") || strings.Contains(name, "aws")
}

func (ec *ec2Client) metadata(ctx context.Context) map[string]any {
	value, ok := ec.fetchMetadataPath(ctx, "meta-data/", 0)
	if !ok {
		return map[string]any{}
	}
	metadata, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return metadata
}

func (ec *ec2Client) userdata(ctx context.Context) string {
	body, ok := ec.getRaw(ctx, "user-data/")
	if !ok {
		return ""
	}
	return body
}

func (ec *ec2Client) fetchMetadataPath(ctx context.Context, path string, depth int) (any, bool) {
	if depth > ec2MaxDepth {
		return "", false
	}
	body, ok := ec.get(ctx, path)
	if !ok {
		return "", true
	}
	if !strings.HasSuffix(path, "/") {
		return body, true
	}

	children := metadataChildren(body)
	result := make(map[string]any, len(children))
	for _, child := range children {
		childPath := path + child
		value, ok := ec.fetchMetadataPath(ctx, childPath, depth+1)
		if !ok {
			continue
		}
		key := strings.TrimSuffix(child, "/")
		result[key] = value
	}
	return result, true
}

func metadataChildren(body string) []string {
	lines := strings.Split(body, "\n")
	children := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		children = append(children, line)
	}
	return children
}

func (ec *ec2Client) get(ctx context.Context, path string) (string, bool) {
	body, ok := ec.getRaw(ctx, path)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(body), true
}

func (ec *ec2Client) getRaw(ctx context.Context, path string) (string, bool) {
	var headers map[string]string
	if token := ec.v2Token(ctx); token != "" {
		headers = map[string]string{"X-aws-ec2-metadata-token": token}
	}
	body, _, ok := fetchMetadata(ctx, ec.httpClient, http.MethodGet, ec.baseURL+"/"+path, headers)
	return body, ok
}

func (ec *ec2Client) v2Token(ctx context.Context) string {
	now := time.Now
	if ec.now != nil {
		now = ec.now
	}
	ttl := ec.tokenTTL
	if ttl == 0 {
		ttl = ec2TokenTTL
	}
	if ec.token != "" && now().Before(ec.tokenUntil) {
		return ec.token
	}
	body, _, ok := fetchMetadata(ctx, ec.httpClient, http.MethodPut, ec.baseURL+"/api/token", map[string]string{
		"X-aws-ec2-metadata-token-ttl-seconds": strconv.FormatInt(int64(ttl/time.Second), 10),
	})
	if !ok {
		return ""
	}
	ec.token = strings.TrimSpace(body)
	ec.tokenUntil = now().Add(ttl)
	return ec.token
}
