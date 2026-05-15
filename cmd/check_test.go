package cmd

import (
	"bytes"
	"strings"
	"testing"

	_ "github.com/Wasylq/FSS/internal/scrapers/all"
)

func executeCheck(t *testing.T, url string) string {
	t.Helper()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"check", url})

	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestCheckSupported(t *testing.T) {
	out := executeCheck(t, "https://www.brazzers.com/videos")

	if !strings.Contains(out, "Scraper:  brazzers") {
		t.Errorf("output missing scraper ID:\n%s", out)
	}
	if !strings.Contains(out, "Patterns:") {
		t.Errorf("output missing patterns:\n%s", out)
	}
}

func TestCheckUnsupported(t *testing.T) {
	out := executeCheck(t, "https://example.com/not-a-site")

	if !strings.Contains(out, "Not supported") {
		t.Errorf("output missing 'Not supported':\n%s", out)
	}
	if !strings.Contains(out, "Request support:") {
		t.Errorf("output missing issue link:\n%s", out)
	}
	if !strings.Contains(out, "template=new_scraper.yml") {
		t.Errorf("output missing issue template param:\n%s", out)
	}
}
