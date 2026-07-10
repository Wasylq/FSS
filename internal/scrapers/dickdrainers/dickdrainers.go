// Package dickdrainers scrapes DickDrainers (dickdrainers.com), a NATS tour CMS
// site. The real tour lives under /tour/ (the root is an age-gate splash). Every
// scene's metadata — title, performers, tags, duration, date, description, and
// thumbnail — is published on the listing cards at
// /tour/categories/movies/{page}/latest/, so no per-scene detail fetch is needed
// (the public detail pages are generic join prompts). Model pages
// (/tour/models/{slug}.html) list a single performer's video updates in a
// different card layout, parsed separately.
package dickdrainers

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID     = "dickdrainers"
	studioName = "DickDrainers"
	siteBase   = "https://dickdrainers.com"
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?dickdrainers\.com(?:/|$)`)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"dickdrainers.com",
		"dickdrainers.com/tour/categories/movies/{N}/latest/",
		"dickdrainers.com/tour/models/{slug}.html",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	// Model page: a single page of video-update cards, no pagination.
	if strings.Contains(studioURL, "/tour/models/") {
		scraper.Debugf(1, "dickdrainers: scraping model page %s", studioURL)
		body, err := s.fetchPage(ctx, studioURL)
		if err != nil {
			select {
			case out <- scraper.Error(err):
			case <-ctx.Done():
			}
			return
		}
		scenes := parseModelPage(body, studioURL)
		select {
		case out <- scraper.Progress(len(scenes)):
		case <-ctx.Done():
			return
		}
		for _, sc := range scenes {
			if opts.KnownIDs[sc.ID] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(sc):
			case <-ctx.Done():
				return
			}
		}
		return
	}

	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		pageURL := fmt.Sprintf("%s/tour/categories/movies/%d/latest/", siteBase, page)
		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := parseListing(body, studioURL)
		return scraper.PageResult{
			Scenes: scenes,
			Done:   len(scenes) == 0,
		}, nil
	})
}

var (
	cardRe      = regexp.MustCompile(`(?s)<div class="videoDetails clear">(.*?)<div class="featuring clear">.*?Tags:.*?</ul>\s*</div>`)
	titleRe     = regexp.MustCompile(`(?s)<h3>\s*<a href="([^"]+)"[^>]*>(.*?)</a>`)
	trailerRe   = regexp.MustCompile(`/tour/trailers/([^"/]+)\.html`)
	descRe      = regexp.MustCompile(`(?s)</h3>(.*?)<div class="episode_thumbs">`)
	thumbRe     = regexp.MustCompile(`src0_1x="([^"]+)"`)
	dateRe      = regexp.MustCompile(`Date Added:</span>\s*([A-Z][a-z]+ \d{1,2}, \d{4})`)
	durationRe  = regexp.MustCompile(`(\d+)(?:&nbsp;|\s)+min`)
	featuringRe = regexp.MustCompile(`(?s)<div class="featuring clear">(.*?)</div>`)
	modelRe     = regexp.MustCompile(`(?s)class="update_models">\s*<a href="[^"]+">([^<]+)</a>`)
	tagRe       = regexp.MustCompile(`<a href="[^"]+">([^<]+)</a>`)
	tagStripRe  = regexp.MustCompile(`<[^>]+>`)
)

// parseListing extracts every scene card from a movies listing page.
func parseListing(body []byte, studioURL string) []models.Scene {
	page := string(body)
	matches := cardRe.FindAllStringSubmatch(page, -1)
	scenes := make([]models.Scene, 0, len(matches))
	now := time.Now().UTC()

	for _, m := range matches {
		block := m[0]
		inner := m[1]

		tm := titleRe.FindStringSubmatch(inner)
		if tm == nil {
			continue
		}
		url := absURL(tm[1])
		slug := trailerRe.FindStringSubmatch(url)
		if slug == nil {
			continue
		}

		scene := models.Scene{
			ID:        slug[1],
			SiteID:    siteID,
			StudioURL: studioURL,
			Studio:    studioName,
			Title:     cleanText(tm[2]),
			URL:       url,
			ScrapedAt: now,
		}

		if dm := descRe.FindStringSubmatch(inner); dm != nil {
			scene.Description = cleanText(dm[1])
		}

		if th := thumbRe.FindStringSubmatch(inner); th != nil {
			scene.Thumbnail = absURL(th[1])
		}

		if dt := dateRe.FindStringSubmatch(block); dt != nil {
			if t, err := time.Parse("January 2, 2006", dt[1]); err == nil {
				scene.Date = t.UTC()
			}
		}

		if du := durationRe.FindStringSubmatch(block); du != nil {
			mins, _ := strconv.Atoi(du[1])
			scene.Duration = mins * 60
		}

		// Two div.featuring blocks: the first labelled "Featuring:" (performers),
		// the second "Tags:" (categories). Scope performers to the Featuring block.
		for _, fb := range featuringRe.FindAllStringSubmatch(block, -1) {
			seg := fb[1]
			switch {
			case strings.Contains(seg, "Featuring:"):
				for _, pm := range modelRe.FindAllStringSubmatch(seg, -1) {
					if name := cleanText(pm[1]); name != "" {
						scene.Performers = append(scene.Performers, name)
					}
				}
			case strings.Contains(seg, "Tags:"):
				for _, tg := range tagRe.FindAllStringSubmatch(seg, -1) {
					if tag := cleanText(tg[1]); tag != "" {
						scene.Tags = append(scene.Tags, tag)
					}
				}
			}
		}

		scenes = append(scenes, scene)
	}
	return scenes
}

var (
	modelVideoRe = regexp.MustCompile(`(?s)<div class="item-video.*?<div class="date">([^<]+)</div>`)
	modelTitleRe = regexp.MustCompile(`(?s)<h4>\s*<a href="([^"]+)"[^>]*>\s*(.*?)\s*</a>`)
	modelTimeRe  = regexp.MustCompile(`(?s)<div class="time">(.*?)</div>`)
	aboutRe      = regexp.MustCompile(`<h3>About ([^<]+)</h3>`)
	colonDurRe   = regexp.MustCompile(`\d{1,2}:\d{2}(?::\d{2})?`)
)

// parseModelPage extracts the video-update cards from a model profile page.
// Photo-update cards use a different (item-portrait) layout and are ignored.
func parseModelPage(body []byte, studioURL string) []models.Scene {
	page := string(body)
	now := time.Now().UTC()

	var performer string
	if am := aboutRe.FindStringSubmatch(page); am != nil {
		performer = cleanText(am[1])
	}

	matches := modelVideoRe.FindAllStringSubmatch(page, -1)
	scenes := make([]models.Scene, 0, len(matches))

	for _, m := range matches {
		block := m[0]

		tm := modelTitleRe.FindStringSubmatch(block)
		if tm == nil {
			continue
		}
		url := absURL(tm[1])
		slug := trailerRe.FindStringSubmatch(url)
		if slug == nil {
			continue
		}

		scene := models.Scene{
			ID:        slug[1],
			SiteID:    siteID,
			StudioURL: studioURL,
			Studio:    studioName,
			Title:     cleanText(tm[2]),
			URL:       url,
			ScrapedAt: now,
		}
		if performer != "" {
			scene.Performers = []string{performer}
		}

		if th := thumbRe.FindStringSubmatch(block); th != nil {
			scene.Thumbnail = absURL(th[1])
		}

		if t, err := time.Parse("2006-01-02", strings.TrimSpace(m[1])); err == nil {
			scene.Date = t.UTC()
		}

		if tt := modelTimeRe.FindStringSubmatch(block); tt != nil {
			if cd := colonDurRe.FindString(tt[1]); cd != "" {
				scene.Duration = parseutil.ParseDurationColon(cd)
			}
		}

		scenes = append(scenes, scene)
	}
	return scenes
}

// cleanText strips HTML tags, unescapes entities, and collapses whitespace.
func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

func absURL(u string) string {
	if strings.HasPrefix(u, "/") {
		return siteBase + u
	}
	return u
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     url,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
