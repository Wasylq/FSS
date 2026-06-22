// Package fotoroutil scrapes Fotoro-network fetish sites (Chastity Babes,
// Metal Bondage, HuCows, Shock Challenge, Tieable, Sybian1, Girl Asylum). They
// all run WordPress with the standard, unauthenticated REST API; scenes are
// plain `post` objects. Performers come from the post's tags, content
// categories from its categories. There is no duration field anywhere.
package fotoroutil

import (
	"context"
	"errors"
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

type SiteConfig struct {
	ID       string
	Studio   string
	SiteBase string // e.g. "https://www.hucows.com" — no trailing slash
	Patterns []string
	MatchRe  *regexp.Regexp
}

type Scraper struct {
	cfg    SiteConfig
	Client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	return &Scraper{
		cfg:    cfg,
		Client: httpx.NewClient(30 * time.Second),
	}
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func (s *Scraper) ID() string { return s.cfg.ID }
func (s *Scraper) Patterns() []string {
	return append(s.cfg.Patterns, strings.TrimPrefix(s.cfg.SiteBase, "https://")+"/tag/{slug}")
}
func (s *Scraper) MatchesURL(u string) bool { return s.cfg.MatchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// ---- API types ----

type wpPost struct {
	ID         int        `json:"id"`
	Date       string     `json:"date"`
	Link       string     `json:"link"`
	Title      wpRendered `json:"title"`
	Excerpt    wpRendered `json:"excerpt"`
	Content    wpRendered `json:"content"`
	Tags       []int      `json:"tags"`
	Categories []int      `json:"categories"`
	JetpackImg string     `json:"jetpack_featured_media_url"`
}

type wpRendered struct {
	Rendered string `json:"rendered"`
}

type wpTerm struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ---- runner ----

var tagSlugRe = regexp.MustCompile(`/tag/([^/?#]+)`)

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	scraper.Debugf(1, "%s: fetching tags + categories", s.cfg.ID)
	tagMap, err := s.fetchTerms(ctx, "tags")
	if err != nil {
		s.sendErr(ctx, out, fmt.Errorf("tags: %w", err))
		return
	}
	catMap, err := s.fetchTerms(ctx, "categories")
	if err != nil {
		s.sendErr(ctx, out, fmt.Errorf("categories: %w", err))
		return
	}

	var tagFilter int
	if m := tagSlugRe.FindStringSubmatch(studioURL); m != nil {
		slug := strings.TrimRight(m[1], "/")
		scraper.Debugf(1, "%s: detected tag page: %s", s.cfg.ID, slug)
		tagFilter, err = s.resolveTagID(ctx, slug)
		if err != nil {
			s.sendErr(ctx, out, fmt.Errorf("resolve tag %q: %w", slug, err))
			return
		}
		scraper.Debugf(1, "%s: resolved tag %q to ID %d", s.cfg.ID, slug, tagFilter)
	}

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, s.cfg.ID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		posts, total, err := s.fetchPosts(ctx, page, tagFilter)
		if err != nil {
			return scraper.PageResult{}, err
		}
		scenes := make([]models.Scene, len(posts))
		for i, p := range posts {
			scenes[i] = s.postToScene(studioURL, p, tagMap, catMap, now)
		}
		return scraper.PageResult{Scenes: scenes, Total: total, Done: len(posts) < 100}, nil
	})
}

func (s *Scraper) sendErr(ctx context.Context, out chan<- scraper.SceneResult, err error) {
	select {
	case out <- scraper.Error(err):
	case <-ctx.Done():
	}
}

// ---- term fetching ----

func (s *Scraper) fetchTerms(ctx context.Context, kind string) (map[int]string, error) {
	terms := make(map[int]string)
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		u := fmt.Sprintf("%s/wp-json/wp/v2/%s?per_page=100&page=%d&_fields=id,name", s.cfg.SiteBase, kind, page)
		resp, err := httpx.Do(ctx, s.Client, httpx.Request{
			URL:     u,
			Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
		})
		if err != nil {
			// WP returns 400 when paging past the last page — treat as done.
			var se *httpx.StatusError
			if errors.As(err, &se) && se.StatusCode == http.StatusBadRequest {
				break
			}
			return nil, fmt.Errorf("%s page %d: %w", kind, page, err)
		}
		var batch []wpTerm
		err = func() error {
			defer func() { _ = resp.Body.Close() }()
			return httpx.DecodeJSON(resp.Body, &batch)
		}()
		if err != nil {
			return nil, fmt.Errorf("%s decode: %w", kind, err)
		}
		for _, t := range batch {
			terms[t.ID] = html.UnescapeString(t.Name)
		}
		if len(batch) < 100 {
			break
		}
	}
	return terms, nil
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

	var tags []wpTerm
	if err := httpx.DecodeJSON(resp.Body, &tags); err != nil {
		return 0, err
	}
	if len(tags) == 0 {
		return 0, fmt.Errorf("tag %q not found", slug)
	}
	return tags[0].ID, nil
}

// ---- post fetching ----

func (s *Scraper) fetchPosts(ctx context.Context, page, tagID int) ([]wpPost, int, error) {
	u := fmt.Sprintf("%s/wp-json/wp/v2/posts?per_page=100&page=%d&orderby=date&order=desc&_fields=id,date,link,title,excerpt,content,tags,categories,jetpack_featured_media_url",
		s.cfg.SiteBase, page)
	if tagID > 0 {
		u += fmt.Sprintf("&tags=%d", tagID)
	}
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     u,
		Headers: httpx.BrowserHeaders(httpx.UserAgentChrome),
	})
	if err != nil {
		var se *httpx.StatusError
		if errors.As(err, &se) && se.StatusCode == http.StatusBadRequest {
			return nil, 0, nil // past last page
		}
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	total, _ := strconv.Atoi(resp.Header.Get("X-WP-Total"))

	var posts []wpPost
	if err := httpx.DecodeJSON(resp.Body, &posts); err != nil {
		return nil, 0, fmt.Errorf("decode: %w", err)
	}
	return posts, total, nil
}

// ---- scene conversion ----

var (
	imgSrcRe = regexp.MustCompile(`<img[^>]+src="([^"]+)"`)
	tagStrip = regexp.MustCompile(`<[^>]+>`)
)

func (s *Scraper) postToScene(studioURL string, p wpPost, tagMap, catMap map[int]string, now time.Time) models.Scene {
	var date time.Time
	if p.Date != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", p.Date); err == nil {
			date = t.UTC()
		}
	}

	var performers []string
	for _, tid := range p.Tags {
		if name, ok := tagMap[tid]; ok {
			performers = append(performers, name)
		}
	}
	var categories []string
	for _, cid := range p.Categories {
		if name, ok := catMap[cid]; ok && !strings.EqualFold(name, "Uncategorized") {
			categories = append(categories, name)
		}
	}

	thumb := p.JetpackImg
	if thumb == "" {
		if m := imgSrcRe.FindStringSubmatch(p.Content.Rendered); m != nil {
			thumb = m[1]
		}
	}

	url := p.Link
	if !strings.HasPrefix(url, "http") {
		url = s.cfg.SiteBase + url
	}

	return models.Scene{
		ID:          strconv.Itoa(p.ID),
		SiteID:      s.cfg.ID,
		StudioURL:   studioURL,
		Title:       html.UnescapeString(p.Title.Rendered),
		URL:         url,
		Date:        date,
		Description: cleanText(p.Excerpt.Rendered),
		Thumbnail:   thumb,
		Performers:  performers,
		Categories:  categories,
		Studio:      s.cfg.Studio,
		ScrapedAt:   now,
	}
}

func cleanText(htmlStr string) string {
	s := tagStrip.ReplaceAllString(htmlStr, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}
