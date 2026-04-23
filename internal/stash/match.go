package stash

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/Wasylq/FSS/models"
)

type studioFile struct {
	StudioURL  string         `json:"studioUrl"`
	ScrapedAt  time.Time      `json:"scrapedAt"`
	SceneCount int            `json:"sceneCount"`
	Scenes     []models.Scene `json:"scenes"`
}

type SceneIndex struct {
	byTitle         map[string][]models.Scene
	byTitleSanitized map[string][]models.Scene // titles with noise words stripped
	all             []models.Scene
}

type MatchConfidence int

const (
	MatchExact     MatchConfidence = iota
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

func stripFormatSuffix(s string) string {
	for {
		stripped := formatSuffixRe.ReplaceAllString(s, "")
		if stripped == s {
			return s
		}
		s = stripped
	}
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

func BuildIndex(scenes []models.Scene) *SceneIndex {
	idx := &SceneIndex{
		byTitle:         make(map[string][]models.Scene),
		byTitleSanitized: make(map[string][]models.Scene),
		all:             scenes,
	}
	for _, s := range scenes {
		norm := Normalize(s.Title)
		if norm != "" {
			idx.byTitle[norm] = append(idx.byTitle[norm], s)
			sanitized := stripNoise(norm)
			if sanitized != "" {
				idx.byTitleSanitized[sanitized] = append(idx.byTitleSanitized[sanitized], s)
			}
		}
	}
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
	if r := matchAgainst(idx.byTitle, norm, fileDurationSec); r.Confidence != MatchNone {
		return r
	}

	// Pass 2: try sanitized index (noise words like "step" stripped from titles).
	// The filename may or may not contain noise words — either way the
	// sanitized title index might match.
	sanitized := stripNoise(norm)
	if sanitized != "" {
		if r := matchAgainst(idx.byTitleSanitized, sanitized, fileDurationSec); r.Confidence != MatchNone {
			return r
		}
	}

	return MatchResult{Confidence: MatchNone}
}

func matchAgainst(titleMap map[string][]models.Scene, norm string, fileDurationSec float64) MatchResult {
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
	for title, scenes := range titleMap {
		if isSubstring(title, norm) {
			filtered := filterByDuration(scenes, fileDurationSec)
			if len(filtered) > 0 {
				candidates = append(candidates, candidate{title: title, scenes: filtered})
			} else if fileDurationSec == 0 {
				candidates = append(candidates, candidate{title: title, scenes: scenes})
			}
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

func isSubstring(title, filename string) bool {
	titleWords := strings.Fields(title)
	if len(titleWords) == 0 {
		return false
	}
	filenameWords := strings.Fields(filename)
	wordSet := make(map[string]bool, len(filenameWords))
	for _, w := range filenameWords {
		wordSet[w] = true
	}
	for _, w := range titleWords {
		if !wordSet[w] {
			return false
		}
	}
	// Title must cover at least half the filename's words to avoid
	// short titles matching long unrelated filenames.
	return float64(len(titleWords)) >= float64(len(filenameWords))*0.5
}

func countCodepoints(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// StripCommonPrefixes removes common filename prefixes like resolution tags,
// site names etc. that might appear before the actual title.
func StripCommonPrefixes(s string) string {
	s = strings.TrimLeftFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '_'
	})
	return s
}
