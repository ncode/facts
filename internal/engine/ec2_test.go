package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestEC2Metadata_fetchesRecursiveMetadataWithIMDSv2Token(t *testing.T) {
	requests := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.Method+" "+r.URL.Path] = r.Header.Get("X-aws-ec2-metadata-token")
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

	client := newEC2Client(server.URL+"/latest", server.Client())
	got := client.metadata(context.Background())
	want := map[string]any{
		"instance_type":   "c1.medium",
		"ami_id":          "ami-5d2dc934",
		"security-groups": "group1\ngroup2",
		"network": map[string]any{
			"interfaces": map[string]any{
				"macs": map[string]any{
					"12:34:56:78:9a:bc": map[string]any{
						"accountId": "41234",
					},
				},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata() = %#v, want %#v", got, want)
	}
	if requests["GET /latest/meta-data/ami_id"] != "v2-token" {
		t.Fatalf("metadata request token = %q, want v2-token", requests["GET /latest/meta-data/ami_id"])
	}
}

func TestEC2Metadata_refreshesExpiredIMDSv2Token(t *testing.T) {
	putRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "PUT /latest/api/token":
			putRequests++
			_, _ = w.Write([]byte("v2-token"))
		case "GET /latest/meta-data/":
			_, _ = w.Write([]byte("ami_id"))
		case "GET /latest/meta-data/ami_id":
			_, _ = w.Write([]byte("ami-5d2dc934"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	client := newEC2Client(server.URL+"/latest", server.Client())
	client.tokenTTL = time.Second
	client.now = func() time.Time { return now }

	client.metadata(context.Background())
	now = now.Add(2 * time.Second)
	client.metadata(context.Background())

	if putRequests != 2 {
		t.Fatalf("IMDSv2 token PUT requests = %d, want 2", putRequests)
	}
}

func TestEC2Metadata_fallsBackToIMDSv1AndKeepsMissingLeafEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-aws-ec2-metadata-token") != "" {
			t.Fatalf("unexpected IMDS token header %q", r.Header.Get("X-aws-ec2-metadata-token"))
		}
		switch r.Method + " " + r.URL.Path {
		case "PUT /latest/api/token":
			http.NotFound(w, r)
		case "GET /latest/meta-data/":
			_, _ = w.Write([]byte("instance_type\nami_id"))
		case "GET /latest/meta-data/instance_type":
			http.NotFound(w, r)
		case "GET /latest/meta-data/ami_id":
			_, _ = w.Write([]byte("ami-5d2dc934"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newEC2Client(server.URL+"/latest", server.Client())
	got := client.metadata(context.Background())
	want := map[string]any{"instance_type": "", "ami_id": "ami-5d2dc934"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata() = %#v, want %#v", got, want)
	}
}

func TestEC2Userdata_returnsEmptyOnHTTPError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	client := newEC2Client(server.URL+"/latest", server.Client())
	if got := client.userdata(context.Background()); got != "" {
		t.Fatalf("userdata() = %q, want empty string", got)
	}
}

func TestEC2Facts_onlyReturnsFactsForAWSHypervisors(t *testing.T) {
	client := &ec2Client{baseURL: "http://127.0.0.1", httpClient: &http.Client{Timeout: time.Nanosecond}}
	got := ec2Facts(testSession, client, virtualization{Name: "docker", IsVirtual: true})
	want := []ResolvedFact{
		{Name: "ec2_metadata", Value: nil},
		{Name: "ec2_userdata", Value: nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ec2Facts(testSession, docker) = %#v, want %#v", got, want)
	}
}

func TestEC2Facts_returnsNilFactsForEmptyAWSMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newEC2Client(server.URL+"/latest", server.Client())
	got := ec2Facts(testSession, client, virtualization{Name: "kvm", IsVirtual: true})
	want := []ResolvedFact{
		{Name: "ec2_metadata", Value: nil},
		{Name: "ec2_userdata", Value: nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ec2Facts(testSession, empty AWS metadata) = %#v, want %#v", got, want)
	}
}

func TestEC2Facts_returnsMetadataAndUserdataForAWSHypervisors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "PUT /latest/api/token":
			_, _ = w.Write([]byte("v2-token"))
		case "GET /latest/meta-data/":
			_, _ = w.Write([]byte("instance_type"))
		case "GET /latest/meta-data/instance_type":
			_, _ = w.Write([]byte("c1.medium"))
		case "GET /latest/user-data/":
			_, _ = w.Write([]byte("  #!/bin/sh\necho userdata\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newEC2Client(server.URL+"/latest", server.Client())
	facts := ec2Facts(testSession, client, virtualization{Name: "aws", IsVirtual: true})
	assertDescriptorDeclaresFacts(t, "ec2_metadata", facts)
	got := Collection(facts)
	metadata, ok := got["ec2_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("ec2_metadata = %#v, want metadata map", got["ec2_metadata"])
	}
	if got, want := metadata["instance_type"], "c1.medium"; got != want {
		t.Fatalf("ec2_metadata.instance_type = %#v, want %#v", got, want)
	}
	if got, want := got["ec2_userdata"], "  #!/bin/sh\necho userdata\n"; got != want {
		t.Fatalf("ec2_userdata = %#v, want %#v", got, want)
	}
	if fact := factByName(facts, "cloud.provider"); fact == nil || fact.Value != "aws" {
		t.Fatalf("cloud.provider fact = %#v, want aws", fact)
	}
}

func factByName(facts []ResolvedFact, name string) *ResolvedFact {
	for i := range facts {
		if facts[i].Name == name {
			return &facts[i]
		}
	}
	return nil
}

func TestCloudProviderFact_returnsAWSForEC2MetadataOnAWSHypervisor(t *testing.T) {
	got := cloudProviderFact(testSession, virtualization{Name: "kvm", IsVirtual: true}, map[string]any{"instance_type": "c1.medium"})
	if got == nil || got.Name != "cloud.provider" || got.Value != "aws" {
		t.Fatalf("cloudProviderFact(testSession) = %#v, want aws provider fact", got)
	}
}

func TestCloudProviderFact_skipsEmptyEC2Metadata(t *testing.T) {
	got := cloudProviderFact(testSession, virtualization{Name: "kvm", IsVirtual: true}, map[string]any{})
	if got != nil {
		t.Fatalf("cloudProviderFact(testSession) = %#v, want nil", got)
	}
}

func TestCloudProviderFactForPlatformRequiresVirtWhatAWSOnLinuxRootKVM(t *testing.T) {
	metadata := map[string]any{"instance_type": "c1.medium"}
	executable := func(path string) bool {
		if path != "/opt/puppetlabs/puppet/bin/virt-what" {
			t.Fatalf("executable path = %q, want virt-what path", path)
		}
		return true
	}

	got := cloudProviderFactForPlatform("linux", virtualization{Name: "kvm", IsVirtual: true}, metadata, 0, executable, func(string, ...string) string {
		return "kvm\n"
	})
	if got != nil {
		t.Fatalf("cloudProviderFactForPlatform(linux root kvm) = %#v, want nil", got)
	}

	got = cloudProviderFactForPlatform("linux", virtualization{Name: "kvm", IsVirtual: true}, metadata, 0, executable, func(string, ...string) string {
		return "kvm\naws\n"
	})
	if got == nil || got.Name != "cloud.provider" || got.Value != "aws" {
		t.Fatalf("cloudProviderFactForPlatform(linux root aws) = %#v, want aws provider fact", got)
	}
}

func TestCloudProviderFactForPlatformUsesMetadataOnNonLinuxAWSHypervisor(t *testing.T) {
	got := cloudProviderFactForPlatform("darwin", virtualization{Name: "xen", IsVirtual: true}, map[string]any{"instance_type": "c1.medium"}, 0, func(string) bool {
		t.Fatal("non-linux provider detection must not inspect virt-what")
		return false
	}, func(string, ...string) string {
		t.Fatal("non-linux provider detection must not run virt-what")
		return ""
	})
	if got == nil || got.Name != "cloud.provider" || got.Value != "aws" {
		t.Fatalf("cloudProviderFactForPlatform(non-linux) = %#v, want aws provider fact", got)
	}
}

func TestLinuxAWSCloudProviderRequiresAWSHypervisorAndMetadata(t *testing.T) {
	metadata := map[string]any{"instance_type": "c1.medium"}
	executable := func(string) bool {
		t.Fatal("linuxAWSCloudProvider() checked virt-what before basic guards")
		return false
	}
	run := func(string, ...string) string {
		t.Fatal("linuxAWSCloudProvider() ran virt-what before basic guards")
		return ""
	}

	if linuxAWSCloudProvider("docker", metadata, 0, executable, run) {
		t.Fatal("linuxAWSCloudProvider(non-AWS hypervisor) = true, want false")
	}
	if linuxAWSCloudProvider("kvm", nil, 0, executable, run) {
		t.Fatal("linuxAWSCloudProvider(empty metadata) = true, want false")
	}
}

func TestLinuxAWSCloudProviderRequiresVirtWhatAWSForRootKVM(t *testing.T) {
	metadata := map[string]any{"instance_type": "c1.medium"}
	executable := func(string) bool { return true }
	run := func(string, ...string) string { return "kvm" }

	if linuxAWSCloudProvider("kvm", metadata, 0, executable, run) {
		t.Fatal("linuxAWSCloudProvider(kvm root virt-what=kvm) = true, want false")
	}

	run = func(string, ...string) string { return "aws" }
	if !linuxAWSCloudProvider("kvm", metadata, 0, executable, run) {
		t.Fatal("linuxAWSCloudProvider(kvm root virt-what=aws) = false, want true")
	}

	run = func(string, ...string) string { return "kvm\naws\n" }
	if !linuxAWSCloudProvider("kvm", metadata, 0, executable, run) {
		t.Fatal("linuxAWSCloudProvider(kvm root virt-what includes aws) = false, want true")
	}

	if !linuxAWSCloudProvider("kvm", metadata, 512, executable, func(string, ...string) string { return "kvm" }) {
		t.Fatal("linuxAWSCloudProvider(kvm non-root) = false, want true")
	}
}

func TestFileExecutableRequiresRegularExecutableFile(t *testing.T) {
	host := &fakeHostOS{
		stats: map[string]os.FileInfo{
			"/bin/virt-what":      fakeFileInfo{name: "virt-what", mode: 0o700},
			"/bin/not-executable": fakeFileInfo{name: "not-executable", mode: 0o600},
			"/bin":                fakeFileInfo{name: "bin", mode: os.ModeDir | 0o755, isDir: true},
		},
	}

	if !fileExecutable(host, "/bin/virt-what") {
		t.Fatal("fileExecutable(executable) = false, want true")
	}
	if fileExecutable(host, "/bin/not-executable") {
		t.Fatal("fileExecutable(non-executable) = true, want false")
	}
	if fileExecutable(host, "/bin") {
		t.Fatal("fileExecutable(directory) = true, want false")
	}
	if fileExecutable(host, "/bin/missing") {
		t.Fatal("fileExecutable(missing) = true, want false")
	}
}

func TestLinuxGCEFacts_fetchesMetadataAndCloudProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Metadata-Flavor"); got != "Google" {
			t.Fatalf("Metadata-Flavor = %q, want Google", got)
		}
		w.Header().Set("Metadata-Flavor", "Google")
		if r.URL.String() != "/computeMetadata/v1/?recursive=true&alt=json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"instance":{"id":"123456789","machineType":"projects/123/machineTypes/e2-medium","zone":"projects/123/zones/us-central1-a","attributes":{"role":"web"}}}`))
	}))
	defer server.Close()

	got := linuxGCEFacts(context.Background(), "linux", "Google", newGCEClient(server.URL+"/computeMetadata/v1", server.Client()))
	want := []ResolvedFact{
		{Name: "gce", Value: map[string]any{
			"instance": map[string]any{
				"id":          "123456789",
				"machineType": "e2-medium",
				"zone":        "us-central1-a",
				"attributes": map[string]any{
					"role": "web",
				},
			},
		}},
		{Name: "cloud.provider", Value: "gce"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linuxGCEFacts(context.Background(), ) = %#v, want %#v", got, want)
	}
}
