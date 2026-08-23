package extensionbuild

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hamed/aistudio-api/internal/aistudio"
)

var bundles = map[string][]string{
	"page.js": {
		"shared/protocol.js",
		"page/provider-store.js",
		"page/snapshot-service.js",
		"page/main.js",
	},
	"content.js": {
		"shared/protocol.js",
		"content/keep-alive.js",
		"content/page-channel.js",
		"content/main.js",
	},
	"chatgpt-page.js": {
		"shared/protocol.js",
		"chatgpt/sse.js",
		"chatgpt/page.js",
	},
	"chatgpt-content.js": {
		"shared/protocol.js",
		"content/keep-alive.js",
		"chatgpt/channel.js",
		"chatgpt/composer.js",
		"chatgpt/content.js",
	},
}

func Build(source string, upstream aistudio.Upstream) error {
	if err := buildManifest(source, upstream.AIStudio["origin"]); err != nil {
		return err
	}
	output := filepath.Join(source, "dist")
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	for name, inputs := range bundles {
		parts := make([]string, 0, len(inputs)+1)
		if name == "page.js" {
			config, err := pageConfig(upstream)
			if err != nil {
				return err
			}
			parts = append(parts, config)
		}
		for _, input := range inputs {
			content, err := os.ReadFile(filepath.Join(source, input))
			if err != nil {
				return err
			}
			parts = append(parts, string(content))
		}
		if err := os.WriteFile(
			filepath.Join(output, name),
			[]byte(strings.Join(parts, "\n\n")+"\n"),
			0644,
		); err != nil {
			return err
		}
	}
	return nil
}

func pageConfig(upstream aistudio.Upstream) (string, error) {
	config := map[string]string{
		"runtimeGlobal":         upstream.Runtime["global_object"],
		"apiKeyProperty":        upstream.Runtime["api_key_property"],
		"visitIdProperty":       upstream.Runtime["visit_id_property"],
		"attestationProperty":   upstream.Runtime["attestation_enabled_property"],
		"attestationNamespace":  upstream.Attestation["namespace"],
		"attestationEntrypoint": upstream.Attestation["entrypoint"],
		"digestProperty":        upstream.Attestation["digest_property"],
	}
	for name, value := range config {
		if value == "" {
			return "", fmt.Errorf("missing extension config value: %s", name)
		}
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return "globalThis.AISTUDIO_UPSTREAM_CONFIG = " + string(encoded) + ";", nil
}

func buildManifest(source, origin string) error {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return fmt.Errorf("aistudio.origin must contain a valid origin")
	}
	labels := strings.Split(parsed.Hostname(), ".")
	if len(labels) < 2 {
		return fmt.Errorf("aistudio.origin must contain a parent domain")
	}
	parentDomain := strings.Join(labels[len(labels)-2:], ".")
	template, err := os.ReadFile(filepath.Join(source, "manifest.template.json"))
	if err != nil {
		return err
	}
	manifest := strings.ReplaceAll(string(template), "__AISTUDIO_MATCH__", origin+"/*")
	manifest = strings.ReplaceAll(
		manifest,
		"__AISTUDIO_GOOGLE_PERMISSION__",
		parsed.Scheme+"://*."+parentDomain+"/*",
	)
	return os.WriteFile(filepath.Join(source, "manifest.json"), []byte(manifest), 0644)
}
