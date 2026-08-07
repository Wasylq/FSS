package cmd

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/config"
)

func isolateXDG(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CONFIG_DIRS", dir)
	old, oldDirs := xdg.ConfigHome, xdg.ConfigDirs
	xdg.Reload()
	t.Cleanup(func() {
		xdg.ConfigHome, xdg.ConfigDirs = old, oldDirs
	})
}

func initConfig(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&buf)
	if err := runConfigInit(c, nil); err != nil {
		t.Fatalf("config init: %v", err)
	}
	return buf.String()
}

// The file `fss config init` writes is what every new user starts from, so it
// has to survive the loader that reads it back.
func TestConfigInitProducesALoadableConfig(t *testing.T) {
	isolateXDG(t)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(nil) })

	initConfig(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("loading the config we just shipped: %v", err)
	}

	// A key the shipped file still sets but Config no longer knows about is
	// only reported through this warning — Load itself ignores it.
	if strings.Contains(logs.String(), "unknown settings") {
		t.Errorf("shipped config has keys FSS no longer understands: %s", logs.String())
	}

	if cfg.Workers != 3 {
		t.Errorf("Workers = %d, want 3", cfg.Workers)
	}
	if cfg.Output != "json" {
		t.Errorf("Output = %q, want json", cfg.Output)
	}
	if cfg.Delay != 500 {
		t.Errorf("Delay = %d, want 500", cfg.Delay)
	}
	if cfg.OutDir != "." {
		t.Errorf("OutDir = %q, want .", cfg.OutDir)
	}
	if cfg.Stash.URL != "http://localhost:9999" {
		t.Errorf("Stash.URL = %q", cfg.Stash.URL)
	}
	if cfg.Stash.Tag != "fss_import" {
		t.Errorf("Stash.Tag = %q, want fss_import", cfg.Stash.Tag)
	}

	// `db: ""` in the shipped file must read back as set-but-empty, or the
	// documented opt-out stops working the moment the default flips.
	value, set := cfg.DBSetting()
	if !set || value != "" {
		t.Errorf("DBSetting() = (%q, %v), want (\"\", true)", value, set)
	}
}
