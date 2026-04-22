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
}

func defaults() *Config {
	return &Config{
		Workers: 3,
		Output:  "json",
		OutDir:  ".",
		DB:      "",
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
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}
