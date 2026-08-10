package camsoda

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Anastylosis/FSS/internal/httpx"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

const (
	defaultBase = "https://www.camsoda.com"
	// mediaDateFormat is the model media API's created_at layout. It carries no
	// zone; the API reports UTC.
	mediaDateFormat = "2006-01-02 15:04:05"
)

// Scraper scrapes CamSoda. The site is a cam platform rather than a tube, so
// its scrapeable catalogue is in two places: the studio-produced
// /exclusive-videos collection, and each model's own for-sale media library.
//
// CamSoda's rate limiter is a small token bucket — two requests back to back
// are fine, a third within the same second draws a 429. Both modes are built
// to issue exactly one request per scrape, so a single scrape cannot trip it
// and there is no RecommendedDelay: opts.Delay only paces a scraper's own page
// walk, and there is no walk here. Scraping many CamSoda URLs in one command
// can still hit the limiter, which httpx.Do absorbs by retrying 429s.
type Scraper struct {
	client *http.Client
	base   string // site base URL, overridable for tests
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   defaultBase,
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "camsoda" }

func (s *Scraper) Patterns() []string {
	return []string{
		"camsoda.com",
		"camsoda.com/exclusive-videos",
		"camsoda.com/{model}",
		"camsoda.com/{model}/media",
		"camsoda.com/{model}/bio",
	}
}

// reservedSegments are top-level paths owned by the site rather than by a
// model. Model profiles sit at the site root, so without this set MatchesURL
// would claim /support, /porn, /girls and the locale prefixes as models. The
// list is the first path segment of every entry in the SPA's own route table
// (`path:"…"` in the sites-default bundle), plus the gender collection routes,
// the locale prefixes the page's hreflang links advertise, and static infra.
var reservedSegments = map[string]bool{
	"account": true, "affiliates": true, "baddiehub": true, "best": true,
	"billing": true, "bitcast": true, "casa-salsa": true, "contests": true,
	"crowdcast": true, "dickometrics": true, "discover": true, "email": true,
	"esports": true, "exclusive-videos": true, "feel-her": true,
	"free-register": true, "games": true, "girls": true, "help": true,
	"inbox": true, "irishcurse": true, "legal": true, "login": true,
	"media": true, "membership": true, "model-referrals": true,
	"models": true, "most-liked": true, "multi-filter": true, "nbafinals": true,
	"pg-login": true, "porn": true, "porn-gifs": true, "press": true,
	"promo-wheel": true, "redir": true, "refer": true, "register": true,
	"search": true, "showresetpassword": true, "speed-date": true,
	"studio": true, "support": true, "vault": true, "verify": true,
	"verify-age": true, "versus-tournament": true, "vibe": true,
	"video-search": true, "view-history": true, "welcome": true,
	"wifeaway": true,
	// Gender collection routes, declared as a regex group in the route table.
	"couples": true, "men": true, "trans": true,
	// Site sections linked from the header that predate the SPA router.
	"about": true, "followed": true, "for-you": true, "reallifecam": true,
	"university": true, "voyeur-cams": true, "voyeur-house": true,
	// Locale prefixes.
	"de": true, "es": true, "fr": true, "id": true, "it": true, "ja": true,
	"nl": true, "pl": true, "pt-br": true, "ru": true,
	// Static infra.
	"api": true, "cdn-cgi": true, "favicon.ico": true, "robots.txt": true,
	"sitemap.xml": true, "static": true,
}

var (
	hostRe = regexp.MustCompile(`^https?://(?:www\.)?camsoda\.com(/.*)?$`)
	// modelRe matches a model profile and the two profile tabs that name the
	// same model. /bio is the URL StashDB records for some performers.
	modelRe = regexp.MustCompile(`^/([a-zA-Z0-9_.-]+)(?:/(?:media|bio))?/?$`)
)

func (s *Scraper) MatchesURL(u string) bool {
	// Host-anchored deliberately: parseURL's fallback accepts any host so
	// httptest URLs work, which must not let this scraper claim another
	// site's URLs.
	m := hostRe.FindStringSubmatch(u)
	if m == nil {
		return false
	}
	mode, arg := classifyPath(m[1])
	return mode != modeNone && (mode != modeModel || arg != "")
}

type mode int

const (
	modeNone mode = iota
	// modeExclusive is the studio-produced /exclusive-videos collection. The
	// bare domain maps here too: camsoda.com's front page is the live-cam
	// directory, which has no catalogue to walk, so the StashDB "CamSoda"
	// studio URL resolves to the only studio-produced content on the site.
	modeExclusive
	// modeModel is one model's for-sale media library.
	modeModel
)

// parseURL classifies a studio URL. For modeModel the second return is the
// model username; it is empty otherwise. A non-camsoda.com host falls back to
// path-only classification so httptest servers work — MatchesURL does not use
// this, so the fallback cannot make the scraper claim another site's URLs.
func parseURL(u string) (mode, string) {
	if m := hostRe.FindStringSubmatch(u); m != nil {
		return classifyPath(m[1])
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return modeNone, ""
	}
	return classifyPath(parsed.Path)
}

// classifyPath maps a site-relative path to a scrape mode.
func classifyPath(path string) (mode, string) {
	if path == "" || path == "/" {
		return modeExclusive, ""
	}
	if trimmed := strings.TrimSuffix(path, "/"); trimmed == "/exclusive-videos" {
		return modeExclusive, ""
	}
	if mm := modelRe.FindStringSubmatch(path); mm != nil {
		if reservedSegments[strings.ToLower(mm[1])] {
			return modeNone, ""
		}
		return modeModel, mm[1]
	}
	return modeNone, ""
}

// ListScenes ignores ListOpts: both modes fetch the whole catalogue in one
// request, so there is no page walk for Delay to pace and no early stop for
// KnownIDs to trigger.
func (s *Scraper) ListScenes(ctx context.Context, studioURL string, _ scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	m, username := parseURL(studioURL)
	if m == modeNone || (m == modeModel && username == "") {
		return nil, fmt.Errorf("camsoda: unsupported URL %q", studioURL)
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, m, username, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, m mode, username string, out chan<- scraper.SceneResult) {
	defer close(out)

	var (
		scenes []models.Scene
		err    error
	)
	switch m {
	case modeExclusive:
		scraper.Debugf(1, "camsoda: scraping the exclusive-videos collection")
		scenes, err = s.exclusiveVideos(ctx, studioURL)
	case modeModel:
		scraper.Debugf(1, "camsoda: scraping media library for model %q", username)
		scenes, err = s.modelMedia(ctx, studioURL, username)
	case modeNone:
		err = fmt.Errorf("camsoda: unsupported URL %q", studioURL)
	}
	if err != nil {
		send(ctx, out, scraper.Error(err))
		return
	}

	// Both endpoints return the whole catalogue in a single response, so
	// there is no page walk to cut short: KnownIDs would save no requests and
	// is deliberately ignored. The model library is also not strictly
	// date-ordered, which would make an early stop wrong as well as useless.
	scraper.Debugf(1, "camsoda: %d scenes", len(scenes))
	if !send(ctx, out, scraper.Progress(len(scenes))) {
		return
	}
	for _, sc := range scenes {
		if !send(ctx, out, scraper.Scene(sc)) {
			return
		}
	}
}

func (s *Scraper) get(ctx context.Context, endpoint, referer string) ([]byte, error) {
	headers := httpx.BrowserHeaders(httpx.UserAgentFirefox)
	if referer != "" {
		headers["Referer"] = referer
		headers["Sec-Fetch-Site"] = "same-origin"
	}
	resp, err := httpx.Do(ctx, s.client, httpx.Request{URL: endpoint, Headers: headers})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}

// --- Exclusive videos -------------------------------------------------------

// preloadedStateRe lifts the SSR Redux state the page ships. The exclusive
// videos are rendered from it, so the whole collection arrives in this one
// document and there is nothing to paginate.
var preloadedStateRe = regexp.MustCompile(`(?s)<script type="application/json" id="__PRELOADED_STATE__">(.*?)</script>`)

type preloadedState struct {
	ExclusiveVideos struct {
		VideoList []exclusiveVideo `json:"videoList"`
	} `json:"exclusiveVideos"`
}

type exclusiveVideo struct {
	ID          string `json:"id"` // a slug, not a number
	Title       string `json:"title"`
	Desc        string `json:"desc"`
	ThumbName   string `json:"thumb_name"`
	VideoName   string `json:"video_name"`
	VideoWidth  int    `json:"video_width"`
	VideoHeight int    `json:"video_height"`
	Models      []struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"models"`
}

func (s *Scraper) exclusiveVideos(ctx context.Context, studioURL string) ([]models.Scene, error) {
	body, err := s.get(ctx, s.base+"/exclusive-videos", s.base+"/")
	if err != nil {
		return nil, fmt.Errorf("fetching exclusive videos: %w", err)
	}

	m := preloadedStateRe.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("camsoda: no __PRELOADED_STATE__ on the exclusive-videos page")
	}
	var state preloadedState
	if err := json.Unmarshal(m[1], &state); err != nil {
		return nil, fmt.Errorf("decoding exclusive videos state: %w", err)
	}

	list := state.ExclusiveVideos.VideoList
	scenes := make([]models.Scene, 0, len(list))
	now := time.Now().UTC()
	for _, v := range list {
		if v.ID == "" {
			continue
		}
		sc := models.Scene{
			ID:        v.ID,
			SiteID:    "camsoda",
			StudioURL: studioURL,
			Title:     strings.TrimSpace(v.Title),
			URL:       s.base + "/exclusive-videos/" + v.ID,
			// The collection carries no publish dates or runtimes — neither
			// the listing nor the player state exposes them.
			Description: strings.TrimSpace(v.Desc),
			Thumbnail:   v.ThumbName,
			Preview:     v.VideoName,
			Width:       v.VideoWidth,
			Height:      v.VideoHeight,
			Studio:      "CamSoda Exclusive Videos",
			ScrapedAt:   now,
		}
		if sc.Title == "" {
			sc.Title = v.ID
		}
		for _, p := range v.Models {
			if name := strings.TrimSpace(p.Name); name != "" {
				sc.Performers = append(sc.Performers, name)
			}
		}
		scenes = append(scenes, sc)
	}
	return scenes, nil
}

// --- Model media library ----------------------------------------------------

type modelMediaResponse struct {
	User struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	} `json:"user"`
	MediaList []mediaItem `json:"mediaList"`
}

type mediaItem struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Description  string `json:"description"`
	TokenPrice   int    `json:"token_price"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	Duration     int    `json:"duration"`
	IsVideo      bool   `json:"is_video"`
	ThumbnailURL string `json:"thumbnail_url"`
	TypeName     string `json:"type_name"`
	Username     string `json:"username"`
	DisplayName  string `json:"user_display_name"`
}

func (s *Scraper) modelMedia(ctx context.Context, studioURL, username string) ([]models.Scene, error) {
	endpoint := s.base + "/api/v1/user/" + url.PathEscape(username) + "/media"
	body, err := s.get(ctx, endpoint, s.base+"/"+username+"/media")
	if err != nil {
		return nil, fmt.Errorf("fetching media library for %q: %w", username, err)
	}

	var resp modelMediaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding media library for %q: %w", username, err)
	}

	scenes := make([]models.Scene, 0, len(resp.MediaList))
	skipped := 0
	now := time.Now().UTC()
	for _, it := range resp.MediaList {
		// A model's library mixes videos with picture sets and playlists.
		if !it.IsVideo {
			skipped++
			continue
		}
		display := firstNonEmpty(it.DisplayName, resp.User.DisplayName, username)
		sc := models.Scene{
			ID:          strconv.Itoa(it.ID),
			SiteID:      "camsoda",
			StudioURL:   studioURL,
			Title:       strings.TrimSpace(it.Name),
			URL:         s.mediaURL(username, it),
			Description: strings.TrimSpace(it.Description),
			Thumbnail:   it.ThumbnailURL,
			Duration:    it.Duration,
			Studio:      display,
			ScrapedAt:   now,
		}
		if sc.Title == "" {
			sc.Title = "Video " + sc.ID
		}
		if display != "" {
			sc.Performers = []string{display}
		}
		if t, err := time.Parse(mediaDateFormat, it.CreatedAt); err == nil {
			sc.Date = t.UTC()
		}
		// token_price is deliberately not recorded: it is denominated in
		// CamSoda tokens, whose dollar value depends on which package the
		// buyer bought, so there is no honest USD figure to put in a
		// PriceSnapshot. Writing the token count into a USD field would
		// corrupt LowestPrice and cross-site comparisons alike.
		scenes = append(scenes, sc)
	}
	if skipped > 0 {
		scraper.Debugf(1, "camsoda: skipped %d non-video items in %q's library", skipped, username)
	}
	return scenes, nil
}

// mediaURL builds the permalink. The SPA route is /:model/media/:title/:id;
// an item with no slug still resolves from the library page alone.
func (s *Scraper) mediaURL(username string, it mediaItem) string {
	if it.Slug == "" {
		return s.base + "/" + username + "/media"
	}
	return s.base + "/" + username + "/media/" + it.Slug + "/" + strconv.Itoa(it.ID)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func send(ctx context.Context, ch chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case ch <- r:
		return true
	case <-ctx.Done():
		return false
	}
}
