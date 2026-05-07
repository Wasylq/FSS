package match

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/models"
)

type studioFile struct {
	StudioURL  string         `json:"studioUrl"`
	ScrapedAt  time.Time      `json:"scrapedAt"`
	SceneCount int            `json:"sceneCount"`
	Scenes     []models.Scene `json:"scenes"`
}

type SceneIndex struct {
	byTitle             map[string][]models.Scene
	byTitleSanitized    map[string][]models.Scene // titles with noise words stripped
	byTitleNoTrailingID map[string][]models.Scene // titles with trailing number stripped
	all                 []models.Scene

	titleSubstrIdx        *substringIndex
	sanitizedSubstrIdx    *substringIndex
	noTrailingIDSubstrIdx *substringIndex
}

// substringIndex narrows the substring-match search from O(N_titles) to roughly
// O(rarest-word-bucket-size). Each title is filed under its lowest-frequency
// word; only that bucket is consulted when its key word appears in the
// filename. The full subset check still runs on the (small) shortlist.
type substringIndex struct {
	byRarestWord map[string][]indexedTitle
}

type indexedTitle struct {
	title string
	words []string
}

func newSubstringIndex(titleMap map[string][]models.Scene) *substringIndex {
	freq := make(map[string]int)
	for title := range titleMap {
		for _, w := range strings.Fields(title) {
			freq[w]++
		}
	}
	si := &substringIndex{byRarestWord: make(map[string][]indexedTitle)}
	for title := range titleMap {
		words := strings.Fields(title)
		if len(words) == 0 {
			continue
		}
		rarest := words[0]
		for _, w := range words[1:] {
			if freq[w] < freq[rarest] {
				rarest = w
			}
		}
		si.byRarestWord[rarest] = append(si.byRarestWord[rarest], indexedTitle{
			title: title,
			words: words,
		})
	}
	return si
}

// findCandidateTitles returns titles whose word-set is a subset of the
// filename's word-set and whose word count is ≥ minWordRatio of the filename's
// word count. Reached via the rarest-word inversion rather than a full scan.
func (si *substringIndex) findCandidateTitles(filenameWords []string, minWordRatio float64) []string {
	if len(filenameWords) == 0 {
		return nil
	}
	filenameSet := make(map[string]bool, len(filenameWords))
	for _, w := range filenameWords {
		filenameSet[w] = true
	}
	minLen := float64(len(filenameWords)) * minWordRatio
	seen := make(map[string]bool)
	var titles []string
	for w := range filenameSet {
		for _, it := range si.byRarestWord[w] {
			if seen[it.title] {
				continue
			}
			seen[it.title] = true
			if float64(len(it.words)) < minLen {
				continue
			}
			ok := true
			for _, tw := range it.words {
				if !filenameSet[tw] {
					ok = false
					break
				}
			}
			if ok {
				titles = append(titles, it.title)
			}
		}
	}
	return titles
}

type MatchConfidence int

const (
	MatchExact MatchConfidence = iota
	MatchSubstring
	MatchNone
	MatchAmbiguous
)

func (c MatchConfidence) String() string {
	switch c {
	case MatchExact:
		return "EXACT"
	case MatchSubstring:
		return "SUBSTR"
	case MatchAmbiguous:
		return "AMBIGUOUS"
	default:
		return "SKIP"
	}
}

type MatchResult struct {
	Confidence MatchConfidence
	Scenes     []models.Scene // all matching FSS scenes (possibly from multiple sites)
	Candidates int            // for ambiguous: how many distinct titles matched
}

var (
	nonAlphanumeric  = regexp.MustCompile(`[^a-z0-9]+`)
	camelLowerUpper  = regexp.MustCompile(`([a-z])([A-Z])`)
	camelUpperSeries = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	formatSuffixRe   = regexp.MustCompile(`(?i)\s*\(\s*(?:full\s+hd|4k|hd|mp4|mov|wmv|avi|mkv|1080p|720p|480p|sd)\s*\)\s*$`)
)

func Normalize(s string) string {
	s = stripFormatSuffix(s)
	s = camelLowerUpper.ReplaceAllString(s, "${1} ${2}")
	s = camelUpperSeries.ReplaceAllString(s, "${1} ${2}")
	lower := strings.ToLower(s)
	clean := nonAlphanumeric.ReplaceAllString(lower, " ")
	return strings.TrimSpace(clean)
}

// stripFormatSuffix removes a single trailing format-tag suffix like " (4K)"
// or " (mp4)". One pass only — nested suffixes like "Title (HD) (4K)" are
// not produced by any FSS scraper in practice (Clips4Sale emits one row per
// format, each with a single suffix), so handling that case isn't worth the
// loop. If a nested case ever shows up, the inner suffix bleeds into the
// normalized title as an extra word, which is acceptable for matching.
func stripFormatSuffix(s string) string {
	return formatSuffixRe.ReplaceAllString(s, "")
}

func stripExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return filename
	}
	return filename[:len(filename)-len(ext)]
}

var noiseWords = map[string]bool{
	"step": true,
}

func stripNoise(s string) string {
	words := strings.Fields(s)
	var out []string
	for _, w := range words {
		if !noiseWords[w] {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

var trailingNumberRe = regexp.MustCompile(`\s+\d+$`)

func stripTrailingNumber(s string) string {
	return trailingNumberRe.ReplaceAllString(s, "")
}

func BuildIndex(scenes []models.Scene) *SceneIndex {
	idx := &SceneIndex{
		byTitle:             make(map[string][]models.Scene),
		byTitleSanitized:    make(map[string][]models.Scene),
		byTitleNoTrailingID: make(map[string][]models.Scene),
		all:                 scenes,
	}
	for _, s := range scenes {
		norm := Normalize(s.Title)
		if norm != "" {
			idx.byTitle[norm] = append(idx.byTitle[norm], s)
			sanitized := stripNoise(norm)
			if sanitized != "" {
				idx.byTitleSanitized[sanitized] = append(idx.byTitleSanitized[sanitized], s)
			}
			noID := stripTrailingNumber(norm)
			if noID != "" && noID != norm {
				idx.byTitleNoTrailingID[noID] = append(idx.byTitleNoTrailingID[noID], s)
			}
		}
	}
	idx.titleSubstrIdx = newSubstringIndex(idx.byTitle)
	idx.sanitizedSubstrIdx = newSubstringIndex(idx.byTitleSanitized)
	idx.noTrailingIDSubstrIdx = newSubstringIndex(idx.byTitleNoTrailingID)
	return idx
}

// LoadJSONFiles reads FSS JSON files and returns all scenes.
// Files that don't match the expected format are silently skipped.
func LoadJSONFiles(paths []string) ([]models.Scene, error) {
	var all []models.Scene
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		var sf studioFile
		if err := json.Unmarshal(data, &sf); err != nil {
			continue
		}
		if len(sf.Scenes) == 0 {
			continue
		}
		all = append(all, sf.Scenes...)
	}
	return all, nil
}

// LoadJSONDir reads all *.json files in a directory.
func LoadJSONDir(dir string) ([]models.Scene, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	return LoadJSONFiles(paths)
}

// Match finds FSS scenes matching a filename. fileDurationSec is the
// duration of the Stash file in seconds (0 means unknown / skip check).
func (idx *SceneIndex) Match(filename string, fileDurationSec float64) MatchResult {
	raw := stripExtension(filename)
	norm := Normalize(raw)
	if norm == "" {
		return MatchResult{Confidence: MatchNone}
	}

	// Pass 1: match against original titles.
	if r := matchAgainst(idx.byTitle, idx.titleSubstrIdx, norm, fileDurationSec, 0.5); r.Confidence != MatchNone {
		return r
	}

	// Pass 2: try sanitized index (noise words like "step" stripped from titles).
	// The filename may or may not contain noise words — either way the
	// sanitized title index might match.
	sanitized := stripNoise(norm)
	if sanitized != "" {
		if r := matchAgainst(idx.byTitleSanitized, idx.sanitizedSubstrIdx, sanitized, fileDurationSec, 0.5); r.Confidence != MatchNone {
			return r
		}
	}

	// Pass 3: strip trailing numbers from both sides. Handles sites where the
	// FSS title uses a per-performer sequence number (e.g. "Artemisia Love 1")
	// but the filename uses a site-wide episode number (e.g. "044"). The
	// performer name is the real match key; duration disambiguates. Only
	// attempted when file duration is known — without it, single-word names
	// like "Marsha" would match too broadly.
	noID := stripTrailingNumber(norm)
	if noID != "" && noID != norm && fileDurationSec > 0 {
		if r := matchAgainst(idx.byTitleNoTrailingID, idx.noTrailingIDSubstrIdx, noID, fileDurationSec, 0); r.Confidence != MatchNone {
			return r
		}
	}

	return MatchResult{Confidence: MatchNone}
}

func matchAgainst(titleMap map[string][]models.Scene, si *substringIndex, norm string, fileDurationSec float64, minWordRatio float64) MatchResult {
	if scenes, ok := titleMap[norm]; ok {
		filtered := filterByDuration(scenes, fileDurationSec)
		if len(filtered) > 0 {
			return MatchResult{Confidence: MatchExact, Scenes: filtered}
		}
		if fileDurationSec > 0 {
			return MatchResult{Confidence: MatchNone}
		}
		return MatchResult{Confidence: MatchExact, Scenes: scenes}
	}

	type candidate struct {
		title  string
		scenes []models.Scene
	}
	var candidates []candidate
	for _, title := range si.findCandidateTitles(strings.Fields(norm), minWordRatio) {
		scenes := titleMap[title]
		filtered := filterByDuration(scenes, fileDurationSec)
		if len(filtered) > 0 {
			candidates = append(candidates, candidate{title: title, scenes: filtered})
		} else if fileDurationSec == 0 {
			candidates = append(candidates, candidate{title: title, scenes: scenes})
		}
	}

	if len(candidates) == 0 {
		return MatchResult{Confidence: MatchNone}
	}

	if len(candidates) == 1 {
		return MatchResult{Confidence: MatchSubstring, Scenes: candidates[0].scenes}
	}

	distinctTitles := map[string]bool{}
	for _, c := range candidates {
		distinctTitles[c.title] = true
	}

	if len(distinctTitles) == 1 {
		var allScenes []models.Scene
		for _, c := range candidates {
			allScenes = append(allScenes, c.scenes...)
		}
		return MatchResult{Confidence: MatchSubstring, Scenes: allScenes}
	}

	// Pick the longest matching title as the most-specific match (more
	// characters = more discriminating). Length is in Unicode codepoints,
	// not bytes — `len(s)` would over-count multi-byte titles like "Café"
	// (5 bytes / 4 chars) and bias the tie-break against ASCII titles.
	// Two titles of equal codepoint length but different content stays
	// ambiguous (we can't pick one over the other without more signal).
	best := candidates[0]
	tied := false
	for _, c := range candidates[1:] {
		codepoints := countCodepoints(c.title)
		bestCodepoints := countCodepoints(best.title)
		if codepoints > bestCodepoints {
			best = c
			tied = false
		} else if codepoints == bestCodepoints && c.title != best.title {
			tied = true
		}
	}

	if tied {
		return MatchResult{
			Confidence: MatchAmbiguous,
			Candidates: len(distinctTitles),
		}
	}
	return MatchResult{Confidence: MatchSubstring, Scenes: best.scenes}
}

const durationTolerancePct = 0.10
const durationToleranceMin = 30.0

func durationClose(fssDuration int, fileDuration float64) bool {
	if fssDuration == 0 || fileDuration == 0 {
		return true
	}
	diff := fileDuration - float64(fssDuration)
	if diff < 0 {
		diff = -diff
	}
	tolerance := fileDuration * durationTolerancePct
	if tolerance < durationToleranceMin {
		tolerance = durationToleranceMin
	}
	return diff <= tolerance
}

func filterByDuration(scenes []models.Scene, fileDuration float64) []models.Scene {
	if fileDuration == 0 {
		return scenes
	}
	var out []models.Scene
	for _, s := range scenes {
		if durationClose(s.Duration, fileDuration) {
			out = append(out, s)
		}
	}
	return out
}

// countCodepoints returns the number of Unicode codepoints (runes) in s.
// `range string` walks runes, so this avoids the allocation of `len([]rune(s))`
// while still being correct for non-ASCII input where `len(s)` (byte count)
// would be wrong.
func countCodepoints(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
