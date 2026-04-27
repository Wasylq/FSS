// Package veutil scrapes WordPress sites running the "video-elements" theme.
// All sites expose the standard WP REST API without authentication.
package veutil

import (
	"context"
	"encoding/json"
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
	ID             string
	Studio         string
	SiteBase       string
	MainCategoryID int // WP category ID for real content (usually 1)
	Patterns       []string
	MatchRe        *regexp.Regexp
}

type Scraper struct {
	Cfg    SiteConfig
	Client *http.Client
}

func New(cfg SiteConfig) *Scraper {
	if cfg.MainCategoryID == 0 {
		cfg.MainCategoryID = 1
	}
	return &Scraper{
		Cfg:    cfg,
		Client: httpx.NewClient(30 * time.Second),
	}
}

func (s *Scraper) ID() string               { return s.Cfg.ID }
func (s *Scraper) Patterns() []string       { return s.Cfg.Patterns }
func (s *Scraper) MatchesURL(u string) bool { return s.Cfg.MatchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, opts, out)
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
}

// ---- runner ----

func (s *Scraper) run(ctx context.Context, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	tagMap, err := s.fetchAllTags(ctx)
	if err != nil {
		select {
		case out <- scraper.SceneResult{Err: fmt.Errorf("tags: %w", err)}:
		case <-ctx.Done():
		}
		return
	}

	now := time.Now().UTC()

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

		posts, total, err := s.fetchPosts(ctx, page)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			return
		}

		if page == 1 && total > 0 {
			select {
			case out <- scraper.SceneResult{Total: total}:
			case <-ctx.Done():
				return
			}
		}

		if len(posts) == 0 {
			return
		}

		stoppedEarly := false
		for _, p := range posts {
			id := strconv.Itoa(p.ID)
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				stoppedEarly = true
				break
			}

			scene := s.postToScene(p, tagMap, now)
			select {
			case out <- scraper.SceneResult{Scene: scene}:
			case <-ctx.Done():
				return
			}
		}

		if stoppedEarly {
			select {
			case out <- scraper.SceneResult{StoppedEarly: true}:
			case <-ctx.Done():
			}
			return
		}
	}
}

// ---- tag fetching ----

func (s *Scraper) fetchAllTags(ctx context.Context) (map[int]string, error) {
	tagMap := make(map[int]string)
	for page := 1; ; page++ {
		u := fmt.Sprintf("%s/wp-json/wp/v2/tags?per_page=100&page=%d&_fields=id,name", s.Cfg.SiteBase, page)
		resp, err := httpx.Do(ctx, s.Client, httpx.Request{
			URL:     u,
			Headers: map[string]string{"User-Agent": httpx.UserAgentChrome},
		})
		if err != nil {
			return nil, fmt.Errorf("tags page %d: %w", page, err)
		}

		var tags []wpTag
		err = json.NewDecoder(resp.Body).Decode(&tags)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("tags decode: %w", err)
		}

		for _, t := range tags {
			tagMap[t.ID] = t.Name
		}

		if len(tags) < 100 {
			break
		}
	}
	return tagMap, nil
}

// ---- post fetching ----

func (s *Scraper) fetchPosts(ctx context.Context, page int) ([]wpPost, int, error) {
	u := fmt.Sprintf("%s/wp-json/wp/v2/posts?per_page=100&page=%d&orderby=date&order=desc&categories=%d&_fields=id,date_gmt,link,title,content,tags",
		s.Cfg.SiteBase, page, s.Cfg.MainCategoryID)

	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL:     u,
		Headers: map[string]string{"User-Agent": httpx.UserAgentChrome},
	})
	if err != nil {
		return nil, 0, err
	}

	total, _ := strconv.Atoi(resp.Header.Get("X-WP-Total"))

	var posts []wpPost
	err = json.NewDecoder(resp.Body).Decode(&posts)
	_ = resp.Body.Close()
	if err != nil {
		return nil, 0, fmt.Errorf("decode: %w", err)
	}

	return posts, total, nil
}

// ---- scene conversion ----

var posterRe = regexp.MustCompile(`poster="([^"]+)"`)

func extractPoster(content string) string {
	if m := posterRe.FindStringSubmatch(content); m != nil {
		return m[1]
	}
	return ""
}

func (s *Scraper) postToScene(p wpPost, tagMap map[int]string, now time.Time) models.Scene {
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
		url = s.Cfg.SiteBase + url
	}

	return models.Scene{
		ID:         strconv.Itoa(p.ID),
		SiteID:     s.Cfg.ID,
		StudioURL:  s.Cfg.SiteBase,
		Title:      title,
		URL:        url,
		Date:       date,
		Thumbnail:  extractPoster(p.Content.Rendered),
		Performers: performers,
		Studio:     s.Cfg.Studio,
		ScrapedAt:  now,
	}
}
