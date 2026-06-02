package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/config"
	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/internal/store"
	"github.com/Wasylq/FSS/scraper"
	"github.com/Wasylq/FSS/stash"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check environment health: config, database, Stash, tools",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStdout()
	allOK := true

	check := func(name string, fn func() (string, bool)) {
		detail, ok := fn()
		status := "ok"
		if !ok {
			status = "FAIL"
			allOK = false
		}
		if detail != "" {
			_, _ = fmt.Fprintf(w, "  %-22s %s  (%s)\n", name, status, detail)
		} else {
			_, _ = fmt.Fprintf(w, "  %-22s %s\n", name, status)
		}
	}

	_, _ = fmt.Fprintln(w, "Config:")
	check("config file", func() (string, bool) {
		path := config.DefaultPath()
		if _, err := os.Stat(path); err != nil {
			return path + " — not found", false
		}
		return path, true
	})
	check("scrapers registered", func() (string, bool) {
		all := scraper.All()
		return fmt.Sprintf("%d", len(all)), len(all) > 0
	})

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Database:")
	check("SQLite", func() (string, bool) {
		dbPath := ""
		if cfg != nil {
			dbPath = cfg.DB
		}
		if dbPath == "" {
			return "disabled (flat JSON mode)", true
		}
		if dbPath == "default" {
			dbPath = config.DefaultDBPath()
		}
		s, err := store.NewSQLite(dbPath)
		if err != nil {
			return dbPath + " — " + err.Error(), false
		}
		_ = s.Close()
		return dbPath, true
	})
	check("output directory", func() (string, bool) {
		dir := "."
		if cfg != nil && cfg.OutDir != "" {
			dir = cfg.OutDir
		}
		info, err := os.Stat(dir)
		if err != nil {
			return dir + " — not found", false
		}
		if !info.IsDir() {
			return dir + " — not a directory", false
		}
		f, err := os.CreateTemp(dir, ".fss-doctor-*")
		if err != nil {
			return dir + " — not writable", false
		}
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		return dir, true
	})

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Stash:")
	check("stash connection", func() (string, bool) {
		u := stashURL(cmd)
		if u == "" {
			return "not configured", true
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		c := stash.NewClient(u, stashAPIKey(cmd))
		if err := c.Ping(ctx); err != nil {
			return u + " — not reachable", false
		}
		return u, true
	})

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Tools:")
	check("ffprobe", func() (string, bool) {
		path, err := exec.LookPath("ffprobe")
		if err != nil {
			return "not found (optional, for fss identify)", true
		}
		return path, true
	})

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Network:")
	check("HTTP egress", func() (string, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := httpx.Do(ctx, client, httpx.Request{
			URL:         "https://api.github.com/zen",
			MaxAttempts: 1,
		})
		if err != nil {
			return err.Error(), false
		}
		_ = resp.Body.Close()
		return "github.com reachable", true
	})

	_, _ = fmt.Fprintln(w)
	if allOK {
		_, _ = fmt.Fprintln(w, "All checks passed.")
	} else {
		_, _ = fmt.Fprintln(w, "Some checks failed — see above.")
	}
	return nil
}
