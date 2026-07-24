package output

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/models"
)

// WriteJSON writes a StudioFile as indented JSON, using atomic file replacement
// to prevent corruption on crash.
func WriteJSON(sf models.StudioFile, path string) error {
	return atomicWriteFile(path, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(sf)
	})
}

// CSVHeaders defines the column order for CSV exports. Multi-value fields use
// | as separator. PriceHistory is serialised as a JSON string.
var CSVHeaders = []string{
	"id", "siteId", "studioUrl",
	"title", "url", "date", "description",
	"thumbnail", "preview",
	"performers", "director", "studio",
	"tags", "categories",
	"series", "seriesPart",
	"duration", "resolution", "width", "height", "format",
	"views", "likes", "comments",
	"lowestPrice", "lowestPriceDate", "priceHistory",
	"scrapedAt", "deletedAt",
}

// WriteCSV writes scenes as CSV with a header row, using atomic file replacement.
func WriteCSV(scenes []models.Scene, path string) error {
	return atomicWriteFile(path, func(out io.Writer) error {
		w := csv.NewWriter(out)
		if err := w.Write(CSVHeaders); err != nil {
			return err
		}
		for _, s := range scenes {
			row, err := sceneToRow(s)
			if err != nil {
				return err
			}
			for i := range row {
				row[i] = escapeCSVFormula(row[i])
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
		w.Flush()
		return w.Error()
	})
}

// Slugify converts a studio URL to a safe filename stem. It appends a short
// hash of the raw URL so that distinct URLs never collide on the same slug —
// e.g. "anna_b" and "anna-b" both sanitize to "anna-b" but get different
// hashes — and so that URLs whose host+path sanitize to the empty string
// (e.g. purely non-ASCII paths) still produce a unique, non-empty stem.
//
// Changing this output renames Flat-store files; the Flat store migrates the
// legacy (un-hashed) filename on read — see LegacySlugify.
func Slugify(rawURL string) string {
	base := slugBase(rawURL)
	sum := sha256.Sum256([]byte(rawURL))
	hash := hex.EncodeToString(sum[:])[:8]
	if base == "" {
		return hash
	}
	return base + "-" + hash
}

// LegacySlugify reproduces the pre-hash Slugify output. The Flat store uses it
// to find and migrate files written before the hash suffix was introduced.
func LegacySlugify(rawURL string) string {
	return slugBase(rawURL)
}

// slugBase sanitizes the host+path of a URL (the human-readable slug prefix).
func slugBase(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return sanitize(rawURL)
	}
	return sanitize(u.Hostname() + u.Path)
}

// SweepStaleTempFiles removes leftover `.fss-tmp-*` files in dir whose
// last-modified time is older than maxAge. These are orphans from a
// previous `atomicWriteFile` whose process died (SIGKILL, OOM, power
// loss) between `os.CreateTemp` and the deferred `os.Remove`. The age
// guard prevents racing with a concurrent live writer — a `--db` SQLite
// scrape can sit holding an open temp for a long time, so we only
// remove files clearly stale enough that no in-flight write could own
// them.
//
// Returns the number of files removed. Errors stat'ing or removing a
// single file are not propagated; this is a best-effort cleanup and a
// failed remove just means a tiny disk leak, not a correctness issue.
// Missing directories are not an error — Flat-store first run.
func SweepStaleTempFiles(dir string, maxAge time.Duration) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, ".fss-tmp-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
		removed++
	}
	return removed
}

func atomicWriteFile(path string, writeFn func(io.Writer) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".fss-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	if err := writeFn(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming %s → %s: %w", tmpPath, path, err)
	}
	_ = os.Chmod(path, 0o600)
	// POSIX makes the rename atomic w.r.t. concurrent readers but does
	// NOT guarantee the new directory entry is on disk before a crash.
	// fsync the parent directory so the rename survives a power loss.
	// Best-effort: on Windows (and some FUSE filesystems) opening a
	// directory for Sync isn't supported; treat ENOTDIR/permission
	// errors as non-fatal since the write already succeeded.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

// escapeCSVFormula defuses spreadsheet formula injection. Scene titles and
// descriptions are attacker-controlled — they come from scraped pages — and a
// cell beginning =, +, - or @ is evaluated as a formula by Excel, LibreOffice
// and Google Sheets when the CSV is opened, which can exfiltrate data or invoke
// external calls. Leading tab and carriage return are included because they are
// stripped before the formula check by some spreadsheet parsers.
//
// Prefixing a single quote is the standard mitigation: spreadsheets treat the
// cell as literal text and do not display the quote. The value is unchanged for
// any other cell, so parsers that read the CSV as data see the original string.
//
// Applied in WriteCSV rather than sceneToRow so no future column can bypass it.
func escapeCSVFormula(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return result
}

func sceneToRow(s models.Scene) ([]string, error) {
	ph, err := json.Marshal(s.PriceHistory)
	if err != nil {
		return nil, fmt.Errorf("marshalling price history: %w", err)
	}
	return []string{
		s.ID,
		s.SiteID,
		s.StudioURL,
		s.Title,
		s.URL,
		formatTime(s.Date),
		s.Description,
		s.Thumbnail,
		s.Preview,
		strings.Join(s.Performers, "|"),
		s.Director,
		s.Studio,
		strings.Join(s.Tags, "|"),
		strings.Join(s.Categories, "|"),
		s.Series,
		strconv.Itoa(s.SeriesPart),
		strconv.Itoa(s.Duration),
		s.Resolution,
		strconv.Itoa(s.Width),
		strconv.Itoa(s.Height),
		s.Format,
		strconv.Itoa(s.Views),
		strconv.Itoa(s.Likes),
		strconv.Itoa(s.Comments),
		strconv.FormatFloat(s.LowestPrice, 'f', 2, 64),
		formatTimePtr(s.LowestPriceDate),
		string(ph),
		s.ScrapedAt.Format(time.RFC3339),
		formatTimePtr(s.DeletedAt),
	}, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
