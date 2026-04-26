package apovstory

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const defaultSiteBase = "https://apovstory.com"

type Scraper struct {
	client   *http.Client
	siteBase string
}

func New() *Scraper {
	return &Scraper{
		client:   httpx.NewClient(30 * time.Second),
		siteBase: defaultSiteBase,
	}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "apovstory" }

func (s *Scraper) Patterns() []string {
	return []string{"apovstory.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?apovstory\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type listEntry struct {
	id         string
	title      string
	url        string
	thumbnail  string
	preview    string
	performers []string
	duration   int
	date       time.Time
	rating     float64
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	work := make(chan listEntry, opts.Workers)
	var wg sync.WaitGroup

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range work {
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}
				scene, err := s.fetchDetail(ctx, studioURL, entry)
				select {
				case out <- scraper.SceneResult{Scene: scene, Err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for page := 1; ; page++ {
		if ctx.Err() != nil {
			break
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				break
			}
		}

		entries, err := s.fetchPage(ctx, page)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			break
		}

		if len(entries) == 0 {
			break
		}

		if page == 1 {
			select {
			case out <- scraper.SceneResult{Total: estimateTotal(len(entries))}:
			case <-ctx.Done():
				break
			}
		}

		cancelled := false
		hitKnown := false
		for _, e := range entries {
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[e.id] {
				hitKnown = true
				break
			}
			select {
			case work <- e:
			case <-ctx.Done():
				cancelled = true
			}
			if cancelled {
				break
			}
		}
		if cancelled || hitKnown {
			if hitKnown {
				select {
				case out <- scraper.SceneResult{StoppedEarly: true}:
				case <-ctx.Done():
				}
			}
			break
		}
	}

	close(work)
	wg.Wait()
}

func estimateTotal(firstPageCount int) int {
	return firstPageCount * 19
}

var (
	latestSectionRe = regexp.MustCompile(`(?s)Latest Updates(.*?)Most Popular Updates`)
	videoBlockRe    = regexp.MustCompile(`(?s)<div class="videoBlock" data-setid="(\d+)">(.*?)</div><!--//updateDetails-->\s*</div>`)
	sceneLinkRe     = regexp.MustCompile(`href="(https?://[^"]*?/trailers/[^"]+\.html)"`)
	sceneTitleRe    = regexp.MustCompile(`title="([^"]+)"`)
	thumbURLRe      = regexp.MustCompile(`src="(/content/[^"]+)"`)
	previewRe       = regexp.MustCompile(`src='(/videothumbs/[^']+)'`)
	modelsBlockRe   = regexp.MustCompile(`(?s)updateDetails_models">(.*?)</div>`)
	performerRe     = regexp.MustCompile(`>([^<]+)</a>`)
	ratingDurRe     = regexp.MustCompile(`star-o"></i>\s*([\d.]+)\s*\|\s*([\d:]+)`)
	dateRe          = regexp.MustCompile(`(?s)updateDetails_date">\s*([^<]+)`)
)

func (s *Scraper) fetchPage(ctx context.Context, page int) ([]listEntry, error) {
	pageURL := fmt.Sprintf("%s/updates/page_%d.html", s.siteBase, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: pageURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	section := body
	if m := latestSectionRe.FindSubmatch(body); m != nil {
		section = m[1]
	}

	blocks := videoBlockRe.FindAllSubmatch(section, -1)
	entries := make([]listEntry, 0, len(blocks))

	for _, block := range blocks {
		setID := string(block[1])
		content := block[2]

		entry := listEntry{id: setID}

		if m := sceneLinkRe.FindSubmatch(content); m != nil {
			entry.url = string(m[1])
		}
		if m := sceneTitleRe.FindSubmatch(content); m != nil {
			entry.title = html.UnescapeString(string(m[1]))
		}
		if m := thumbURLRe.FindSubmatch(content); m != nil {
			entry.thumbnail = s.siteBase + string(m[1])
		}
		if m := previewRe.FindSubmatch(content); m != nil {
			entry.preview = s.siteBase + string(m[1])
		}
		if m := modelsBlockRe.FindSubmatch(content); m != nil {
			for _, pm := range performerRe.FindAllSubmatch(m[1], -1) {
				name := strings.TrimSpace(string(pm[1]))
				if name != "" {
					entry.performers = append(entry.performers, name)
				}
			}
		}
		if m := ratingDurRe.FindSubmatch(content); m != nil {
			entry.rating, _ = strconv.ParseFloat(string(m[1]), 64)
			entry.duration = parseDuration(string(m[2]))
		}
		if m := dateRe.FindSubmatch(content); m != nil {
			entry.date = parseDate(strings.TrimSpace(string(m[1])))
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

var (
	ogDescRe       = regexp.MustCompile(`og:description"\s+content="([^"]+)"`)
	categoryLinkRe = regexp.MustCompile(`categories/\w+_1_d\.html">([^<]+)</a>`)
)

func (s *Scraper) fetchDetail(ctx context.Context, studioURL string, entry listEntry) (models.Scene, error) {
	now := time.Now().UTC()

	scene := models.Scene{
		ID:         entry.id,
		SiteID:     "apovstory",
		StudioURL:  studioURL,
		Title:      entry.title,
		URL:        entry.url,
		Thumbnail:  entry.thumbnail,
		Preview:    entry.preview,
		Performers: entry.performers,
		Duration:   entry.duration,
		Date:       entry.date,
		Studio:     "A POV Story",
		ScrapedAt:  now,
	}

	if entry.url == "" {
		return scene, nil
	}

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: entry.url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return scene, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return scene, nil
	}

	if m := ogDescRe.FindSubmatch(body); m != nil {
		scene.Description = html.UnescapeString(string(m[1]))
	}

	for _, cm := range categoryLinkRe.FindAllSubmatch(body, -1) {
		scene.Tags = append(scene.Tags, strings.TrimSpace(string(cm[1])))
	}

	return scene, nil
}

func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		total = total*60 + n
	}
	return total
}

func parseDate(s string) time.Time {
	t, err := time.Parse("January 2, 2006", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
