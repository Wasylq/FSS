package config

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// captureLog redirects the standard logger for the duration of fn.
func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	orig := log.Writer()
	flags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(orig)
		log.SetFlags(flags)
	})
	fn()
	return buf.String()
}

// A typo'd key is otherwise completely silent: yaml.Unmarshal drops it and the
// default stays in force, so the setting looks applied but never is.
func TestWarnUnknownConfigKeysReportsTypo(t *testing.T) {
	out := captureLog(t, func() {
		warnUnknownConfigKeys([]byte("dealy: 800\n"), "/tmp/config.yaml")
	})

	if !strings.Contains(out, "dealy") {
		t.Errorf("warning did not name the unknown key; got %q", out)
	}
	if !strings.Contains(out, "/tmp/config.yaml") {
		t.Errorf("warning did not name the config path; got %q", out)
	}
}

func TestWarnUnknownConfigKeysSilentOnValidConfig(t *testing.T) {
	out := captureLog(t, func() {
		warnUnknownConfigKeys([]byte("workers: 4\ndelay: 800ms\n"), "/tmp/config.yaml")
	})
	if out != "" {
		t.Errorf("valid config produced a warning: %q", out)
	}
}

func TestWarnUnknownConfigKeysSilentOnEmptyConfig(t *testing.T) {
	out := captureLog(t, func() {
		warnUnknownConfigKeys(nil, "/tmp/config.yaml")
	})
	if out != "" {
		t.Errorf("empty config produced a warning: %q", out)
	}
}

// Malformed YAML is already reported by the real Unmarshal in Load; this must
// not emit a second, differently-worded complaint about the same problem.
func TestWarnUnknownConfigKeysQuietOnMalformedYAML(t *testing.T) {
	out := captureLog(t, func() {
		warnUnknownConfigKeys([]byte("workers: [unclosed\n"), "/tmp/config.yaml")
	})
	if out != "" {
		t.Errorf("malformed YAML produced a duplicate warning: %q", out)
	}
}

// An unknown key must be a warning, not a hard failure: a config that has
// worked for months should keep working, and may carry settings from a newer
// version.
func TestUnknownKeyDoesNotBreakParsing(t *testing.T) {
	cfg := defaults()
	raw := []byte("workers: 7\nnot_a_real_setting: yes\n")

	if err := yaml.Unmarshal(raw, cfg); err != nil {
		t.Fatalf("unknown key made parsing fail: %v", err)
	}
	if cfg.Workers != 7 {
		t.Errorf("Workers = %d, want 7 — known keys must still apply", cfg.Workers)
	}
}
