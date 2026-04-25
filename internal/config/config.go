package config

import (
	"fmt"
	"os"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Workers int    `yaml:"workers"`
	Output  string `yaml:"output"`
	OutDir  string `yaml:"out_dir"`
	DB      string `yaml:"db"`
	Delay   int    `yaml:"delay"`
	// SiteDelays overrides Delay per scraper ID (e.g. "manyvids", "pornhub").
	// Sites without an entry fall back to Delay.
	SiteDelays map[string]int `yaml:"site_delays"`
	Stash      StashConfig    `yaml:"stash"`
}

type StashConfig struct {
	URL            string `yaml:"url"`
	APIKey         string `yaml:"api_key"`
	Tag            string `yaml:"tag"`
	StashboxTag    string `yaml:"stashbox_tag"`
	ResolutionTags bool   `yaml:"resolution_tags"`
}

func defaults() *Config {
	return &Config{
		Workers: 3,
		Output:  "json",
		OutDir:  ".",
		DB:      "",
		Delay:   0,
		Stash: StashConfig{
			URL:            "http://localhost:9999",
			Tag:            "fss_import",
			StashboxTag:    "fss_stashbox_override",
			ResolutionTags: true,
		},
	}
}

// DefaultPath returns the canonical config file path for the current platform.
// The file may not exist yet — this is where it should be created.
func DefaultPath() string {
	path, _ := xdg.ConfigFile("fss/config.yaml")
	return path
}

// Load reads the config file from the XDG config directory.
// If no file exists, defaults are returned without error.
func Load() (*Config, error) {
	cfg := defaults()

	path, err := xdg.SearchConfigFile("fss/config.yaml")
	if err != nil {
		// No config file found — use defaults.
		return cfg, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening config %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}
