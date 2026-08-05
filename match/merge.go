package match

import (
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/models"
)

var (
	mergeMultiSpaceRe = regexp.MustCompile(`[ \t]{3,}`)
	mergeBlankLinesRe = regexp.MustCompile(`\n{3,}`)
)

// cleanName is what gets stored: surrounding whitespace removed and internal
// runs collapsed to a single space. Case is untouched.
//
// Collapsing matters for the same reason trimming does — Stash lookups are by
// exact name, so "Nikki  Nuttz" never matches the existing performer.
func cleanName(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// normName is the deduplication key: cleanName, case-folded. Two spellings that
// differ only in case or spacing are one entry.
func normName(s string) string {
	return strings.ToLower(cleanName(s))
}

// appendNames adds each cleaned name to out unless its canonical key was
// already seen, recording keys in seen.
func appendNames(out []string, seen map[string]bool, names []string) []string {
	for _, n := range names {
		clean := cleanName(n)
		if clean == "" {
			continue
		}
		key := normName(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}
	return out
}

func cleanDescription(s string) string {
	s = mergeMultiSpaceRe.ReplaceAllString(s, "\n")
	s = mergeBlankLinesRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// MergedScene holds the combined metadata from one or more FSS scenes,
// ready to be applied to a Stash scene.
type MergedScene struct {
	Title       string
	Description string
	Date        time.Time
	URLs        []string
	Tags        []string
	Categories  []string
	Performers  []string
	Studio      string
	Thumbnail   string
	Duration    int
	Width       int
	Height      int
	Resolution  string
	Sites       []string // which site IDs contributed
}

// MergeScenes combines metadata from multiple FSS scenes (potentially from
// different sites) into a single MergedScene. Optionally incorporates the
// existing Stash scene date for earliest-date logic.
//
// Performer, tag, category and studio names are trimmed before deduplication,
// and entries that trim to nothing are dropped.
//
// This is not cosmetic tidying. Several site APIs return names with stray
// whitespace — Aylo serves "Nikki Nuttz " with a trailing space — and every
// downstream consumer compares these strings exactly:
//
//   - deduplication here is by exact string, so "Nikki Nuttz " from one site and
//     "Nikki Nuttz" from another survived as two separate performers, which
//     defeats the point of merging across sites;
//   - `fss stash import` looks performers, tags and studios up in Stash **by
//     name**, so the untrimmed variant never matches the existing entity and
//     `--apply` creates a duplicate with a trailing space.
//
// Trimming here rather than only in the scrapers also repairs catalogues that
// were already written to disk with the stray whitespace, which no scraper-side
// fix can do without a full re-scrape. Scrapers should still avoid emitting it.
//
// Deduplication is by canonical key (see normName): case-folded, with runs of
// whitespace collapsed. The *stored* value keeps its original case — sites
// legitimately differ on capitalisation and folding it would change which name
// is written to Stash — but "Big Tits" and "big tits" from two sites are one
// entry, not two.
func MergeScenes(scenes []models.Scene, existingDate time.Time) MergedScene {
	m := MergedScene{}

	urlSet := map[string]bool{}
	tagSet := map[string]bool{}
	catSet := map[string]bool{}
	perfSet := map[string]bool{}
	siteSet := map[string]bool{}

	for _, s := range scenes {
		if m.Title == "" && s.Title != "" {
			m.Title = s.Title
		}
		if desc := cleanDescription(s.Description); len(desc) > len(m.Description) {
			m.Description = desc
		}

		if !s.Date.IsZero() && (m.Date.IsZero() || s.Date.Before(m.Date)) {
			m.Date = s.Date
		}

		if s.URL != "" && !urlSet[s.URL] {
			urlSet[s.URL] = true
			m.URLs = append(m.URLs, s.URL)
		}

		m.Tags = appendNames(m.Tags, tagSet, s.Tags)
		m.Categories = appendNames(m.Categories, catSet, s.Categories)
		m.Performers = appendNames(m.Performers, perfSet, s.Performers)

		// Studio is looked up in Stash by name too (checkStudio), so it gets the
		// same treatment.
		if studio := cleanName(s.Studio); m.Studio == "" && studio != "" {
			m.Studio = studio
		}

		if m.Thumbnail == "" && s.Thumbnail != "" {
			m.Thumbnail = s.Thumbnail
		}

		if s.Duration > m.Duration {
			m.Duration = s.Duration
		}

		if s.Width > m.Width {
			m.Width = s.Width
			m.Height = s.Height
			m.Resolution = s.Resolution
		}

		if !siteSet[s.SiteID] {
			siteSet[s.SiteID] = true
			m.Sites = append(m.Sites, s.SiteID)
		}
	}

	if !existingDate.IsZero() && (m.Date.IsZero() || existingDate.Before(m.Date)) {
		m.Date = existingDate
	}

	return m
}

// ResolutionTags returns the single highest resolution tag for the video width.
func ResolutionTags(width int) []string {
	switch {
	case width >= 3840:
		return []string{"4K Available"}
	case width >= 1920:
		return []string{"Full HD Available"}
	case width >= 1280:
		return []string{"HD Available"}
	default:
		return nil
	}
}

// MergeStrings returns the ordered union of two slices, preserving the order
// from existing first, then appending any incoming entries not already present.
func MergeStrings(existing, incoming []string) []string {
	seen := make(map[string]bool, len(existing))
	result := make([]string, 0, len(existing)+len(incoming))
	for _, s := range existing {
		seen[s] = true
		result = append(result, s)
	}
	for _, s := range incoming {
		if !seen[s] {
			seen[s] = true // else a repeat within incoming is appended twice
			result = append(result, s)
		}
	}
	return result
}
