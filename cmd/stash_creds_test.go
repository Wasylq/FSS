package cmd

import (
	"net"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Anastylosis/FSS/internal/config"
)

// isLocalOrPrivate gates the "your API key is about to be sent off-host" warning.
// A false positive here silences that warning for a genuinely remote address, so
// each branch is pinned rather than sampled.
func TestIsLocalOrPrivate(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},      // loopback
		{"::1", true},            // loopback v6
		{"10.1.2.3", true},       // RFC1918
		{"172.16.0.1", true},     // RFC1918
		{"172.31.255.254", true}, // RFC1918 upper bound
		{"192.168.1.10", true},   // RFC1918
		{"fd00::1", true},        // unique local v6
		{"169.254.1.1", true},    // link-local
		{"fe80::1", true},        // link-local v6
		{"0.0.0.0", true},        // unspecified
		{"::", true},             // unspecified v6

		// Public addresses must NOT be treated as local.
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"172.32.0.1", false},     // just outside RFC1918
		{"172.15.255.255", false}, // just below RFC1918
		{"93.184.216.34", false},
		{"2606:4700:4700::1111", false},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("test bug: %q is not a valid IP", c.ip)
		}
		if got := isLocalOrPrivate(ip); got != c.want {
			t.Errorf("isLocalOrPrivate(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
}

func newStashTestCmd() *cobra.Command {
	c := &cobra.Command{Use: "x"}
	c.Flags().String("url", "", "")
	c.Flags().String("api-key", "", "")
	return c
}

// The API key has three sources with a defined precedence. Getting this wrong
// either ignores the operator's explicit flag or leaks a config key when they
// meant to override it.
func TestStashAPIKeyPrecedence(t *testing.T) {
	orig := cfg
	t.Cleanup(func() { cfg = orig })
	cfg = &config.Config{Stash: config.StashConfig{APIKey: "from-config"}}

	t.Run("flag wins over env and config", func(t *testing.T) {
		t.Setenv("FSS_STASH_API_KEY", "from-env")
		c := newStashTestCmd()
		if err := c.Flags().Set("api-key", "from-flag"); err != nil {
			t.Fatal(err)
		}
		if got := stashAPIKey(c); got != "from-flag" {
			t.Errorf("stashAPIKey = %q, want from-flag", got)
		}
	})

	t.Run("env wins over config", func(t *testing.T) {
		t.Setenv("FSS_STASH_API_KEY", "from-env")
		if got := stashAPIKey(newStashTestCmd()); got != "from-env" {
			t.Errorf("stashAPIKey = %q, want from-env", got)
		}
	})

	t.Run("config is the fallback", func(t *testing.T) {
		t.Setenv("FSS_STASH_API_KEY", "")
		if got := stashAPIKey(newStashTestCmd()); got != "from-config" {
			t.Errorf("stashAPIKey = %q, want from-config", got)
		}
	})
}

func TestStashURLPrefersFlagOverConfig(t *testing.T) {
	orig := cfg
	t.Cleanup(func() { cfg = orig })
	cfg = &config.Config{Stash: config.StashConfig{URL: "http://localhost:9999"}}

	if got := stashURL(newStashTestCmd()); got != "http://localhost:9999" {
		t.Errorf("stashURL = %q, want the config value", got)
	}

	c := newStashTestCmd()
	if err := c.Flags().Set("url", "http://127.0.0.1:1234"); err != nil {
		t.Fatal(err)
	}
	if got := stashURL(c); got != "http://127.0.0.1:1234" {
		t.Errorf("stashURL = %q, want the flag value", got)
	}
}

// An empty URL must not trip the remote warning or panic on a nil parse.
func TestWarnIfRemoteStashIgnoresEmpty(t *testing.T) {
	warnIfRemoteStash("")
	warnIfRemoteStash("::::not a url")
}
