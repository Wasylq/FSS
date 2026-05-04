package lucasentertainment

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

const (
	defaultBase     = "https://www.lucasentertainment.com"
	apiPath         = "/wp-json/wp/v2/posts"
	sceneCategoryID = 10
	perPage         = 100
)

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{
		client: httpx.NewClient(30 * time.Second),
		base:   defaultBase,
	}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "lucasentertainment" }

func (s *Scraper) Patterns() []string {
	return []string{
		"lucasentertainment.com",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?lucasentertainment\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type wpPost struct {
	ID      int    `json:"id"`
	DateGMT string `json:"date_gmt"`
	Slug    string `json:"slug"`
	Link    string `json:"link"`
	Title   struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
	Content struct {
		Rendered string `json:"rendered"`
	} `json:"content"`
	FeaturedMedia int    `json:"featured_media"`
	YoastHead     string `json:"yoast_head"`
	Embedded      struct {
		Media []struct {
			SourceURL string `json:"source_url"`
		} `json:"wp:featuredmedia"`
		Terms [][]struct {
			Name     string `json:"name"`
			Taxonomy string `json:"taxonomy"`
		} `json:"wp:term"`
	} `json:"_embedded"`
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

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

		posts, total, err := s.fetchPage(ctx, page)
		if err != nil {
			send(ctx, out, scraper.Error(fmt.Errorf("page %d: %w", page, err)))
			return
		}

		if page == 1 && total > 0 {
			send(ctx, out, scraper.Progress(total))
		}

		if len(posts) == 0 {
			return
		}

		now := time.Now().UTC()
		for _, p := range posts {
			id := strconv.Itoa(p.ID)
			if opts.KnownIDs != nil && opts.KnownIDs[id] {
				send(ctx, out, scraper.StoppedEarly())
				return
			}
			if !send(ctx, out, scraper.Scene(toScene(studioURL, p, now))) {
				return
			}
		}

		totalPages := (total + perPage - 1) / perPage
		if page >= totalPages {
			return
		}
	}
}

func (s *Scraper) fetchPage(ctx context.Context, page int) ([]wpPost, int, error) {
	u := fmt.Sprintf("%s%s?categories=%d&per_page=%d&page=%d&orderby=date&order=desc&_embed", s.base, apiPath, sceneCategoryID, perPage, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: u,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "application/json",
		},
	})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	total, _ := strconv.Atoi(resp.Header.Get("X-WP-Total"))

	var posts []wpPost
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, 0, fmt.Errorf("decoding response: %w", err)
	}
	return posts, total, nil
}

var (
	ogImageRe = regexp.MustCompile(`og:image"\s+content="([^"]+)"`)
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)
)

func toScene(studioURL string, p wpPost, now time.Time) models.Scene {
	title := html.UnescapeString(p.Title.Rendered)

	desc := htmlTagRe.ReplaceAllString(p.Content.Rendered, "")
	desc = html.UnescapeString(strings.TrimSpace(desc))

	sc := models.Scene{
		ID:          strconv.Itoa(p.ID),
		SiteID:      "lucasentertainment",
		StudioURL:   studioURL,
		Title:       title,
		URL:         p.Link,
		Description: desc,
		Studio:      "Lucas Entertainment",
		Performers:  extractPerformers(title),
		ScrapedAt:   now,
	}

	if p.DateGMT != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", p.DateGMT); err == nil {
			sc.Date = t.UTC()
		}
	}

	if len(p.Embedded.Media) > 0 && p.Embedded.Media[0].SourceURL != "" {
		sc.Thumbnail = p.Embedded.Media[0].SourceURL
	}
	if sc.Thumbnail == "" {
		if m := ogImageRe.FindStringSubmatch(p.YoastHead); m != nil {
			sc.Thumbnail = m[1]
		}
	}

	for _, group := range p.Embedded.Terms {
		for _, term := range group {
			if term.Taxonomy == "post_tag" && term.Name != "" {
				sc.Tags = append(sc.Tags, term.Name)
			}
		}
	}

	return sc
}

var splitWords = map[string]bool{
	"tops": true, "bottoms": true, "fucks": true, "rides": true,
	"pounds": true, "breeds": true, "sucks": true, "services": true,
	"drills": true, "dominates": true, "owns": true, "destroys": true,
	"wrecks": true, "slams": true, "takes": true, "gets": true,
	"goes": true, "has": true, "and": true, "gives": true,
	"barebacks": true, "raw-fucks": true, "flip-fucks": true,
	"service": true, "his": true, "her": true, "their": true,
	"with": true, "in": true, "on": true, "at": true, "the": true,
	"a": true, "an": true, "to": true, "for": true, "from": true,
	"is": true, "are": true, "by": true, "of": true, "or": true,
	"big": true, "huge": true, "deep": true, "raw": true, "hard": true,
	"balls": true, "cock": true, "ass": true, "hole": true, "dick": true,
	"uncut": true, "inch": true, "double": true, "bareback": true,
}

func extractPerformers(title string) []string {
	words := strings.Fields(title)
	var names []string
	var current []string

	flush := func() {
		if len(current) >= 1 {
			names = append(names, strings.Join(current, " "))
		}
		current = nil
	}

	for _, w := range words {
		lower := strings.ToLower(w)
		lower = strings.TrimSuffix(lower, "'s")
		lower = strings.TrimSuffix(lower, "’s")
		if splitWords[lower] {
			flush()
			continue
		}
		if strings.ContainsAny(w, "0123456789-") {
			flush()
			continue
		}
		if isNameWord(w) {
			current = append(current, w)
		} else {
			flush()
		}
	}
	flush()

	var result []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" && len(strings.Fields(n)) <= 4 {
			result = append(result, n)
		}
	}
	return result
}

func isNameWord(w string) bool {
	if len(w) == 0 {
		return false
	}
	r := rune(w[0])
	return r >= 'A' && r <= 'Z'
}

func send(ctx context.Context, ch chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case ch <- r:
		return true
	case <-ctx.Done():
		return false
	}
}
