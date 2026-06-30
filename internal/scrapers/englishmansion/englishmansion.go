// Package englishmansion scrapes The English Mansion (theenglishmansion.com).
// The recent-movies listing is delivered by an AJAX endpoint
// (updates_ajax.html?type=recent-movies&offset=N) that returns a chunk of fully
// rendered update blocks. The offset is item-based and steps by the page size;
// past the end the endpoint clamps and repeats the final items, so the loop
// terminates by de-duplicating scene ids. The per-day publish date is rendered
// in separate day headers that are awkward to attach per scene, so Date is left
// zero.
package englishmansion

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

var siteBase = "https://www.theenglishmansion.com"

const (
	siteID   = "englishmansion"
	pageSize = 8
)

type Scraper struct{ Client *http.Client }

func New() *Scraper { return &Scraper{Client: httpx.NewClient(30 * time.Second)} }

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }
func (s *Scraper) Patterns() []string {
	return []string{"theenglishmansion.com", "theenglishmansion.com/updates.html"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?theenglishmansion\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

var (
	blockRe    = regexp.MustCompile(`(?s)class="update-block-recent">(.*?)</table>`)
	titleRe    = regexp.MustCompile(`(?s)class="block-title-2"[^>]*>(.*?)</td>`)
	lengthRe   = regexp.MustCompile(`class="block-title-1"[^>]*>\s*Length:\s*([0-9:]+)`)
	featRe     = regexp.MustCompile(`(?s)class="featuring">\s*(?:Featuring\s*)?(.*?)</p>`)
	synRe      = regexp.MustCompile(`(?s)class="synopsis"[^>]*>(.*?)</p>`)
	posterRe   = regexp.MustCompile(`class="cover"><img src=['"]([^'"]+)['"]`)
	slugRe     = regexp.MustCompile(`/still/\d+/[^/]+/([^/]+)/`)
	splitRe    = regexp.MustCompile(`(?i)\s*(?:&amp;|&|,| and )\s*`)
	tagStripRe = regexp.MustCompile(`<[^>]+>`)
)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	seen := make(map[string]bool)
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		offset := (page - 1) * pageSize
		pageURL := fmt.Sprintf("%s/updates_ajax.html?type=recent-movies&offset=%d", siteBase, offset)
		body, err := s.get(ctx, pageURL)
		if err != nil {
			return scraper.PageResult{}, err
		}
		all := parseListing(string(body), studioURL, now)
		// The endpoint clamps/repeats at the end — stop when nothing is new.
		fresh := all[:0]
		for _, sc := range all {
			if !seen[sc.ID] {
				seen[sc.ID] = true
				fresh = append(fresh, sc)
			}
		}
		return scraper.PageResult{Scenes: fresh}, nil
	})
}

func parseListing(doc, studioURL string, now time.Time) []models.Scene {
	var scenes []models.Scene
	for _, m := range blockRe.FindAllStringSubmatch(doc, -1) {
		if sc, ok := toScene(m[1], studioURL, now); ok {
			scenes = append(scenes, sc)
		}
	}
	return scenes
}

func toScene(block, studioURL string, now time.Time) (models.Scene, bool) {
	tm := titleRe.FindStringSubmatch(block)
	if tm == nil {
		return models.Scene{}, false
	}
	title := cleanText(tm[1])
	if title == "" {
		return models.Scene{}, false
	}
	sc := models.Scene{
		SiteID:    siteID,
		StudioURL: studioURL,
		Title:     title,
		URL:       siteBase + "/updates.html",
		Studio:    "The English Mansion",
		ScrapedAt: now,
	}
	if p := posterRe.FindStringSubmatch(block); p != nil {
		sc.Thumbnail = absURL(p[1])
		if sm := slugRe.FindStringSubmatch(p[1]); sm != nil {
			sc.ID = sm[1]
		}
	}
	if sc.ID == "" {
		sc.ID = slugify(title)
	}
	if l := lengthRe.FindStringSubmatch(block); l != nil {
		sc.Duration = parseutil.ParseDurationColon(l[1])
	}
	if f := featRe.FindStringSubmatch(block); f != nil {
		for _, name := range splitRe.Split(cleanText(f[1]), -1) {
			name = strings.TrimSpace(name)
			if name != "" {
				sc.Performers = append(sc.Performers, name)
			}
		}
	}
	if syn := synRe.FindStringSubmatch(block); syn != nil {
		sc.Description = cleanText(syn[1])
	}
	return sc, true
}

func absURL(p string) string {
	if strings.HasPrefix(p, "http") {
		return p
	}
	return siteBase + "/" + strings.TrimPrefix(p, "/")
}

var slugStripRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugStripRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func (s *Scraper) get(ctx context.Context, u string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{URL: u, Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox)})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

func cleanText(s string) string {
	s = tagStripRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
