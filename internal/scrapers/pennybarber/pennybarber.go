package pennybarber

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	defaultBase = "https://pennybarber.com"
	siteID      = "pennybarber"
	perPage     = 100
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?pennybarber\.com(?:/videos)?/?(?:\?.*)?$`)

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

func (s *Scraper) ID() string               { return siteID }
func (s *Scraper) Patterns() []string       { return []string{"pennybarber.com/videos"} }
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	for offset := 0; ; offset += perPage {
		if ctx.Err() != nil {
			return
		}

		if offset > 0 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}

		listing, total, err := s.fetchListing(ctx, offset)
		if err != nil {
			send(ctx, out, scraper.Error(err))
			return
		}

		if len(listing) == 0 {
			return
		}

		if offset == 0 && total > 0 {
			if !send(ctx, out, scraper.Progress(total)) {
				return
			}
		}

		now := time.Now().UTC()
		for _, item := range listing {
			id := strconv.Itoa(item.ID)

			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				send(ctx, out, scraper.StoppedEarly())
				return
			}

			if opts.Delay > 0 {
				select {
				case <-time.After(opts.Delay):
				case <-ctx.Done():
					return
				}
			}

			detail, err := s.fetchDetail(ctx, item.ID)
			if err != nil {
				send(ctx, out, scraper.Error(err))
				return
			}

			scene := toScene(item, detail, studioURL, s.base, now)
			if !send(ctx, out, scraper.Scene(scene)) {
				return
			}
		}

		if offset+len(listing) >= total {
			return
		}
	}
}

func send(ctx context.Context, ch chan<- scraper.SceneResult, r scraper.SceneResult) bool {
	select {
	case ch <- r:
		return true
	case <-ctx.Done():
		return false
	}
}

// API types.

type apiResponse struct {
	Status   bool       `json:"status"`
	Response apiPayload `json:"response"`
}

type apiPayload struct {
	Collection []apiScene `json:"collection"`
	Meta       apiMeta    `json:"meta"`
}

type apiMeta struct {
	TotalCount int `json:"totalCount"`
}

type apiScene struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Len   int    `json:"length"`
	Sites struct {
		Collection map[string]apiSiteEntry `json:"collection"`
	} `json:"sites"`
	Description string          `json:"description"`
	Tags        json.RawMessage `json:"tags"`
}

type apiSiteEntry struct {
	PublishDate string `json:"publishDate"`
}

type apiTagEntry struct {
	Alias string `json:"alias"`
}

func (s *Scraper) fetchListing(ctx context.Context, offset int) ([]apiScene, int, error) {
	u := fmt.Sprintf("%s/api/content.load?_method=content.load&tz=2"+
		"&fields[0]=id&fields[1]=title&fields[2]=length&fields[3]=sites.publishDate"+
		"&limit=%d&offset=%d"+
		"&metaFields[totalCount]=1"+
		"&transitParameters[preset]=videos",
		s.base, perPage, offset)

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: u,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
			"Accept":     "application/json",
		},
	})
	if err != nil {
		return nil, 0, fmt.Errorf("fetch listing offset %d: %w", offset, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var ar apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, 0, fmt.Errorf("parse listing: %w", err)
	}

	return ar.Response.Collection, ar.Response.Meta.TotalCount, nil
}

func (s *Scraper) fetchDetail(ctx context.Context, id int) (*apiScene, error) {
	u := fmt.Sprintf("%s/api/content.load?_method=content.load&tz=2"+
		"&filter[id][fields][0]=id&filter[id][values][0]=%d"+
		"&fields[0]=id&fields[1]=title&fields[2]=description"+
		"&fields[3]=tags&fields[4]=tags.alias"+
		"&fields[5]=length&fields[6]=sites.publishDate"+
		"&limit=1"+
		"&transitParameters[preset]=scene",
		s.base, id)

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: u,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
			"Accept":     "application/json",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("fetch detail %d: %w", id, err)
	}
	defer func() { _ = resp.Body.Close() }()

	var ar apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("parse detail %d: %w", id, err)
	}

	if len(ar.Response.Collection) == 0 {
		return nil, fmt.Errorf("detail %d: empty response", id)
	}

	return &ar.Response.Collection[0], nil
}

func parseTags(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	// Try {"collection": {"id": {"alias": "..."}}}
	var wrapped struct {
		Collection map[string]apiTagEntry `json:"collection"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Collection) > 0 {
		tags := make([]string, 0, len(wrapped.Collection))
		for _, t := range wrapped.Collection {
			if t.Alias != "" {
				tags = append(tags, t.Alias)
			}
		}
		return tags
	}

	// Try flat map: {"id": {"alias": "..."}}
	var flat map[string]apiTagEntry
	if err := json.Unmarshal(raw, &flat); err == nil && len(flat) > 0 {
		tags := make([]string, 0, len(flat))
		for _, t := range flat {
			if t.Alias != "" {
				tags = append(tags, t.Alias)
			}
		}
		return tags
	}

	return nil
}

func parsePublishDate(sc apiScene) time.Time {
	idStr := strconv.Itoa(sc.ID)
	if entry, ok := sc.Sites.Collection[idStr]; ok && entry.PublishDate != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", entry.PublishDate); err == nil {
			return t
		}
	}
	for _, entry := range sc.Sites.Collection {
		if entry.PublishDate != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", entry.PublishDate); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func slugify(title string) string {
	var sb strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			prevDash = false
		} else if !prevDash && sb.Len() > 0 {
			sb.WriteByte('-')
			prevDash = true
		}
	}
	return strings.TrimRight(sb.String(), "-")
}

func toScene(listing apiScene, detail *apiScene, studioURL, base string, now time.Time) models.Scene {
	id := strconv.Itoa(listing.ID)
	slug := slugify(listing.Title)
	sceneURL := fmt.Sprintf("%s/scene/%d/%s", base, listing.ID, slug)

	sc := models.Scene{
		ID:         id,
		SiteID:     siteID,
		StudioURL:  studioURL,
		Title:      listing.Title,
		URL:        sceneURL,
		Duration:   listing.Len,
		Date:       parsePublishDate(listing),
		Studio:     "Penny Barber",
		Performers: []string{"Penny Barber"},
		ScrapedAt:  now,
	}

	if detail != nil {
		sc.Description = strings.TrimSpace(detail.Description)
		sc.Tags = parseTags(detail.Tags)
	}

	return sc
}
