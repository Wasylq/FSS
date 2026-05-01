package rachelsteele

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	defaultSiteBase = "https://rachel-steele.com"
	apiPath         = "/api/cancellable-request"
	studioName      = "Rachel Steele"
)

type Scraper struct {
	client   *http.Client
	siteBase string
}

func New() *Scraper {
	return &Scraper{
		client:   httpx.NewClient(30 * time.Second),
		siteBase: defaultSiteBase,
	}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "rachelsteele" }

func (s *Scraper) Patterns() []string {
	return []string{"rachel-steele.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?rachel-steele\.com`)

func (s *Scraper) MatchesURL(u string) bool {
	return matchRe.MatchString(u)
}

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	if opts.Workers <= 0 {
		opts.Workers = 3
	}
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// apiResponse is the outer JSON wrapper from the cancellable-request endpoint.
type apiResponse struct {
	OK   bool            `json:"ok"`
	Data json.RawMessage `json:"data"`
}

// videosPage is the paginated list of videos returned by fetchVideosApi.
type videosPage struct {
	CurrentPage int        `json:"current_page"`
	LastPage    int        `json:"last_page"`
	Total       int        `json:"total"`
	PerPage     int        `json:"per_page"`
	Data        []apiVideo `json:"data"`
}

type apiVideo struct {
	ID                        int    `json:"id"`
	Title                     string `json:"title"`
	IsPublished               bool   `json:"is_published"`
	PublishDate               string `json:"publish_date"`
	Duration                  int    `json:"duration"`
	ContentMappingID          int    `json:"content_mapping_id"`
	ViewsCount                int    `json:"views_count"`
	LikesCount                int    `json:"likes_count"`
	CommentsCount             int    `json:"comments_count"`
	StreamPrice               any    `json:"stream_price"`
	PosterSrc                 string `json:"poster_src"`
	SystemPreviewVideoFullSrc string `json:"system_preview_video_full_src"`
	Has4K                     bool   `json:"has_4k"`
	IsVRVideo                 bool   `json:"is_vr_video"`
}

func (v apiVideo) price() (float64, bool) {
	switch p := v.StreamPrice.(type) {
	case float64:
		return p, true
	case string:
		f, err := strconv.ParseFloat(p, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	work := make(chan apiVideo, opts.Workers)
	var wg sync.WaitGroup

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for vid := range work {
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}
				scene, err := s.buildScene(ctx, studioURL, vid)
				if err != nil {
					select {
					case out <- scraper.Error(err):
					case <-ctx.Done():
						return
					}
					continue
				}
				select {
				case out <- scraper.Scene(scene):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for page := 1; ; page++ {
		if ctx.Err() != nil {
			break
		}
		if page > 1 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				break
			}
			if ctx.Err() != nil {
				break
			}
		}

		vp, err := s.fetchPage(ctx, page)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("page %d: %w", page, err)):
			case <-ctx.Done():
			}
			break
		}

		if page == 1 {
			select {
			case out <- scraper.Progress(vp.Total):
			case <-ctx.Done():
			}
		}

		if len(vp.Data) == 0 {
			break
		}

		cancelled := false
		hitKnown := false
		for _, vid := range vp.Data {
			if !vid.IsPublished {
				continue
			}
			id := strconv.Itoa(vid.ID)
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[id] {
				hitKnown = true
				break
			}
			select {
			case work <- vid:
			case <-ctx.Done():
				cancelled = true
			}
			if cancelled {
				break
			}
		}
		if cancelled || hitKnown {
			if hitKnown {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
			}
			break
		}

		if page >= vp.LastPage {
			break
		}
	}

	close(work)
	wg.Wait()
}

func (s *Scraper) fetchPage(ctx context.Context, page int) (videosPage, error) {
	args, _ := json.Marshal([]any{[]string{fmt.Sprintf("page=%d", page)}})
	reqURL := fmt.Sprintf("%s%s?functionName=fetchVideosApi&args=%s", s.siteBase, apiPath, args)

	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: reqURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return videosPage{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return videosPage{}, err
	}

	var outer apiResponse
	if err := json.Unmarshal(body, &outer); err != nil {
		return videosPage{}, fmt.Errorf("decode API response: %w", err)
	}
	if !outer.OK {
		return videosPage{}, fmt.Errorf("API returned ok=false: %s", string(outer.Data))
	}

	var vp videosPage
	if err := json.Unmarshal(outer.Data, &vp); err != nil {
		return videosPage{}, fmt.Errorf("decode videos page: %w", err)
	}
	return vp, nil
}

func (s *Scraper) buildScene(ctx context.Context, studioURL string, vid apiVideo) (models.Scene, error) {
	now := time.Now().UTC()

	scene := models.Scene{
		ID:        strconv.Itoa(vid.ID),
		SiteID:    "rachelsteele",
		StudioURL: studioURL,
		Title:     vid.Title,
		URL:       fmt.Sprintf("%s/%d-%s", s.siteBase, vid.ContentMappingID, slugify(vid.Title)),
		Thumbnail: vid.PosterSrc,
		Preview:   vid.SystemPreviewVideoFullSrc,
		Duration:  vid.Duration,
		Date:      parseDate(vid.PublishDate),
		Studio:    studioName,
		Views:     vid.ViewsCount,
		Likes:     vid.LikesCount,
		Comments:  vid.CommentsCount,
		ScrapedAt: now,
	}

	if vid.Has4K {
		scene.Width = 3840
		scene.Height = 2160
		scene.Resolution = "2160p"
	}

	if p, ok := vid.price(); ok {
		scene.AddPrice(models.PriceSnapshot{
			Date:    now,
			Regular: p,
			IsFree:  p == 0,
		})
	}

	detail, err := s.fetchDetail(ctx, scene.URL)
	if err == nil {
		if detail.description != "" {
			scene.Description = detail.description
		}
		if len(detail.performers) > 0 {
			scene.Performers = detail.performers
		}
		if len(detail.tags) > 0 {
			scene.Tags = detail.tags
		}
		if detail.thumbnail != "" {
			scene.Thumbnail = detail.thumbnail
		}
	}

	return scene, nil
}

type sceneDetail struct {
	description string
	performers  []string
	tags        []string
	thumbnail   string
}

var (
	ogDescRe   = regexp.MustCompile(`og:description["\s]+content="([^"]+)"`)
	ogImageRe  = regexp.MustCompile(`og:image["\s]+content="([^"]+)"`)
	keywordsRe = regexp.MustCompile(`keywords\\":\\"([^\\]+)\\`)
)

func (s *Scraper) fetchDetail(ctx context.Context, pageURL string) (sceneDetail, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: pageURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return sceneDetail{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return sceneDetail{}, err
	}

	var d sceneDetail

	if m := ogDescRe.FindSubmatch(body); m != nil {
		d.description = html.UnescapeString(string(m[1]))
	}

	if m := ogImageRe.FindSubmatch(body); m != nil {
		img := html.UnescapeString(string(m[1]))
		d.thumbnail = img
	}

	if m := keywordsRe.FindSubmatch(body); m != nil {
		d.performers, d.tags = splitKeywords(string(m[1]))
	}

	return d, nil
}

// knownPerformers is the list of performer names from the /models page.
// Keywords matching these are classified as performers; the rest are tags.
var knownPerformers = map[string]bool{
	"ophelia fae":       true,
	"lily starfire":     true,
	"reya lovenlight":   true,
	"mia simone":        true,
	"mindi mink":        true,
	"leo malonee":       true,
	"pixie smalls":      true,
	"cherie deville":    true,
	"danni jones":       true,
	"ryan keely":        true,
	"josh rivers":       true,
	"anthony pierce":    true,
	"karen fisher":      true,
	"mellanie monroe":   true,
	"richard glaze":     true,
	"damson jenkins":    true,
	"rachael cavalli":   true,
	"london river":      true,
	"max fills":         true,
	"hailey rose":       true,
	"slave marcelo":     true,
	"tyler cruise":      true,
	"honey heston":      true,
	"ares":              true,
	"dallas diamondz":   true,
	"kenny koxx":        true,
	"brycen ward":       true,
	"leihla":            true,
	"keri lynn":         true,
	"arianna labarbara": true,
	"rachel steele":     true,
}

func splitKeywords(keywords string) (performers, tags []string) {
	for _, kw := range strings.Split(keywords, ", ") {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		if knownPerformers[strings.ToLower(kw)] {
			performers = append(performers, kw)
		} else {
			tags = append(tags, kw)
		}
	}
	return performers, tags
}

func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05.000000Z", s)
		if err != nil {
			return time.Time{}
		}
	}
	return t.UTC()
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := strings.ToLower(title)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
