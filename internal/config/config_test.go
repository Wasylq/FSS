package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSanitizeWindowsPaths(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "windows absolute path",
			in:   `db: "C:\Users\nlaci\bin\fss.db"`,
			want: `db: "C:/Users/nlaci/bin/fss.db"`,
		},
		{
			name: "multiple path fields",
			in:   "out_dir: \"D:\\data\\fss\"\ndb: \"C:\\Users\\nlaci\\fss.db\"",
			want: "out_dir: \"D:/data/fss\"\ndb: \"C:/Users/nlaci/fss.db\"",
		},
		{
			name: "single-quoted path unchanged",
			in:   `db: 'C:\Users\nlaci\fss.db'`,
			want: `db: 'C:\Users\nlaci\fss.db'`,
		},
		{
			name: "unquoted path unchanged",
			in:   `db: C:\Users\nlaci\fss.db`,
			want: `db: C:\Users\nlaci\fss.db`,
		},
		{
			name: "unix path unchanged",
			in:   `db: "/home/user/fss.db"`,
			want: `db: "/home/user/fss.db"`,
		},
		{
			name: "url unchanged",
			in:   `url: "http://localhost:9999"`,
			want: `url: "http://localhost:9999"`,
		},
		{
			name: "empty string unchanged",
			in:   `db: ""`,
			want: `db: ""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(sanitizeWindowsPaths([]byte(tt.in)))
			if got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestWindowsPathsRoundTrip(t *testing.T) {
	raw := []byte(`workers: 5
output: csv
out_dir: "D:\data\fss\output"
db: "C:\Users\nlaci\bin\fss.db"
delay: 500

site_delays:
  manyvids: 100

stash:
  url: "http://192.168.1.50:9999"
  api_key: "secret"
  tag: "my_import"
  stashbox_tag: "my_override"
  resolution_tags: false
`)

	// Without sanitization, yaml.Unmarshal fails on \U in "C:\Users\..."
	var discard Config
	if err := yaml.Unmarshal(raw, &discard); err == nil {
		t.Fatal("expected raw YAML with Windows paths to fail, but it didn't")
	}

	// With sanitization, it should decode correctly.
	sanitized := sanitizeWindowsPaths(raw)
	var cfg Config
	if err := yaml.Unmarshal(sanitized, &cfg); err != nil {
		t.Fatalf("Unmarshal after sanitization: %v", err)
	}

	if cfg.Workers != 5 {
		t.Errorf("Workers = %d, want 5", cfg.Workers)
	}
	if cfg.Output != "csv" {
		t.Errorf("Output = %q, want csv", cfg.Output)
	}
	if cfg.OutDir != "D:/data/fss/output" {
		t.Errorf("OutDir = %q, want D:/data/fss/output", cfg.OutDir)
	}
	if cfg.DB != "C:/Users/nlaci/bin/fss.db" {
		t.Errorf("DB = %q, want C:/Users/nlaci/bin/fss.db", cfg.DB)
	}
	if cfg.Delay != 500 {
		t.Errorf("Delay = %d, want 500", cfg.Delay)
	}
	if cfg.SiteDelays["manyvids"] != 100 {
		t.Errorf("SiteDelays[manyvids] = %d, want 100", cfg.SiteDelays["manyvids"])
	}
	if cfg.Stash.URL != "http://192.168.1.50:9999" {
		t.Errorf("Stash.URL = %q, want http://192.168.1.50:9999", cfg.Stash.URL)
	}
	if cfg.Stash.APIKey != "secret" {
		t.Errorf("Stash.APIKey = %q, want secret", cfg.Stash.APIKey)
	}
	if cfg.Stash.Tag != "my_import" {
		t.Errorf("Stash.Tag = %q, want my_import", cfg.Stash.Tag)
	}
	if cfg.Stash.StashboxTag != "my_override" {
		t.Errorf("Stash.StashboxTag = %q, want my_override", cfg.Stash.StashboxTag)
	}
	if cfg.Stash.ResolutionTags {
		t.Error("Stash.ResolutionTags = true, want false")
	}
}
