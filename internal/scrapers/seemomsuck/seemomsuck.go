package seemomsuck

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const defaultSiteBase = "https://www.seemomsuck.com"

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

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "seemomsuck" }

func (s *Scraper) Patterns() []string {
	return []string{
		"seemomsuck.com",
		"seemomsuck.com/models/{name}.html",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?seemomsuck\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)
	now := time.Now().UTC()

	if isModelURL(studioURL) {
		s.scrapeModelPage(ctx, studioURL, opts, out, now)
	} else {
		s.scrapeListingPages(ctx, opts, out, now)
	}
}

func isModelURL(u string) bool {
	return strings.Contains(u, "/models/")
}

func (s *Scraper) scrapeListingPages(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
	for page := 1; ; page++ {
		if ctx.Err() != nil {
			return
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		var pageURL string
		if page == 1 {
			pageURL = s.siteBase + "/updates.html?sort=date"
		} else {
			pageURL = fmt.Sprintf("%s/updates_%d.html?sort=date", s.siteBase, page)
		}

		body, err := s.fetchPage(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			return
		}

		scenes := parseListingPage(body, s.siteBase)
		if len(scenes) == 0 {
			return
		}

		if page == 1 {
			maxPage := extractMaxPage(body)
			total := len(scenes) * maxPage
			select {
			case out <- scraper.Progress(total):
			case <-ctx.Done():
				return
			}
		}

		for _, ls := range scenes {
			if opts.KnownIDs[ls.slug] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(ls.toScene(s.siteBase, now)):
			case <-ctx.Done():
				return
			}
		}

		if !hasNextPage(body, page) {
			return
		}
	}
}

func (s *Scraper) scrapeModelPage(ctx context.Context, modelURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult, now time.Time) {
	cleanURL := stripNATS(modelURL)
	body, err := s.fetchPage(ctx, cleanURL)
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	modelName := extractModelName(body)
	scenes := parseListingPage(body, s.siteBase)
	for i := range scenes {
		if len(scenes[i].performers) == 0 && modelName != "" {
			scenes[i].performers = []string{modelName}
		}
	}

	select {
	case out <- scraper.Progress(len(scenes)):
	case <-ctx.Done():
		return
	}

	for _, ls := range scenes {
		if opts.KnownIDs[ls.slug] {
			select {
			case out <- scraper.StoppedEarly():
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- scraper.Scene(ls.toScene(s.siteBase, now)):
		case <-ctx.Done():
			return
		}
	}
}

type listingScene struct {
	slug        string
	title       string
	performers  []string
	description string
	thumb       string
}

func (ls listingScene) toScene(siteBase string, now time.Time) models.Scene {
	return models.Scene{
		ID:          ls.slug,
		SiteID:      "seemomsuck",
		StudioURL:   siteBase,
		Title:       ls.title,
		URL:         siteBase + "/videos/" + ls.slug + ".html",
		Thumbnail:   ls.thumb,
		Description: ls.description,
		Performers:  ls.performers,
		Studio:      "See Mom Suck",
		ScrapedAt:   now,
	}
}

var (
	articleStartRe     = regexp.MustCompile(`<article class="content-list__item`)
	videoLinkRe        = regexp.MustCompile(`href="/videos/([^".]+)\.html`)
	titleRe            = regexp.MustCompile(`class="item-title">([^<]+)<`)
	modelNameDivRe     = regexp.MustCompile(`(?s)class="model-name">(.*?)</div>`)
	performerLinkRe    = regexp.MustCompile(`>([^<]+)</a>`)
	descRe             = regexp.MustCompile(`(?s)class="item-description">(.*?)</p>`)
	thumbRe            = regexp.MustCompile(`src="(/content/[^"]+)"`)
	profileModelNameRe = regexp.MustCompile(`<span class="model-name">([^<]+)</span>`)
	maxPageRe          = regexp.MustCompile(`updates_(\d+)\.html`)
)

func parseListingPage(body []byte, siteBase string) []listingScene {
	page := string(body)
	locs := articleStartRe.FindAllStringIndex(page, -1)
	var scenes []listingScene

	for i, loc := range locs {
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		linkM := videoLinkRe.FindStringSubmatch(block)
		if linkM == nil {
			continue
		}
		slug := linkM[1]

		ls := listingScene{slug: slug}

		if m := titleRe.FindStringSubmatch(block); m != nil {
			ls.title = strings.TrimSpace(html.UnescapeString(m[1]))
		}

		if m := modelNameDivRe.FindStringSubmatch(block); m != nil {
			for _, pm := range performerLinkRe.FindAllStringSubmatch(m[1], -1) {
				name := strings.TrimSpace(html.UnescapeString(pm[1]))
				if name != "" {
					ls.performers = append(ls.performers, name)
				}
			}
		}

		if m := descRe.FindStringSubmatch(block); m != nil {
			ls.description = cleanDescription(m[1])
		}

		if m := thumbRe.FindStringSubmatch(block); m != nil {
			ls.thumb = siteBase + m[1]
		}

		scenes = append(scenes, ls)
	}
	return scenes
}

func cleanDescription(s string) string {
	s = strings.TrimSpace(s)
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	return s
}

func extractModelName(body []byte) string {
	if m := profileModelNameRe.FindSubmatch(body); m != nil {
		return strings.TrimSpace(html.UnescapeString(string(m[1])))
	}
	return ""
}

func extractMaxPage(body []byte) int {
	max := 1
	for _, m := range maxPageRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > max {
			max = n
		}
	}
	return max
}

func hasNextPage(body []byte, current int) bool {
	for _, m := range maxPageRe.FindAllSubmatch(body, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > current {
			return true
		}
	}
	return false
}

func stripNATS(u string) string {
	if idx := strings.Index(u, "?nats="); idx > 0 {
		return u[:idx]
	}
	if idx := strings.Index(u, "&nats="); idx > 0 {
		return u[:idx]
	}
	return u
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}
