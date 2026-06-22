package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ncode/facts/internal/cli"
)

func TestCLIOptionDocumentationIncludesAcceptedNonHiddenOptions(t *testing.T) {
	installedManPage, err := os.ReadFile(filepath.Join("..", "..", "man", "man8", "facts.8"))
	if err != nil {
		t.Fatal(err)
	}

	surfaces := []struct {
		name string
		text string
	}{
		{name: "help", text: helpText()},
		{name: "man", text: manText()},
		{name: "installed man page", text: normalizeManPage(string(installedManPage))},
	}

	for _, option := range cli.Options() {
		if option.Hidden {
			continue
		}
		names := append([]string{option.Canonical}, option.Aliases...)
		for _, surface := range surfaces {
			for _, name := range names {
				if !strings.Contains(surface.text, name) {
					t.Fatalf("%s output missing documented option %q:\n%s", surface.name, name, surface.text)
				}
			}
		}
	}
}

func normalizeManPage(text string) string {
	replacer := strings.NewReplacer(
		`\fB`, "",
		`\fR`, "",
		`\-`, "-",
		`\.`, ".",
		`\&`, "",
	)
	return replacer.Replace(text)
}
