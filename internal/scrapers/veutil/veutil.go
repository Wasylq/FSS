// Package veutil scrapes WordPress sites running the "video-elements" theme.
// All sites expose the standard WP REST API without authentication.
package veutil

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
	"github.com/Wasylq/FSS/scraper"
)

// postsPerPage is the WP REST page size used for both the post listing and the
// tag enumeration. WP caps per_page at 100.
const postsPerPage = 100

type SiteConfig struct {
	ID             string
	Studio         string
	SiteBase       string
	MainCategoryID int // WP category ID for real content (usually 1)
	Patterns       []string
	MatchRe        *regexp.Regexp
}

type Scraper struct {
	cfg    SiteConfig
	Client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	if cfg.MainCategoryID == 0 {
		cfg.MainCategoryID = 1
	}
	return &Scraper{
		cfg:    cfg,
		Client: httpx.NewClient(30 * time.Second),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string { return s.cfg.ID }
func (s *Scraper) Patterns() []string {
	return append(s.cfg.Patterns, strings.TrimPrefix(s.cfg.SiteBase, "https://")+"tag/{slug}")
}
func (s *Scraper) MatchesURL(u string) bool { return s.cfg.MatchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- API types ----

type wpPost struct {
	ID      int        `json:"id"`
	DateGMT string     `json:"date_gmt"`
	Link    string     `json:"link"`
	Title   wpRendered `json:"title"`
	Content wpRendered `json:"content"`
	Tags    []int      `json:"tags"`
}

type wpRendered struct {
	Rendered string `json:"rendered"`
}

type wpTag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug,omitempty"`
}

// ---- runner ----

var tagSlugRe = regexp.MustCompile(`/tag/([^/?#]+)`)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Debugf(1, "%s: fetching tags", s.cfg.ID)
	tagMap, err := s.fetchAllTags(ctx)
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("tags: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	var tagFilter int
	if m := tagSlugRe.FindStringSubmatch(studioURL); m != nil {
		slug := strings.TrimRight(m[1], "/")
		scraper.Debugf(1, "%s: detected tag page: %s", s.cfg.ID, slug)
		tagFilter, err = s.resolveTagID(ctx, slug)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("resolve tag %q: %w", slug, err)):
			case <-ctx.Done():
			}
			return
		}
		scraper.Debugf(1, "%s: resolved tag %q to ID %d", s.cfg.ID, slug, tagFilter)
	}

	now := time.Now().UTC()
	totalPages := 0
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		posts, total, pages, err := s.fetchPostsFiltered(ctx, page, tagFilter)
		if err != nil {
			return scraper.PageResult{}, err
		}
		if pages > 0 {
			totalPages = pages
		}

		if len(posts) == 0 {
			return scraper.PageResult{}, nil
		}

		scenes := make([]models.Scene, len(posts))
		for i, p := range posts {
			scenes[i] = s.postToScene(studioURL, p, tagMap, now)
		}
		// WP REST answers HTTP 400 for a page past the last one, which would
		// surface as a spurious error on every full run. X-WP-TotalPages says
		// where to stop; the page-size check is the fallback for a response
		// that omits the header.
		return scraper.PageResult{
			Scenes: scenes,
			Total:  total,
			Done:   (totalPages > 0 && page >= totalPages) || len(posts) < postsPerPage,
		}, nil
	})
}

// ---- tag fetching ----

func (s *Scraper) fetchAllTags(ctx context.Context) (map[int]string, error) {
	tagMap := make(map[int]string)
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		scraper.Debugf(1, "%s: fetching tags page %d", s.cfg.ID, page)
		u := fmt.Sprintf("%s/wp-json/wp/v2/tags?per_page=%d&page=%d&_fields=id,name", s.cfg.SiteBase, postsPerPage, page)
		resp, err := httpx.Do(ctx, s.Client, httpx.Request{
			URL:     u,
			Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
		})
		if err != nil {
			// With a tag count that is an exact multiple of the page size, the
			// loop asks for one page past the end and WP answers HTTP 400.
			// That is the end of the list, not a failure — the tags gathered
			// so far are complete. Only a first-page error is fatal.
			if page > 1 {
				scraper.Debugf(1, "%s: tags page %d past end (%v), stopping", s.cfg.ID, page, err)
				break
			}
			return nil, fmt.Errorf("tags page %d: %w", page, err)
		}

		totalPages, _ := strconv.Atoi(resp.Header.Get("X-WP-TotalPages"))

		var tags []wpTag
		err = func() error {
			defer func() { _ = resp.Body.Close() }()
			return httpx.DecodeJSON(resp.Body, &tags)
		}()
		if err != nil {
			return nil, fmt.Errorf("tags decode: %w", err)
		}

		for _, t := range tags {
			tagMap[t.ID] = t.Name
		}

		if len(tags) < postsPerPage || (totalPages > 0 && page >= totalPages) {
			break
		}
	}
	return tagMap, nil
}

// ---- post fetching ----

// fetchPostsFiltered returns one page of posts along with the total post count
// and total page count from the WP REST pagination headers.
func (s *Scraper) fetchPostsFiltered(ctx context.Context, page int, tagID int) ([]wpPost, int, int, error) {
	u := fmt.Sprintf("%s/wp-json/wp/v2/posts?per_page=%d&page=%d&orderby=date&order=desc&categories=%d&_fields=id,date_gmt,link,title,content,tags",
		s.cfg.SiteBase, postsPerPage, page, s.cfg.MainCategoryID)
	if tagID > 0 {
		u += fmt.Sprintf("&tags=%d", tagID)
	}

	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return nil, 0, 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	total, _ := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	totalPages, _ := strconv.Atoi(resp.Header.Get("X-WP-TotalPages"))

	var posts []wpPost
	if err := httpx.DecodeJSON(resp.Body, &posts); err != nil {
		return nil, 0, 0, fmt.Errorf("decode: %w", err)
	}

	return posts, total, totalPages, nil
}

func (s *Scraper) resolveTagID(ctx context.Context, slug string) (int, error) {
	u := fmt.Sprintf("%s/wp-json/wp/v2/tags?slug=%s&_fields=id", s.cfg.SiteBase, slug)
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var tags []wpTag
	if err := httpx.DecodeJSON(resp.Body, &tags); err != nil {
		return 0, err
	}
	if len(tags) == 0 {
		return 0, fmt.Errorf("tag %q not found", slug)
	}
	return tags[0].ID, nil
}

// ---- scene conversion ----

var posterRe = regexp.MustCompile(`poster="([^"]+)"`)

func extractPoster(content string) string {
	if m := posterRe.FindStringSubmatch(content); m != nil {
		return m[1]
	}
	return ""
}

func (s *Scraper) postToScene(studioURL string, p wpPost, tagMap map[int]string, now time.Time) models.Scene {
	title := html.UnescapeString(p.Title.Rendered)

	var date time.Time
	if p.DateGMT != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", p.DateGMT); err == nil {
			date = t.UTC()
		}
	}

	var performers []string
	for _, tid := range p.Tags {
		if name, ok := tagMap[tid]; ok {
			performers = append(performers, name)
		}
	}

	url := p.Link
	if !strings.HasPrefix(url, "http") {
		url = s.cfg.SiteBase + url
	}

	return models.Scene{
		ID:         strconv.Itoa(p.ID),
		SiteID:     s.cfg.ID,
		StudioURL:  studioURL,
		Title:      title,
		URL:        url,
		Date:       date,
		Thumbnail:  extractPoster(p.Content.Rendered),
		Performers: performers,
		Studio:     s.cfg.Studio,
		ScrapedAt:  now,
	}
}
