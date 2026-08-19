package cmd

import (
	"testing"
)

func TestLangFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"space form", []string{"--lang", "ko"}, "ko"},
		{"equals form", []string{"--lang=ko"}, "ko"},
		{"missing value at end", []string{"--lang"}, ""},
		{"value looks like a flag", []string{"--lang", "--debug"}, ""},
		{"stops at double dash", []string{"--", "--lang=ko"}, ""},
		{"after subcommand", []string{"scrape", "--lang", "ko"}, "ko"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := langFromArgs(tc.args); got != tc.want {
				t.Errorf("langFromArgs(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestResolveLanguage(t *testing.T) {
	isolateXDG(t) // no config file present, so the config tier never fires

	t.Run("FSS_LANG beats LC_ALL", func(t *testing.T) {
		t.Setenv("FSS_LANG", "ko")
		t.Setenv("LC_ALL", "fr_FR.UTF-8")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "")
		tag, explicit := resolveLanguage()
		if tag != "ko" || !explicit {
			t.Errorf("resolveLanguage() = (%q, %v), want (\"ko\", true)", tag, explicit)
		}
	})

	t.Run("LC_ALL beats LANG", func(t *testing.T) {
		t.Setenv("FSS_LANG", "")
		t.Setenv("LC_ALL", "fr_FR.UTF-8")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "de_DE.UTF-8")
		tag, explicit := resolveLanguage()
		if tag != "fr_FR.UTF-8" || explicit {
			t.Errorf("resolveLanguage() = (%q, %v), want (\"fr_FR.UTF-8\", false)", tag, explicit)
		}
	})

	t.Run("LANG is ambient, not explicit", func(t *testing.T) {
		t.Setenv("FSS_LANG", "")
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "ko_KR.UTF-8")
		tag, explicit := resolveLanguage()
		if tag != "ko_KR.UTF-8" || explicit {
			t.Errorf("resolveLanguage() = (%q, %v), want (\"ko_KR.UTF-8\", false)", tag, explicit)
		}
	})

	t.Run("all empty", func(t *testing.T) {
		t.Setenv("FSS_LANG", "")
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "")
		tag, explicit := resolveLanguage()
		if tag != "" || explicit {
			t.Errorf("resolveLanguage() = (%q, %v), want (\"\", false)", tag, explicit)
		}
	})
}
