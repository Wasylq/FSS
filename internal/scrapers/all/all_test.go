package all

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// TestReadmeScraperCount keeps the "**N sites**" claim in README.md in sync
// with the actual scraper registry. When the count drifts, the test
// auto-updates README.md and fails so the diff shows up in git.
func TestReadmeScraperCount(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")
	readmePath := filepath.Join(repoRoot, "README.md")

	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}

	re := regexp.MustCompile(`\*\*(\d+) sites\*\*`)
	m := re.FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal(`README.md is missing the "**N sites**" marker that this test reads`)
	}
	claimed, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("parsing site count from README: %v", err)
	}

	actual := len(scraper.All())
	if claimed != actual {
		updated := re.ReplaceAll(data, []byte(fmt.Sprintf("**%d sites**", actual)))
		if err := os.WriteFile(readmePath, updated, 0o644); err != nil {
			t.Fatalf("auto-updating README.md: %v", err)
		}
		t.Errorf("README.md claimed %d sites; registry has %d. Auto-updated README.md — commit the change.", claimed, actual)
	}
}

// TestSitesMdInSync regenerates docs/sites.md from the live scraper registry
// and fails (after writing the fresh file) when the on-disk copy doesn't match.
// docs/sites.md is the canonical "all covered domains, alphabetically searchable"
// reference — keeping it auto-generated means it can't go stale.
func TestSitesMdInSync(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")
	sitesPath := filepath.Join(repoRoot, "docs", "sites.md")

	want := buildSitesMd(scraper.All())

	got, err := os.ReadFile(sitesPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("reading docs/sites.md: %v", err)
	}

	if !bytes.Equal(got, want) {
		if err := os.WriteFile(sitesPath, want, 0o644); err != nil {
			t.Fatalf("auto-updating docs/sites.md: %v", err)
		}
		t.Errorf("docs/sites.md was out of sync with the scraper registry — auto-updated, commit the change.")
	}
}

// buildSitesMd produces the full canonical content of docs/sites.md from the
// given scrapers. One row per unique domain — when multiple scraper IDs share
// a domain (e.g. all 97 adultprime sub-studios on adultprime.com), the IDs
// are folded into a single cell.
//
// stashbox is excluded: it's the only config-driven scraper in the registry,
// so the domains it reports depend on whatever instances the developer
// running the test has in their local config.yaml. Including it would make
// sites.md drift between local runs and CI. stashbox is documented
// separately in docs/scrapers.md.
func buildSitesMd(scrapers []scraper.StudioScraper) []byte {
	// Collect IDs per domain, preserving order seen + deduping per (domain, id).
	idsByDomain := map[string][]string{}
	rendered := 0
	for _, s := range scrapers {
		if s.ID() == "stashbox" {
			continue
		}
		rendered++
		seen := map[string]bool{}
		for _, p := range s.Patterns() {
			d := domainOfPattern(p)
			if d == "" || seen[d] {
				continue
			}
			seen[d] = true
			idsByDomain[d] = append(idsByDomain[d], s.ID())
		}
	}
	domains := make([]string, 0, len(idsByDomain))
	for d := range idsByDomain {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	for _, d := range domains {
		sort.Strings(idsByDomain[d])
	}

	var b bytes.Buffer
	fmt.Fprintln(&b, "# Covered Sites — All Domains")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "%d scrapers covering %d distinct domains. Auto-generated from the scraper registry by `TestSitesMdInSync` — do not hand-edit; instead update the scraper's `Patterns()` and re-run `go test ./internal/scrapers/all/...`. The `stashbox` scraper is omitted here because its covered hosts are config-driven; see [`scrapers.md`](scrapers.md).\n", rendered, len(domains))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Use your editor's find-in-file (Ctrl+F / Cmd+F) to look up a domain. The **Scraper ID(s)** column lists every scraper that claims URLs on that domain — most domains have one entry, but table-driven networks (`adultprime`, `tmw`, `nextdoorstudios`, etc.) host many sub-studios on a single root domain. See [`scrapers.md`](scrapers.md) for what each ID covers.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Domain | Scraper ID(s) |")
	fmt.Fprintln(&b, "|--------|---------------|")
	for _, d := range domains {
		fmt.Fprintf(&b, "| %s | %s |\n", d, formatIDs(idsByDomain[d]))
	}
	return b.Bytes()
}

// formatIDs renders a domain's scraper-ID list as a markdown table cell.
// One ID: backticked. 2-3 IDs: comma-separated. >3: first three + "(+N more)".
func formatIDs(ids []string) string {
	const maxShown = 3
	tick := func(s string) string { return "`" + s + "`" }
	if len(ids) <= maxShown {
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = tick(id)
		}
		return strings.Join(out, ", ")
	}
	parts := make([]string, maxShown)
	for i := 0; i < maxShown; i++ {
		parts[i] = tick(ids[i])
	}
	return fmt.Sprintf("%s (+%d more)", strings.Join(parts, ", "), len(ids)-maxShown)
}

// domainOfPattern extracts the host from a scraper Pattern string.
// Patterns generally look like "example.com" or "example.com/some/path", but a
// handful ship full URLs like "https://example.org/...".
func domainOfPattern(p string) string {
	p = strings.TrimSpace(p)
	// Strip leading scheme if present.
	if i := strings.Index(p, "://"); i >= 0 {
		p = p[i+3:]
	}
	// First path segment is the host.
	if i := strings.Index(p, "/"); i >= 0 {
		p = p[:i]
	}
	return strings.ToLower(p)
}
