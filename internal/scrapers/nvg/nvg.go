// Package nvg scrapes the NVG Network of casting/amateur sites:
// Net Video Girls (netvideogirls.com), Casting Couch-HD (castingcouch-hd.com),
// and Net Girl (netgirl.com).
//
// All three only expose ~42 of their most-recent scenes publicly; the deep
// catalog is members-only. There is no public per-scene page (anonymous links
// point at /join), so the scene URL is set to the site root.
//
// Two different front-ends back the network:
//   - Net Video Girls is a Gatsby tour. The home listing lives at
//     /page-data/home/page-data.json under result.data.allupdates.nodes.
//   - Casting Couch-HD and Net Girl are Next.js tours. The home listing is
//     embedded in the <script id="__NEXT_DATA__"> JSON under
//     props.pageProps.videos.
//
// Neither front-end paginates, so each scraper does a single fetch and emits
// every public scene.
package nvg

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// frontEnd identifies which tour technology a site runs.
type frontEnd int

const (
	frontGatsby frontEnd = iota // Net Video Girls (page-data JSON)
	frontNext                   // Casting Couch-HD / Net Girl (__NEXT_DATA__)
)

// siteConfig holds the per-site values that vary across the three sites.
type siteConfig struct {
	id        string         // stable scraper ID, e.g. "netvideogirls"
	studio    string         // human studio name, e.g. "Net Video Girls"
	base      string         // site root, e.g. "https://netvideogirls.com"
	thumbBase string         // CDN prefix for Next sites; "" for Gatsby
	front     frontEnd       // which front-end this site runs
	match     *regexp.Regexp // URL matcher
	patterns  []string       // patterns for `fss list-scrapers`
}

var sites = []siteConfig{
	{
		id:       "netvideogirls",
		studio:   "Net Video Girls",
		base:     "https://netvideogirls.com",
		front:    frontGatsby,
		match:    regexp.MustCompile(`^https?://(?:www\.)?netvideogirls\.com`),
		patterns: []string{"netvideogirls.com"},
	},
	{
		id:        "castingcouchhd",
		studio:    "Casting Couch-HD",
		base:      "https://castingcouch-hd.com",
		thumbBase: "https://dist.castingcouch-hd.com/web-images/",
		front:     frontNext,
		match:     regexp.MustCompile(`^https?://(?:www\.)?castingcouch-hd\.com`),
		patterns:  []string{"castingcouch-hd.com"},
	},
	{
		id:        "netgirl",
		studio:    "Net Girl",
		base:      "https://netgirl.com",
		thumbBase: "https://cdn3.netgirl.com/images/web/",
		front:     frontNext,
		match:     regexp.MustCompile(`^https?://(?:www\.)?netgirl\.com`),
		patterns:  []string{"netgirl.com"},
	},
}

// Scraper is one StudioScraper instance per NVG-network site.
type Scraper struct {
	cfg    siteConfig
	client *http.Client
}

// New constructs a scraper for the given site config.
func New(cfg siteConfig) *Scraper {
	return &Scraper{cfg: cfg, client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}

func (s *Scraper) ID() string         { return s.cfg.id }
func (s *Scraper) Patterns() []string { return s.cfg.patterns }

func (s *Scraper) MatchesURL(u string) bool { return s.cfg.match.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, _ scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()

	var scenes []models.Scene
	var err error
	switch s.cfg.front {
	case frontGatsby:
		scraper.Debugf(1, "%s: fetching gatsby page-data", s.cfg.id)
		scenes, err = s.fetchGatsby(ctx, studioURL, now)
	case frontNext:
		scraper.Debugf(1, "%s: fetching next-data home", s.cfg.id)
		scenes, err = s.fetchNext(ctx, studioURL, now)
	}
	if err != nil {
		select {
		case out <- scraper.Error(err):
		case <-ctx.Done():
		}
		return
	}

	scraper.Debugf(1, "%s: %d public scenes", s.cfg.id, len(scenes))
	select {
	case out <- scraper.Progress(len(scenes)):
	case <-ctx.Done():
		return
	}

	for _, sc := range scenes {
		select {
		case out <- scraper.Scene(sc):
		case <-ctx.Done():
			return
		}
	}
}

// ---- Gatsby front-end (Net Video Girls) ----

type gatsbyPage struct {
	Result struct {
		Data struct {
			AllUpdates struct {
				Nodes []gatsbyNode `json:"nodes"`
			} `json:"allupdates"`
		} `json:"data"`
	} `json:"result"`
}

type gatsbyNode struct {
	ShortTitle  string `json:"short_title"`
	ReleaseDate string `json:"release_date"`
	MysqlID     int    `json:"mysqlId"`
	TourStats   []struct {
		TourThumb struct {
			ThumbName  string `json:"thumb_name"`
			LocalImage struct {
				ChildImageSharp struct {
					GatsbyImageData struct {
						Images struct {
							Fallback struct {
								Src string `json:"src"`
							} `json:"fallback"`
						} `json:"images"`
					} `json:"gatsbyImageData"`
				} `json:"childImageSharp"`
			} `json:"localImage"`
		} `json:"tour_thumb"`
	} `json:"tour_stats"`
}

func (s *Scraper) fetchGatsby(ctx context.Context, studioURL string, now time.Time) ([]models.Scene, error) {
	headers := httpx.BrowserHeaders(httpx.UserAgentFirefox)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     s.cfg.base + "/page-data/home/page-data.json",
		Headers: headers,
	})
	if err != nil {
		return nil, err
	}
	var page gatsbyPage
	if err := func() error {
		defer func() { _ = resp.Body.Close() }()
		return httpx.DecodeJSON(resp.Body, &page)
	}(); err != nil {
		return nil, fmt.Errorf("decoding page-data: %w", err)
	}

	nodes := page.Result.Data.AllUpdates.Nodes
	scenes := make([]models.Scene, 0, len(nodes))
	for _, n := range nodes {
		if n.MysqlID == 0 {
			continue
		}
		scene := models.Scene{
			ID:        fmt.Sprintf("%d", n.MysqlID),
			SiteID:    s.cfg.id,
			StudioURL: studioURL,
			Title:     html.UnescapeString(strings.TrimSpace(n.ShortTitle)),
			URL:       s.cfg.base,
			Studio:    s.cfg.studio,
			Date:      parseISODate(n.ReleaseDate),
			ScrapedAt: now,
		}
		if len(n.TourStats) > 0 {
			if src := n.TourStats[0].TourThumb.LocalImage.ChildImageSharp.GatsbyImageData.Images.Fallback.Src; src != "" {
				scene.Thumbnail = s.cfg.base + src
			}
		}
		scenes = append(scenes, scene)
	}
	return scenes, nil
}

// ---- Next.js front-end (Casting Couch-HD / Net Girl) ----

var nextDataRe = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

type nextData struct {
	Props struct {
		PageProps struct {
			Videos []nextVideo `json:"videos"`
		} `json:"pageProps"`
	} `json:"props"`
}

type nextVideo struct {
	ID            int         `json:"id"`
	ShortTitle    string      `json:"short_title"`
	CustomTitle   string      `json:"custom_title"`
	ReleaseDate   string      `json:"release_date"`
	VideoDuration int         `json:"video_duration"`
	AllModels     string      `json:"allModels"`
	Models        []nextModel `json:"models"`
}

type nextModel struct {
	ModelName string `json:"model_name"`
}

func (s *Scraper) fetchNext(ctx context.Context, studioURL string, now time.Time) ([]models.Scene, error) {
	headers := httpx.BrowserHeaders(httpx.UserAgentFirefox)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{URL: s.cfg.base + "/", Headers: headers})
	if err != nil {
		return nil, err
	}
	body, err := func() ([]byte, error) {
		defer func() { _ = resp.Body.Close() }()
		return httpx.ReadBody(resp.Body)
	}()
	if err != nil {
		return nil, fmt.Errorf("reading home page: %w", err)
	}

	m := nextDataRe.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("no __NEXT_DATA__ block found")
	}
	var data nextData
	if err := json.Unmarshal(m[1], &data); err != nil {
		return nil, fmt.Errorf("decoding __NEXT_DATA__: %w", err)
	}

	videos := data.Props.PageProps.Videos
	scenes := make([]models.Scene, 0, len(videos))
	for _, v := range videos {
		if v.ID == 0 {
			continue
		}
		title := v.CustomTitle
		if strings.TrimSpace(title) == "" {
			title = v.ShortTitle
		}
		scene := models.Scene{
			ID:        fmt.Sprintf("%d", v.ID),
			SiteID:    s.cfg.id,
			StudioURL: studioURL,
			Title:     html.UnescapeString(strings.TrimSpace(title)),
			URL:       s.cfg.base,
			Studio:    s.cfg.studio,
			Date:      parseNextDate(v.ReleaseDate),
			Duration:  v.VideoDuration,
			ScrapedAt: now,
		}
		scene.Performers = parsePerformers(v)
		if s.cfg.thumbBase != "" {
			scene.Thumbnail = fmt.Sprintf("%s%d-1-med.jpg", s.cfg.thumbBase, v.ID)
		}
		scenes = append(scenes, scene)
	}
	return scenes, nil
}

func parsePerformers(v nextVideo) []string {
	var names []string
	seen := map[string]bool{}
	for _, m := range v.Models {
		n := strings.TrimSpace(m.ModelName)
		if n != "" && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	if len(names) == 0 && strings.TrimSpace(v.AllModels) != "" {
		for _, n := range strings.Split(v.AllModels, ",") {
			n = strings.TrimSpace(n)
			if n != "" && !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	return names
}

// ---- date parsing ----

// parseISODate parses release dates like "2026-06-23T21:27:25.000Z".
func parseISODate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// parseNextDate parses release dates like "2026-06-19 14:13:45".
func parseNextDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
