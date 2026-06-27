package facts

import (
	"os"
	"strings"
	"testing"
)

func TestHugoSiteContract(t *testing.T) {
	checkFileContains(t, "hugo.toml",
		`baseURL = "https://facts.martinez.io/"`,
		`contentDir = "docs"`,
		`publishDir = "public"`,
		`disableKinds = ["rss", "taxonomy", "term"]`,
		`dir = ":cacheDir/images"`,
	)
	checkFileExcludes(t, "hugo.toml", "theme =", "[module]")

	checkFileContains(t, "static/CNAME", "facts.martinez.io")
	checkFileContains(t, ".gitignore", "/public/", "/.hugo_build.lock")

	checkFileContains(t, "layouts/index.html",
		"Facts",
		"Go port of Puppet Facter",
		"go get github.com/ncode/facts",
		"brew install ncode/tap/facts",
		"facts --json os.family kernel.version.full",
		`site.GetPage "/supported-facts"`,
		".RegularPages",
	)

	checkFileContains(t, "layouts/_default/baseof.html",
		"css/site.css",
		"Start",
		"Library",
		"CLI",
		"Supported facts",
		"Project",
	)
	checkFileContains(t, "layouts/_default/single.html", ".Content")
	checkFileContains(t, "layouts/_default/list.html", ".RegularPages", `site.GetPage "/supported-facts/readme"`)
	checkFileContains(t, "layouts/partials/docs-nav.html", `site.GetPage "/supported-facts"`, ".RegularPages")
	checkFileContains(t, "layouts/partials/page-title.html", "RawContent", `findRE`)
	checkFileContains(t, "layouts/_default/_markup/render-link.html",
		`/supported-facts/%s/`,
		`docs/supported-facts/`,
		`docs/schema/facts.yaml`,
		`github.com/ncode/facts/blob/main`,
	)
	checkFileExcludes(t, "layouts/_default/_markup/render-link.html", "safeURL")
	checkFileExcludes(t, "layouts/index.html", `RelPermalink "/supported-facts/readme/"`)

	checkFileContains(t, "static/css/site.css",
		"#171717",
		"#fafafa",
		"#4d4d4d",
		"#666666",
		"#ebebeb",
		"#007cf0",
		"#00dfd8",
		"#7928ca",
		"#ff0080",
		"#ff4d4d",
		"#f9cb28",
		"SFMono-Regular",
		":focus-visible",
		"scroll-margin-top",
		"text-underline-offset",
	)
	checkFileExcludes(t, "static/css/site.css", "@import", "https://", "http://")

	checkFileContains(t, ".github/workflows/pages.yaml",
		"workflow_dispatch:",
		"actions/configure-pages@",
		"actions/upload-pages-artifact@",
		"actions/deploy-pages@",
		"hugo --cleanDestinationDir --minify",
		"pages: write",
		"id-token: write",
		"path: ./public",
	)
}

func checkFileContains(t *testing.T, path string, needles ...string) {
	t.Helper()
	body := readTextFile(t, path)
	for _, needle := range needles {
		if !strings.Contains(body, needle) {
			t.Fatalf("%s missing %q", path, needle)
		}
	}
}

func checkFileExcludes(t *testing.T, path string, needles ...string) {
	t.Helper()
	body := readTextFile(t, path)
	for _, needle := range needles {
		if strings.Contains(body, needle) {
			t.Fatalf("%s unexpectedly contains %q", path, needle)
		}
	}
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
