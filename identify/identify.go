package identify

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/match"
	"github.com/Wasylq/FSS/nfo"
)

var videoExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".wmv":  true,
	".mov":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".mpg":  true,
	".mpeg": true,
	".ts":   true,
}

type Result struct {
	VideoPath  string
	NFOPath    string
	Confidence match.MatchConfidence
	Scene      *match.MergedScene
	Skipped    bool
	SkipReason string
}

type Options struct {
	Apply    bool
	Force    bool
	NoReport bool
}

func FindVideos(dir string) ([]string, error) {
	var videos []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if videoExtensions[ext] {
			videos = append(videos, path)
		}
		return nil
	})
	return videos, err
}

func Run(videos []string, idx *match.SceneIndex, opts Options) []Result {
	var results []Result

	for _, vpath := range videos {
		basename := filepath.Base(vpath)
		nfoPath := nfoPathFor(vpath)

		if !opts.Force {
			if _, err := os.Stat(nfoPath); err == nil {
				results = append(results, Result{
					VideoPath:  vpath,
					NFOPath:    nfoPath,
					Confidence: match.MatchNone,
					Skipped:    true,
					SkipReason: "nfo exists",
				})
				continue
			}
		}

		mr := idx.Match(basename, probeDuration(vpath))
		if mr.Confidence == match.MatchNone || mr.Confidence == match.MatchAmbiguous {
			results = append(results, Result{
				VideoPath:  vpath,
				NFOPath:    nfoPath,
				Confidence: mr.Confidence,
			})
			continue
		}

		merged := match.MergeScenes(mr.Scenes, time.Time{})
		r := Result{
			VideoPath:  vpath,
			NFOPath:    nfoPath,
			Confidence: mr.Confidence,
			Scene:      &merged,
		}

		if opts.Apply {
			if err := writeNFO(nfoPath, merged); err != nil {
				r.Skipped = true
				r.SkipReason = fmt.Sprintf("write error: %v", err)
			}
		}

		results = append(results, r)
	}

	return results
}

type Stats struct {
	Total     int
	Matched   int
	Unmatched int
	Ambiguous int
	Skipped   int
}

func Summarize(results []Result) Stats {
	var s Stats
	s.Total = len(results)
	for _, r := range results {
		switch {
		case r.Skipped:
			s.Skipped++
		case r.Confidence == match.MatchNone:
			s.Unmatched++
		case r.Confidence == match.MatchAmbiguous:
			s.Ambiguous++
		default:
			s.Matched++
		}
	}
	return s
}

func WriteReport(dir string, results []Result) error {
	var sb strings.Builder
	sb.WriteString("# FSS Identify Report\n")
	fmt.Fprintf(&sb, "# Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))

	var unmatched, skipped []string
	for _, r := range results {
		rel, _ := filepath.Rel(dir, r.VideoPath)
		if rel == "" {
			rel = r.VideoPath
		}
		if r.Skipped {
			skipped = append(skipped, fmt.Sprintf("%s (%s)", rel, r.SkipReason))
		} else if r.Confidence == match.MatchNone || r.Confidence == match.MatchAmbiguous {
			unmatched = append(unmatched, rel)
		}
	}

	if len(unmatched) > 0 {
		fmt.Fprintf(&sb, "## Unmatched (%d)\n", len(unmatched))
		for _, f := range unmatched {
			sb.WriteString(f + "\n")
		}
		sb.WriteString("\n")
	}

	if len(skipped) > 0 {
		fmt.Fprintf(&sb, "## Skipped (%d)\n", len(skipped))
		for _, f := range skipped {
			sb.WriteString(f + "\n")
		}
		sb.WriteString("\n")
	}

	if len(unmatched) == 0 && len(skipped) == 0 {
		return nil
	}

	return os.WriteFile(filepath.Join(dir, "fss-report.txt"), []byte(sb.String()), 0o644)
}

func nfoPathFor(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return videoPath[:len(videoPath)-len(ext)] + ".nfo"
}

func writeNFO(path string, m match.MergedScene) error {
	mov := nfo.FromMergedScene(m)
	data, err := nfo.Marshal(mov)
	if err != nil {
		return fmt.Errorf("marshalling NFO: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// probeDuration returns the file's duration in seconds via ffprobe, or 0 if
// ffprobe is not installed or fails. Best-effort: matching still works without
// duration, it just can't disambiguate same-title scenes.
func probeDuration(path string) float64 {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0
	}
	dur, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return dur
}
