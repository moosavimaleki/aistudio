package extensionbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

func TestBuild(t *testing.T) {
	source := t.TempDir()
	writeFixture(t, source, "manifest.template.json",
		`{"matches":["__AISTUDIO_MATCH__"],"hosts":["__AISTUDIO_GOOGLE_PERMISSION__"]}`)
	for _, name := range append(bundles["page.js"], bundles["content.js"]...) {
		writeFixture(t, source, name, "// "+name)
	}

	upstream := aistudio.Upstream{
		AIStudio: map[string]string{"origin": "https://aistudio.google.com"},
		Runtime: map[string]string{
			"global_object":                "runtime",
			"api_key_property":             "apiKey",
			"visit_id_property":            "visitID",
			"attestation_enabled_property": "enabled",
		},
		Attestation: map[string]string{
			"namespace":       "attestation",
			"entrypoint":      "create",
			"digest_property": "digest",
		},
	}
	if err := Build(source, upstream); err != nil {
		t.Fatal(err)
	}

	assertContains(t, filepath.Join(source, "manifest.json"), "https://*.google.com/*")
	assertContains(t, filepath.Join(source, "dist/page.js"), "AISTUDIO_UPSTREAM_CONFIG")
	assertContains(t, filepath.Join(source, "dist/content.js"), "// content/main.js")
}

func writeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertContains(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), expected) {
		t.Fatalf("%s does not contain %q", path, expected)
	}
}
