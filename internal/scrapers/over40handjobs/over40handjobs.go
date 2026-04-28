package over40handjobs

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

const (
	defaultBase = "https://www.over40handjobs.com"
	siteID      = "over40handjobs"
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?over40handjobs\.com`)

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   defaultBase,
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }
func (s *Scraper) Patterns() []string {
	return []string{
		"over40handjobs.com",
		"over40handjobs.com/updates.htm",
		"over40handjobs.com/models/*.html",
	}
}
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type listingEntry struct {
	slug     string
	title    string
	date     time.Time
	duration int
	desc     string
	thumb    string
}

var (
	titleLinkRe = regexp.MustCompile(`<h3><a href="([^"]*)"[^>]*>(.*?)</a></h3>`)
	h4Re        = regexp.MustCompile(`<h4>Date:\s*(.*?)<br\s*/?>\s*(\d+:\d+)`)
	descRe      = regexp.MustCompile(`(?s)<p>(.+?)</p>`)
	thumbRe     = regexp.MustCompile(`<img src="([^"]*)"[^>]*alt="`)
	performerRe = regexp.MustCompile(`<a href="/models/[^"]*">([^<]+)</a>`)
	slugRe      = regexp.MustCompile(`/videos/([^.?]+)\.html`)
)

func stripNATS(u string) string {
	if i := strings.Index(u, "?nats="); i >= 0 {
		return u[:i]
	}
	return u
}

func parseDuration(raw string) int {
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return 0
	}
	mins, _ := strconv.Atoi(parts[0])
	secs, _ := strconv.Atoi(parts[1])
	return mins*60 + secs
}

func parseDate(raw string) time.Time {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), ",")
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse("January 2, 2006", raw); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func parseListingPage(body []byte) []listingEntry {
	blocks := strings.Split(string(body), `<div class="updateBlock`)
	var entries []listingEntry
	for _, block := range blocks[1:] {
		var e listingEntry

		if m := titleLinkRe.FindStringSubmatch(block); m != nil {
			href := stripNATS(m[1])
			if sm := slugRe.FindStringSubmatch(href); sm != nil {
				e.slug = sm[1]
			}
			e.title = strings.TrimSpace(html.UnescapeString(m[2]))
		}

		if m := h4Re.FindStringSubmatch(block); m != nil {
			e.date = parseDate(m[1])
			e.duration = parseDuration(m[2])
		}

		if m := descRe.FindStringSubmatch(block); m != nil {
			e.desc = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			e.thumb = m[1]
		}

		if e.slug == "" || e.title == "" {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

func parseDetailPage(body []byte) []string {
	page := string(body)
	i := strings.Index(page, "featuringWrapper")
	if i < 0 {
		return nil
	}
	end := strings.Index(page[i:], "</div>")
	if end < 0 {
		return nil
	}
	block := page[i : i+end]

	matches := performerRe.FindAllStringSubmatch(block, -1)
	performers := make([]string, 0, len(matches))
	for _, m := range matches {
		name := strings.TrimSpace(html.UnescapeString(m[1]))
		if name != "" {
			performers = append(performers, name)
		}
	}
	return performers
}

func pageURL(base string, page int) string {
	if page <= 1 {
		return base + "/updates.htm"
	}
	return fmt.Sprintf("%s/updates_%d.html", base, page)
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	work := make(chan listingEntry)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range work {
				scene, err := s.buildScene(ctx, e, studioURL, opts.Delay)
				if err != nil {
					select {
					case out <- scraper.Error(err):
					case <-ctx.Done():
						return
					}
					continue
				}
				select {
				case out <- scraper.Scene(scene):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	if strings.Contains(studioURL, "/models/") {
		s.feedSingle(ctx, stripNATS(studioURL), opts, work, out)
	} else {
		s.feedPaginated(ctx, opts, work, out)
	}
	close(work)
	wg.Wait()
}

func (s *Scraper) feedPaginated(ctx context.Context, opts scraper.ListOpts, work chan<- listingEntry, out chan<- scraper.SceneResult) {
	progressSent := false
	for page := 1; ; page++ {
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		body, err := s.fetchPage(ctx, pageURL(s.base, page))
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		entries := parseListingPage(body)
		if len(entries) == 0 {
			return
		}

		if !progressSent {
			select {
			case out <- scraper.Progress(len(entries) * 40):
			case <-ctx.Done():
				return
			}
			progressSent = true
		}

		for _, e := range entries {
			if opts.KnownIDs[e.slug] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case work <- e:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *Scraper) feedSingle(ctx context.Context, pageURL string, opts scraper.ListOpts, work chan<- listingEntry, out chan<- scraper.SceneResult) {
	body, err := s.fetchPage(ctx, pageURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("model page: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	entries := parseListingPage(body)
	if len(entries) == 0 {
		select {
		case out <- scraper.Error(fmt.Errorf("no scenes found on model page")):
		case <-ctx.Done():
		}
		return
	}

	select {
	case out <- scraper.Progress(len(entries)):
	case <-ctx.Done():
		return
	}

	for _, e := range entries {
		if opts.KnownIDs[e.slug] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case work <- e:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) buildScene(ctx context.Context, e listingEntry, studioURL string, delay time.Duration) (models.Scene, error) {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return models.Scene{}, ctx.Err()
		}
	}

	sceneURL := fmt.Sprintf("%s/videos/%s.html", s.base, e.slug)
	body, err := s.fetchPage(ctx, sceneURL)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", e.slug, err)
	}

	performers := parseDetailPage(body)

	thumb := e.thumb
	if thumb != "" && !strings.HasPrefix(thumb, "http") {
		thumb = s.base + thumb
	}

	now := time.Now().UTC()
	return models.Scene{
		ID:          e.slug,
		SiteID:      siteID,
		StudioURL:   studioURL,
		URL:         sceneURL,
		Title:       e.title,
		Date:        e.date,
		Duration:    e.duration,
		Description: e.desc,
		Thumbnail:   thumb,
		Performers:  performers,
		Studio:      "Over 40 Handjobs",
		ScrapedAt:   now,
	}, nil
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}
