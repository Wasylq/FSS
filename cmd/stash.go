package cmd

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"sync"

	"github.com/spf13/cobra"
)

var stashCmd = &cobra.Command{
	Use:   "stash",
	Short: "Interact with a Stash instance",
}

func init() {
	rootCmd.AddCommand(stashCmd)

	stashCmd.PersistentFlags().String("url", "", "Stash server URL (default from config)")
	stashCmd.PersistentFlags().String("api-key", "", "Stash API key (env: FSS_STASH_API_KEY)")
}

func stashURL(cmd *cobra.Command) string {
	u, _ := cmd.Flags().GetString("url")
	if u == "" {
		u = cfg.Stash.URL
	}
	warnIfRemoteStash(u)
	return u
}

var remoteStashOnce sync.Once

func warnIfRemoteStash(rawURL string) {
	if rawURL == "" {
		return
	}
	remoteStashOnce.Do(func() {
		u, err := url.Parse(rawURL)
		if err != nil {
			return
		}
		host := u.Hostname()
		if host == "" {
			return
		}
		if ip := net.ParseIP(host); ip != nil {
			if !isLocalOrPrivate(ip) {
				fmt.Fprintf(os.Stderr, "[warn] stash URL %q points to a non-local address; API key will be sent there\n", rawURL)
			}
			return
		}
		ips, err := net.LookupIP(host)
		if err != nil {
			return
		}
		for _, ip := range ips {
			if !isLocalOrPrivate(ip) {
				fmt.Fprintf(os.Stderr, "[warn] stash URL %q resolves to non-local IP %s; API key will be sent there\n", rawURL, ip)
				return
			}
		}
	})
}

func isLocalOrPrivate(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}

func stashAPIKey(cmd *cobra.Command) string {
	k, _ := cmd.Flags().GetString("api-key")
	if k != "" {
		return k
	}
	if env := os.Getenv("FSS_STASH_API_KEY"); env != "" {
		return env
	}
	return cfg.Stash.APIKey
}
