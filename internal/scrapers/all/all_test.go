package all

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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
