package config

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

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
	SiteDelays map[string]int   `yaml:"site_delays"`
	Stash      StashConfig      `yaml:"stash"`
	Stashbox   []StashboxConfig `yaml:"stashbox"`
}

type StashboxConfig struct {
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
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
	return filepath.Join(xdg.ConfigHome, "fss", "config.yaml")
}

// windowsPathRe matches double-quoted YAML values that look like Windows
// absolute paths (drive letter followed by :\). YAML treats backslashes as
// escape characters inside double-quoted strings, so "C:\Users" fails
// because \U is parsed as a Unicode escape. We replace \ with / before
// decoding — Go's filepath functions accept forward slashes on all platforms.
var windowsPathRe = regexp.MustCompile(`"([A-Za-z]:\\[^"]*)"`)

func sanitizeWindowsPaths(data []byte) []byte {
	return windowsPathRe.ReplaceAllFunc(data, func(match []byte) []byte {
		out := make([]byte, len(match))
		copy(out, match)
		for i, b := range out {
			if b == '\\' {
				out[i] = '/'
			}
		}
		return out
	})
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

	if runtime.GOOS != "windows" {
		if info, err := f.Stat(); err == nil && info.Mode().Perm()&0o077 != 0 {
			log.Printf("warning: %s is readable by other users (mode %04o); consider chmod 600", path, info.Mode().Perm())
		}
	}

	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	raw = sanitizeWindowsPaths(raw)

	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Workers < 0 {
		return fmt.Errorf("workers must be non-negative, got %d", c.Workers)
	}
	if c.Delay < 0 {
		return fmt.Errorf("delay must be non-negative, got %d", c.Delay)
	}
	for name, d := range c.SiteDelays {
		if d < 0 {
			return fmt.Errorf("site_delays[%s] must be non-negative, got %d", name, d)
		}
	}
	if c.Output != "" {
		for _, f := range strings.Split(c.Output, ",") {
			f = strings.TrimSpace(f)
			if f != "" && f != "json" && f != "csv" {
				return fmt.Errorf("unknown output format %q (valid: json, csv)", f)
			}
		}
	}
	return nil
}
