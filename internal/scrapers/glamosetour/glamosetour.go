// Package glamosetour scrapes Glamose network tour pages that use the
// "refstat.php" template. These are single-page listings (~20-24 recent
// entries) with no historical pagination. Each site shares the same HTML
// structure but serves a different content pool filtered by sid.
package glamosetour

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

type SiteConfig struct {
	SiteID     string
	Domain     string
	StudioName string
}

type Scraper struct {
	cfg    SiteConfig
	client *http.Client
	re     *regexp.Regexp
}

func New(cfg SiteConfig) *Scraper {
	escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
	return &Scraper{
		cfg:    cfg,
		client: httpx.NewClient(30 * time.Second),
		re:     regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s\b`, escaped)),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string               { return s.cfg.SiteID }
func (s *Scraper) Patterns() []string       { return []string{s.cfg.Domain + "/"} }
func (s *Scraper) MatchesURL(u string) bool { return s.re.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	cardRe      = regexp.MustCompile(`(?s)<div class="itemmain[^"]*">(.*?)<div class="clearfix">`)
	lidRe       = regexp.MustCompile(`refstat\.php\?lid=(\d+)`)
	performerRe = regexp.MustCompile(`window\.status='([^']+)'`)
	dateRe      = regexp.MustCompile(`Added:\s*([A-Z][a-z]+ \d{1,2}, \d{4})`)
	durationRe  = regexp.MustCompile(`(\d+)\.(\d+)mins`)
	thumbRe     = regexp.MustCompile(`<img\s+src="([^"]+)"`)
	tagRe       = regexp.MustCompile(`search\('([^']+)'`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	listURL := fmt.Sprintf("https://www.%s/?videos", s.cfg.Domain)
	scraper.Debugf(1, "%s: fetching tour listing %s", s.cfg.SiteID, listURL)

	body, err := s.fetchHTML(ctx, listURL)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("listing: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	scenes := s.parseListingPage(body, studioURL)
	scraper.Debugf(1, "%s: %d scenes found", s.cfg.SiteID, len(scenes))

	if len(scenes) > 0 {
		select {
		case out <- scraper.Progress(len(scenes)):
		case <-ctx.Done():
			return
		}
	}

	for _, scene := range scenes {
		if opts.KnownIDs[scene.ID] {
			scraper.Debugf(1, "%s: hit known ID %s, stopping early", s.cfg.SiteID, scene.ID)
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- scraper.Scene(scene):
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) parseListingPage(body []byte, studioURL string) []models.Scene {
	cards := cardRe.FindAllSubmatch(body, -1)
	now := time.Now().UTC()
	var scenes []models.Scene

	for _, card := range cards {
		content := card[1]

		m := lidRe.FindSubmatch(content)
		if m == nil {
			continue
		}
		lid := string(m[1])

		scene := models.Scene{
			ID:        lid,
			SiteID:    s.cfg.SiteID,
			StudioURL: studioURL,
			Studio:    s.cfg.StudioName,
			URL:       fmt.Sprintf("https://www.%s/refstat.php?lid=%s", s.cfg.Domain, lid),
			ScrapedAt: now,
		}

		if m := performerRe.FindSubmatch(content); m != nil {
			name := html.UnescapeString(string(m[1]))
			scene.Title = name
			scene.Performers = []string{name}
		}

		if m := dateRe.FindSubmatch(content); m != nil {
			if t, err := time.Parse("January 2, 2006", string(m[1])); err == nil {
				scene.Date = t.UTC()
			}
		}

		if m := durationRe.FindSubmatch(content); m != nil {
			mins, _ := strconv.Atoi(string(m[1]))
			secs, _ := strconv.Atoi(string(m[2]))
			scene.Duration = mins*60 + secs
		}

		if m := thumbRe.FindSubmatch(content); m != nil {
			thumb := string(m[1])
			if strings.Contains(thumb, "image_resize.php") {
				if u, err := url.Parse(thumb); err == nil {
					if img := u.Query().Get("i"); img != "" {
						thumb = fmt.Sprintf("https://www.%s/%s", s.cfg.Domain, img)
					}
				}
			}
			if !strings.HasPrefix(thumb, "http") {
				thumb = fmt.Sprintf("https://www.%s/%s", s.cfg.Domain, strings.TrimPrefix(thumb, "/"))
			}
			scene.Thumbnail = thumb
		}

		var tags []string
		for _, m := range tagRe.FindAllSubmatch(content, -1) {
			tag := strings.ReplaceAll(string(m[1]), "__", " ")
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
		scene.Tags = tags

		scenes = append(scenes, scene)
	}
	return scenes
}

func (s *Scraper) fetchHTML(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
