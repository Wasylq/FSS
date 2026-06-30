// Package insexarchives scrapes the Insex (1997-2005) Archives
// (insexarchives.com), a legacy PHP site. The updates_new.php?start={N}
// listing pages each show 10 shoots, stepping the start offset by 10. Every
// shoot carries its name, a blurb, image/clip counts, a runtime (MM:SS) and a
// cover image on the listing itself; the media.php?file=... link is the scene
// URL. There is no reliable per-shoot publish date. All data is on the
// listing, so no detail fetch is needed.
package insexarchives

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/parseutil"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID     = "insexarchives"
	studioName = "Insex"
	perPage    = 10
)

var siteBase = "http://www.insexarchives.com"

type Scraper struct {
	Client *http.Client
}

func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"insexarchives.com",
		"insexarchives.com/updates_new.php",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?insexarchives\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	nameRe      = regexp.MustCompile(`articleTextRed">([^<]*)</div>`)
	fileRe      = regexp.MustCompile(`media\.php\?file=([^"&]+?)/index\.php`)
	descRe      = regexp.MustCompile(`(?s)articleText">(.*?)</div>`)
	durationRe  = regexp.MustCompile(`([0-9]+:[0-9]+)\s*(?:&nbsp;|\s)*Minutes`)
	imagesRe    = regexp.MustCompile(`(\d+)\s*Images`)
	clipsRe     = regexp.MustCompile(`(\d+)\s*Clips`)
	coverRe     = regexp.MustCompile(`(images/updates/[^"']+?/promo[^"']*)`)
	continuedRe = regexp.MustCompile(`(?i)\s*\(continued\)\s*$`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	seen := make(map[string]bool)
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		start := (page - 1) * perPage
		pageURL := fmt.Sprintf("%s/updates_new.php?start=%d", siteBase, start)
		body, err := s.get(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := parseListing(body, studioURL, now)
		fresh := scenes[:0]
		for _, sc := range scenes {
			if !seen[sc.ID] {
				seen[sc.ID] = true
				fresh = append(fresh, sc)
			}
		}
		return scraper.PageResult{Scenes: fresh, Done: len(scenes) < perPage}, nil
	})
}

func parseListing(body []byte, studioURL string, now time.Time) []models.Scene {
	page := string(body)
	locs := nameRe.FindAllStringSubmatchIndex(page, -1)
	scenes := make([]models.Scene, 0, len(locs))
	for i, loc := range locs {
		name := strings.TrimSpace(html.UnescapeString(page[loc[2]:loc[3]]))
		start := loc[0]
		end := len(page)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := page[start:end]

		fm := fileRe.FindStringSubmatch(block)
		if fm == nil {
			continue
		}
		file := fm[1] // e.g. VOL0114/26lf5_17

		scene := models.Scene{
			ID:        file,
			SiteID:    siteID,
			StudioURL: studioURL,
			URL:       siteBase + "/media.php?file=" + file + "/index.php",
			Title:     name,
			Studio:    studioName,
			ScrapedAt: now,
		}
		if scene.Title == "" {
			scene.Title = file
		}

		if m := descRe.FindStringSubmatch(block); m != nil {
			scene.Description = continuedRe.ReplaceAllString(cleanText(m[1]), "")
		}

		if m := durationRe.FindStringSubmatch(block); m != nil {
			scene.Duration = parseutil.ParseDurationColon(m[1])
		}

		var tags []string
		if m := imagesRe.FindStringSubmatch(block); m != nil {
			tags = append(tags, m[1]+" Images")
		}
		if m := clipsRe.FindStringSubmatch(block); m != nil {
			tags = append(tags, m[1]+" Clips")
		}
		scene.Tags = tags

		if m := coverRe.FindStringSubmatch(block); m != nil {
			scene.Thumbnail = absURL(m[1])
		}

		scenes = append(scenes, scene)
	}
	return scenes
}

func absURL(p string) string {
	p = strings.TrimSpace(html.UnescapeString(p))
	if p == "" || strings.HasPrefix(p, "http") {
		return p
	}
	return siteBase + "/" + strings.TrimPrefix(p, "/")
}

func (s *Scraper) get(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{URL: u, Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox)})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

var tagStripRe = regexp.MustCompile(`<[^>]+>`)

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
