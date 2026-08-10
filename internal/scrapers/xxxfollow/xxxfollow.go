package xxxfollow

import (
	"context"
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
	defaultBase = "https://www.xxxfollow.com"
	// perPage is the API's own ceiling — a larger limit is rejected with
	// {"error":"The limit must not be greater than 50."} rather than clamped.
	perPage    = 50
	dateFormat = "2006-01-02T15:04:05-0700"
)

// Scraper scrapes creator profiles on xxxfollow.com (formerly xfollow.com), a
// CamSoda-family fan platform. The site is a React SPA, but its backing REST
// API is public and unauthenticated, so scenes come from JSON rather than HTML.
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

func (s *Scraper) ID() string { return "xxxfollow" }

func (s *Scraper) Patterns() []string {
	return []string{
		"xxxfollow.com/{creator}",
		"xxxfollow.com/{creator}/premium",
		"xxxfollow.com/{creator}/premium/best-sellers",
	}
}

// reservedSegments are top-level paths that belong to the site itself rather
// than to a creator. Every profile lives at the site root (/{username}), so
// without this set MatchesURL would claim /support, /tag, /login and the
// locale subdirectories as creators.
var reservedSegments = map[string]bool{
	"account": true, "api": true, "contest": true, "creator": true,
	"e": true, "email": true, "home": true, "img": true, "login": true,
	"manifest.json": true, "media": true, "most-popular": true,
	"newest-creators": true, "password": true, "post": true, "premiums": true,
	"register": true, "robots.txt": true, "search": true, "sitemap.xml": true,
	"static": true, "support": true, "tag": true, "top": true,
	// Locale subdirectories (site_config.locale_subdirs).
	"ar": true, "de": true, "es-419": true, "es-es": true, "es-mx": true,
	"es-us": true, "fr": true, "id": true, "pt-br": true, "tr": true,
}

// matchRe accepts the bare profile plus its /premium tab. xfollow.com is the
// site's former domain and 301s to xxxfollow.com, but StashDB and older links
// still carry it, so both hosts are matched.
var matchRe = regexp.MustCompile(`^https?://(?:www\.)?(?:xxxfollow|xfollow)\.com/([a-zA-Z0-9_.-]+)(?:/premium(?:/(best-sellers))?)?/?$`)

func (s *Scraper) MatchesURL(u string) bool {
	m := matchRe.FindStringSubmatch(u)
	return m != nil && !reservedSegments[strings.ToLower(m[1])]
}

// parseURL extracts the creator username and the listing sort from a studio
// URL. sortBy is empty for the default (most recent first) ordering.
func parseURL(u string) (username, sortBy string) {
	if m := matchRe.FindStringSubmatch(u); m != nil {
		return m[1], m[2]
	}
	// Fallback for test-server URLs, whose host is not xxxfollow.com. Mirrors
	// the real routes: /{user}, /{user}/premium, /{user}/premium/best-sellers.
	parsed, err := url.Parse(u)
	if err != nil {
		return "", ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	if len(parts) >= 3 && parts[1] == "premium" {
		return parts[0], parts[2]
	}
	return parts[0], ""
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	username, sortBy := parseURL(studioURL)
	if username == "" {
		return nil, fmt.Errorf("xxxfollow: cannot extract creator username from %q", studioURL)
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, username, sortBy, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL, username, sortBy string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	if sortBy == "" {
		scraper.Debugf(1, "xxxfollow: scraping premium listing for %q (most recent first)", username)
	} else {
		scraper.Debugf(1, "xxxfollow: scraping premium listing for %q (sort=%s)", username, sortBy)
		// Only the default ordering is date-descending. Under any other sort a
		// known ID can appear on page 1 with unseen older posts behind it, so
		// the early-stop hint must not reach Paginate.
		opts.KnownIDs = nil
	}

	scraper.Paginate(ctx, opts, "xxxfollow", out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		items, err := s.fetchPage(ctx, username, sortBy, page)
		if err != nil {
			return scraper.PageResult{}, err
		}

		scenes := make([]models.Scene, 0, len(items))
		skipped := 0
		for _, it := range items {
			// Premium posts are sold as galleries and may hold pictures or
			// audio instead of video; only video posts are scenes.
			if it.Post.MediaCount.Video == 0 {
				skipped++
				continue
			}
			scenes = append(scenes, toScene(s.base, studioURL, username, it))
		}
		if skipped > 0 {
			scraper.Debugf(1, "xxxfollow: page %d: skipped %d non-video premium posts", page, skipped)
		}

		return scraper.PageResult{
			Scenes: scenes,
			// The API reports no total, so the end is a short page. Keep
			// walking when a full page filtered down to zero scenes.
			Done:     len(items) < perPage,
			Continue: len(items) > 0,
		}, nil
	})
}

type listResponse struct {
	List []listItem `json:"list"`
}

type listItem struct {
	LikeCount    int  `json:"like_count"`
	ViewCount    int  `json:"view_count"`
	CommentCount int  `json:"comment_count"`
	Post         post `json:"post"`
}

type post struct {
	ID        int     `json:"id"`
	Access    string  `json:"access"`
	AmountUSD float64 `json:"amount_usd"`
	CreatedAt string  `json:"created_at"`
	Slug      string  `json:"slug"`
	Text      string  `json:"text"`
	Media     []media `json:"media"`
	Preview   *struct {
		URL      string `json:"url"`
		ThumbURL string `json:"thumb_url"`
	} `json:"preview"`
	MediaCount struct {
		Picture int `json:"picture"`
		Video   int `json:"video"`
		Audio   int `json:"audio"`
	} `json:"media_count"`
	VideoDurationTotal int `json:"video_duration_total"`
	DurationTotal      int `json:"duration_total"`
	Tags               []struct {
		Tag string `json:"tag"`
	} `json:"tags"`
	User struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	} `json:"user"`
}

type media struct {
	Type           string `json:"type"`
	DurationInSecs int    `json:"duration_in_second"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	BlurURL        string `json:"blur_url"`
	ThumbURL       string `json:"thumb_url"`
	PreviewURL     string `json:"preview_url"`
	PreviewSDURL   string `json:"preview_sd_url"`
	StartURL       string `json:"start_url"`
}

func (s *Scraper) fetchPage(ctx context.Context, username, sortBy string, page int) ([]listItem, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(perPage))
	q.Set("page", strconv.Itoa(page))
	// The site omits sort_by entirely for the default "recent" ordering.
	if sortBy != "" {
		q.Set("sort_by", sortBy)
	}

	endpoint := fmt.Sprintf("%s/api/v1/user/%s/media/public/premium?%s",
		s.base, url.PathEscape(username), q.Encode())

	headers := httpx.BrowserHeaders(httpx.UserAgentFirefox)
	headers["Accept"] = "application/json"
	headers["Referer"] = s.base + "/" + username + "/premium"

	resp, err := httpx.Do(ctx, s.client, httpx.Request{URL: endpoint, Headers: headers})
	if err != nil {
		return nil, fmt.Errorf("fetching premium listing: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result listResponse
	if err := httpx.DecodeJSON(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("decoding premium listing: %w", err)
	}
	return result.List, nil
}

func toScene(base, studioURL, username string, it listItem) models.Scene {
	p := it.Post
	now := time.Now().UTC()

	sc := models.Scene{
		ID:        strconv.Itoa(p.ID),
		SiteID:    "xxxfollow",
		StudioURL: studioURL,
		Title:     title(p),
		URL:       sceneURL(base, username, p),
		Duration:  p.VideoDurationTotal,
		Studio:    p.User.DisplayName,
		Likes:     it.LikeCount,
		Views:     it.ViewCount,
		Comments:  it.CommentCount,
		ScrapedAt: now,
	}
	if sc.Duration == 0 {
		sc.Duration = p.DurationTotal
	}
	if p.Text != "" {
		sc.Description = strings.TrimSpace(p.Text)
	}
	if t, err := time.Parse(dateFormat, p.CreatedAt); err == nil {
		sc.Date = t.UTC()
	}
	if p.User.DisplayName != "" {
		sc.Performers = []string{p.User.DisplayName}
	}
	for _, t := range p.Tags {
		if tag := strings.TrimSpace(t.Tag); tag != "" {
			sc.Tags = append(sc.Tags, tag)
		}
	}

	// Prefer the post's own preview image; most posts have none, in which case
	// the blurred still shipped with the locked video is the only thumbnail
	// available to a signed-out client.
	if p.Preview != nil {
		sc.Thumbnail = firstNonEmpty(p.Preview.URL, p.Preview.ThumbURL)
	}
	for _, m := range p.Media {
		if m.Type != "video" {
			continue
		}
		if sc.Thumbnail == "" {
			sc.Thumbnail = firstNonEmpty(m.ThumbURL, m.BlurURL, m.StartURL)
		}
		if sc.Preview == "" {
			sc.Preview = firstNonEmpty(m.PreviewURL, m.PreviewSDURL)
		}
		if sc.Width == 0 && m.Width > 0 {
			sc.Width, sc.Height = m.Width, m.Height
		}
		if sc.Duration == 0 {
			sc.Duration = m.DurationInSecs
		}
		break
	}

	sc.AddPrice(models.PriceSnapshot{
		Date:    now,
		Regular: p.AmountUSD,
		IsFree:  p.Access == "free" || (p.Access != "paid" && p.AmountUSD == 0),
	})

	return sc
}

// title mirrors the site's own fallback chain: the post's caption, else its
// tags, else a generic label. A premium post with neither is rare but real.
func title(p post) string {
	if t := strings.TrimSpace(p.Text); t != "" {
		return t
	}
	var tags []string
	for _, t := range p.Tags {
		if tag := strings.TrimSpace(t.Tag); tag != "" {
			tags = append(tags, tag)
		}
		if len(tags) == 3 {
			break
		}
	}
	if len(tags) > 0 {
		return strings.Join(tags, ", ")
	}
	return "Video " + strconv.Itoa(p.ID)
}

// sceneURL builds the canonical permalink. Slugless posts have no address
// under /{user}/premium/, but /post/{id} resolves for every post.
func sceneURL(base, username string, p post) string {
	if p.Slug == "" {
		return base + "/post/" + strconv.Itoa(p.ID)
	}
	return base + "/" + username + "/premium/" + p.Slug
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
