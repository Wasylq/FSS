package cmd

import (
	"testing"

	"github.com/Wasylq/FSS/internal/config"
)

// list-studios used to read only its own --db flag, so an operator who set
// `db:` in config.yaml and scraped with it would be told no database was
// configured.
func TestListStudiosFallsBackToConfigDB(t *testing.T) {
	origFlag, origCfg := listStudiosDB, cfg
	t.Cleanup(func() { listStudiosDB, cfg = origFlag, origCfg })

	cases := []struct {
		name string
		flag string
		conf string
		want string
	}{
		{"flag wins over config", "/from/flag.db", "/from/config.db", "/from/flag.db"},
		{"config used when flag unset", "", "/from/config.db", "/from/config.db"},
		{"neither set", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			listStudiosDB = c.flag
			cfg = &config.Config{DB: c.conf}
			if got := config.ResolveDBPath(resolveListStudiosDB()); got != c.want {
				t.Errorf("resolved db = %q, want %q", got, c.want)
			}
		})
	}
}

// A nil config (no config file loaded) must not panic.
func TestListStudiosNilConfig(t *testing.T) {
	origFlag, origCfg := listStudiosDB, cfg
	t.Cleanup(func() { listStudiosDB, cfg = origFlag, origCfg })

	listStudiosDB, cfg = "", nil
	if got := resolveListStudiosDB(); got != "" {
		t.Errorf("resolved db = %q, want empty", got)
	}
}
