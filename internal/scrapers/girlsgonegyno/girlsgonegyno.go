// Package girlsgonegyno scrapes GirlsGoneGyno (girlsgonegyno.com), a PornCMS.com
// site served via privatemediacloud.com.
//
// The video grid is lazy-loaded: the "Videos" listing page
// (?mb=VmlkZW9zfHw=, base64 for "Videos||") is a thin wrapper whose
// #mainbody is populated by a jQuery .load() call pointing at a per-page
// content fragment under content/pages/<hash>.list.htm. Pagination is
// driven by index.php?mb=VmlkZW9zfHw=&clearcache=1&p=<offset> where offset
// steps by 27 (the page size); each wrapper embeds the fragment hash for
// that page. So each page is two requests: fetch the wrapper to discover
// the fragment hash, then fetch the fragment grid and parse the cards.
//
// Each card carries the full per-scene metadata (video id, title,
// performers, posted date, length, views, thumbnail), so no detail-page
// fetch is needed. The scene detail/trailer URL is ?mb=Trailer||<id>
// (base64-encoded).
package girlsgonegyno

import (
	"context"
	"encoding/base64"
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
	siteBase = "https://www.girlsgonegyno.com"
	siteID   = "girlsgonegyno"
	pageSize = 27
	// videosMB is base64 for "Videos||", the PornCMS menu token for the
	// newest-first video listing.
	videosMB = "VmlkZW9zfHw="
)

type Scraper struct{ client *http.Client }

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{"girlsgonegyno.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?girlsgonegyno\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		offset := (page - 1) * pageSize
		cards, err := s.fetchPage(ctx, offset)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, 0, len(cards))
		for _, c := range cards {
			if sc, ok := toScene(studioURL, c, now); ok {
				scenes = append(scenes, sc)
			}
		}
		return scraper.PageResult{Scenes: scenes, Done: len(cards) < pageSize}, nil
	})
}

var (
	loadHashRe  = regexp.MustCompile(`\.load\("(content/pages/[a-z0-9_]+\.list\.htm)"\)`)
	cardSplitRe = regexp.MustCompile(`<div class='col-sm-4 img-portfolio'>`)
)

// fetchPage fetches the wrapper for the given offset, extracts the lazy-load
// fragment URL, fetches that fragment, and returns the raw per-card HTML
// chunks.
func (s *Scraper) fetchPage(ctx context.Context, offset int) ([]string, error) {
	wrapperURL := fmt.Sprintf("%s/index.php?mb=%s&clearcache=1&p=%d", siteBase, videosMB, offset)
	wrapper, err := s.fetchBody(ctx, wrapperURL)
	if err != nil {
		return nil, fmt.Errorf("fetching wrapper: %w", err)
	}
	m := loadHashRe.FindStringSubmatch(wrapper)
	if m == nil {
		scraper.Debugf(1, "%s: no fragment hash in wrapper for offset %d", siteID, offset)
		return nil, nil
	}
	fragURL := siteBase + "/" + m[1]
	scraper.Debugf(1, "%s: loading fragment %s", siteID, m[1])
	frag, err := s.fetchBody(ctx, fragURL)
	if err != nil {
		return nil, fmt.Errorf("fetching fragment: %w", err)
	}
	parts := cardSplitRe.Split(frag, -1)
	if len(parts) <= 1 {
		return nil, nil
	}
	return parts[1:], nil
}

func (s *Scraper) fetchBody(ctx context.Context, u string) (string, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

var (
	ridRe        = regexp.MustCompile(`data-rid="([a-z0-9]+)"`)
	titleRe      = regexp.MustCompile(`(?s)<h4[^>]*>\s*<a[^>]+>([^<]+)</a>`)
	thumbRe      = regexp.MustCompile(`<img class='img-responsive thumbvideo'[^>]+src='([^']+)'`)
	modelBlockRe = regexp.MustCompile(`(?s)<strong>Model:\s*</strong>(.*?)<br>`)
	performerRe  = regexp.MustCompile(`<a[^>]*>([^<]+)</a>`)
	postedRe     = regexp.MustCompile(`<strong>Posted:\s*</strong>([^<]+)<br>`)
	lengthRe     = regexp.MustCompile(`<strong>Length:\s*</strong>([0-9:]+)`)
	viewsRe      = regexp.MustCompile(`<strong>Views:\s*</strong>([0-9,]+)`)
)

func toScene(studioURL, card string, now time.Time) (models.Scene, bool) {
	m := ridRe.FindStringSubmatch(card)
	if m == nil {
		return models.Scene{}, false
	}
	id := m[1]
	scene := models.Scene{
		ID:        id,
		SiteID:    siteID,
		StudioURL: studioURL,
		URL:       siteBase + "/?mb=" + base64.StdEncoding.EncodeToString([]byte("Trailer||"+id)),
		Studio:    "GirlsGoneGyno",
		ScrapedAt: now,
	}
	if t := titleRe.FindStringSubmatch(card); t != nil {
		scene.Title = html.UnescapeString(strings.TrimSpace(t[1]))
	}
	if th := thumbRe.FindStringSubmatch(card); th != nil {
		scene.Thumbnail = th[1]
	}
	if mb := modelBlockRe.FindStringSubmatch(card); mb != nil {
		for _, p := range performerRe.FindAllStringSubmatch(mb[1], -1) {
			name := html.UnescapeString(strings.TrimSpace(p[1]))
			if name != "" {
				scene.Performers = append(scene.Performers, name)
			}
		}
	}
	if p := postedRe.FindStringSubmatch(card); p != nil {
		if d, err := parseutil.TryParseDate(strings.TrimSpace(p[1]), "Mon, 2 Jan 2006"); err == nil {
			scene.Date = d.UTC()
		}
	}
	if l := lengthRe.FindStringSubmatch(card); l != nil {
		scene.Duration = parseutil.ParseDurationColon(l[1])
	}
	if v := viewsRe.FindStringSubmatch(card); v != nil {
		if n, err := strconv.Atoi(strings.ReplaceAll(v[1], ",", "")); err == nil {
			scene.Views = n
		}
	}
	return scene, true
}
