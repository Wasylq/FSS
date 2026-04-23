package stash

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

		for _, t := range s.Tags {
			if !tagSet[t] {
				tagSet[t] = true
				m.Tags = append(m.Tags, t)
			}
		}

		for _, c := range s.Categories {
			if !catSet[c] {
				catSet[c] = true
				m.Categories = append(m.Categories, c)
			}
		}

		for _, p := range s.Performers {
			if !perfSet[p] {
				perfSet[p] = true
				m.Performers = append(m.Performers, p)
			}
		}

		if m.Studio == "" && s.Studio != "" {
			m.Studio = s.Studio
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

// MergeURLs returns the union of existing and new URLs.
func MergeURLs(existing, new []string) []string {
	seen := make(map[string]bool, len(existing))
	result := make([]string, 0, len(existing)+len(new))
	for _, u := range existing {
		seen[u] = true
		result = append(result, u)
	}
	for _, u := range new {
		if !seen[u] {
			result = append(result, u)
		}
	}
	return result
}

// MergeTagIDs returns the union of existing and new tag IDs.
func MergeTagIDs(existing, new []string) []string {
	seen := make(map[string]bool, len(existing))
	result := make([]string, 0, len(existing)+len(new))
	for _, id := range existing {
		seen[id] = true
		result = append(result, id)
	}
	for _, id := range new {
		if !seen[id] {
			result = append(result, id)
		}
	}
	return result
}
