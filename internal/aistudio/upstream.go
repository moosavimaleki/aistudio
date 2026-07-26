package aistudio

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type Upstream struct {
	Version     int               `yaml:"version"`
	AIStudio    map[string]string `yaml:"aistudio"`
	Runtime     map[string]string `yaml:"runtime"`
	MakerSuite  map[string]string `yaml:"makersuite"`
	Drive       map[string]string `yaml:"drive"`
	Attestation map[string]string `yaml:"attestation"`
	Opaque      map[string]string `yaml:"opaque"`
}

var upstreamOnce sync.Once
var upstreamConfig Upstream
var upstreamError error

func LoadUpstream() (Upstream, error) {
	upstreamOnce.Do(func() {
		path := os.Getenv("AISTUDIO_UPSTREAM_CONFIG")
		if path == "" {
			for _, candidate := range []string{"/app/config/upstream.yaml", "config/upstream.yaml"} {
				if _, err := os.Stat(candidate); err == nil {
					path = candidate
					break
				}
			}
		}
		if path == "" {
			upstreamError = fmt.Errorf("config/upstream.yaml was not found")
			return
		}
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			upstreamError = err
			return
		}
		if err := yaml.Unmarshal(data, &upstreamConfig); err != nil {
			upstreamError = err
			return
		}
		if upstreamConfig.Version != 1 {
			upstreamError = fmt.Errorf("invalid upstream config: %s", path)
		}
	})
	return upstreamConfig, upstreamError
}

func mustValue(section map[string]string, name string) string {
	value := section[name]
	if value == "" {
		panic("missing upstream config value: " + name)
	}
	return value
}
