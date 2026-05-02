package all

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// TestReadmeScraperCount fails when README.md's "**N sites**" claim diverges
// from the number of scrapers actually registered through this package. Bump
// the README when this fires; the registry is authoritative.
func TestReadmeScraperCount(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")

	data, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
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
		t.Errorf("README.md claims %d sites; registry has %d. Update README.md to **%d sites**.", claimed, actual, actual)
	}
}
