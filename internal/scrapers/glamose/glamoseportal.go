package glamose

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

const portalBase = "https://www.glamose.com"

type portalScraper struct {
	client *http.Client
}

var _ scraper.StudioScraper = (*portalScraper)(nil)

var portalMatchRe = regexp.MustCompile(`^https?://(?:www\.)?glamose\.com\b`)

func (s *portalScraper) ID() string               { return "glamose" }
func (s *portalScraper) Patterns() []string       { return []string{"glamose.com/"} }
func (s *portalScraper) MatchesURL(u string) bool { return portalMatchRe.MatchString(u) }

func (s *portalScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.runPortal(ctx, studioURL, opts, out)
	return out, nil
}

var (
	boxRe      = regexp.MustCompile(`(?s)<div class="box">(.*?)</div>\s*</div>`)
	updateIDRe = regexp.MustCompile(`update_id=(\d+)`)
	boxModelRe = regexp.MustCompile(`/model/[^"]*">([^<]+)</a>`)
	boxSiteRe  = regexp.MustCompile(`<span class="site">([^<]+)</span>`)
	boxDateRe  = regexp.MustCompile(`<span class="date">([^<]+)</span>`)
	boxImgRe   = regexp.MustCompile(`(?:data-src|src)="(https?://cdn\.glamose\.com/[^"]+)"`)
	boxVideoRe = regexp.MustCompile(`play-icon`)
)

func (s *portalScraper) runPortal(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Paginate(ctx, opts, "glamose", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		start := (page - 1) * 50
		u := fmt.Sprintf("%s/?start=%d", portalBase, start)

		body, err := s.fetchPortalHTML(ctx, u)
		if err != nil {
			return scraper.PageResult{}, err
		}

		scenes := parsePortalPage(body, studioURL)
		return scraper.PageResult{
			Scenes: scenes,
			Done:   len(scenes) < 50,
		}, nil
	})
}

func parsePortalPage(body []byte, studioURL string) []models.Scene {
	boxes := boxRe.FindAllSubmatch(body, -1)
	now := time.Now().UTC()
	var scenes []models.Scene

	for _, box := range boxes {
		content := box[1]

		m := updateIDRe.FindSubmatch(content)
		if m == nil {
			continue
		}
		uid := string(m[1])

		scene := models.Scene{
			ID:        uid,
			SiteID:    "glamose",
			StudioURL: studioURL,
			URL:       portalBase + "/?update_id=" + uid,
			Studio:    "Glamose",
			ScrapedAt: now,
		}

		if m := boxModelRe.FindSubmatch(content); m != nil {
			name := strings.TrimSpace(html.UnescapeString(string(m[1])))
			scene.Title = name
			scene.Performers = []string{name}
		}

		if m := boxSiteRe.FindSubmatch(content); m != nil {
			scene.Series = strings.TrimSpace(string(m[1]))
		}

		if m := boxDateRe.FindSubmatch(content); m != nil {
			dateStr := strings.TrimSpace(string(m[1]))
			cleaned := parseutil.StripOrdinalSuffix(dateStr)
			for _, layout := range []string{"2 Jan 2006", "2 January 2006"} {
				if t, err := time.Parse(layout, cleaned); err == nil {
					scene.Date = t.UTC()
					break
				}
			}
		}

		if m := boxImgRe.FindSubmatch(content); m != nil {
			scene.Thumbnail = string(m[1])
		}

		if boxVideoRe.Match(content) {
			scene.Tags = []string{"Video"}
		}

		scenes = append(scenes, scene)
	}
	return scenes
}

func (s *portalScraper) fetchPortalHTML(ctx context.Context, rawURL string) ([]byte, error) {
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
