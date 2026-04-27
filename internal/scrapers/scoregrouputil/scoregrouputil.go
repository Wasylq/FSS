package scoregrouputil

import (
	"context"
	"fmt"
	"html"
	"io"
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

type SiteConfig struct {
	SiteID     string
	SiteBase   string
	StudioName string
	VideosPath string // e.g. "/xxx-milf-videos/"
	ModelsPath string // e.g. "/xxx-milf-models/"
}

type Scraper struct {
	Client *http.Client
	Config SiteConfig
}

func NewScraper(cfg SiteConfig) *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second), Config: cfg}
}

func (s *Scraper) Run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	type detailWork struct {
		listing listingScene
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	work := make(chan detailWork)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for dw := range work {
				scene, err := s.fetchDetail(ctx, dw.listing, opts.Delay)
				if err != nil {
					select {
					case out <- scraper.SceneResult{Err: err}:
					case <-ctx.Done():
						return
					}
					continue
				}
				select {
				case out <- scraper.SceneResult{Scene: scene}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	singlePage := s.isModelURL(studioURL)

	go func() {
		defer close(work)
		for page := 1; ; page++ {
			var url string
			if singlePage {
				url = stripNATS(studioURL)
			} else {
				url = fmt.Sprintf("%s%s?page=%d", s.Config.SiteBase, s.Config.VideosPath, page)
			}

			body, err := s.fetchPage(ctx, url)
			if err != nil {
				select {
				case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
				case <-ctx.Done():
				}
				return
			}

			scenes := parseListingPage(body, s.Config.SiteBase)
			if page == 1 {
				if singlePage {
					select {
					case out <- scraper.SceneResult{Total: len(scenes)}:
					case <-ctx.Done():
						return
					}
				} else {
					totalPages := extractMaxPage(body)
					total := len(scenes) * totalPages
					select {
					case out <- scraper.SceneResult{Total: total}:
					case <-ctx.Done():
						return
					}
				}
			}

			if len(scenes) == 0 {
				return
			}

			for _, sc := range scenes {
				if opts.KnownIDs[sc.id] {
					select {
					case out <- scraper.SceneResult{StoppedEarly: true}:
					case <-ctx.Done():
					}
					return
				}
				select {
				case work <- detailWork{listing: sc}:
				case <-ctx.Done():
					return
				}
			}

			if singlePage || !hasNextPage(body, page) {
				return
			}

			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
}

func (s *Scraper) isModelURL(u string) bool {
	if s.Config.ModelsPath != "" && strings.Contains(u, s.Config.ModelsPath) {
		return true
	}
	return false
}

func stripNATS(u string) string {
	if idx := strings.Index(u, "?nats="); idx > 0 {
		return u[:idx]
	}
	if idx := strings.Index(u, "&nats="); idx > 0 {
		return u[:idx]
	}
	return u
}

type listingScene struct {
	id         string
	url        string
	title      string
	performers []string
	duration   int
	thumb      string
}

var (
	sceneStartRe = regexp.MustCompile(`class="li-item compact video"`)
	sceneLinkRe  = regexp.MustCompile(`href="(https?://[^"]*?/(\d+)/\?[^"]*)"[^>]*>\s*(?:<[^>]+>\s*)*<div class="lazyload-wrap`)
	titleRe      = regexp.MustCompile(`class="i-title[^"]*">\s*([^<]+)`)
	modelRe      = regexp.MustCompile(`class="i-model"[^>]*>([^<]+)`)
	durationRe   = regexp.MustCompile(`class="time-ol[^"]*"[^>]*>(\d+):(\d+)\s*mins?`)
	thumbRe      = regexp.MustCompile(`src="(https?://[^"]*posting_\d+[^"]*\.jpg)"`)
)

func parseListingPage(body []byte, siteBase string) []listingScene {
	html := string(body)
	locs := sceneStartRe.FindAllStringIndex(html, -1)
	var scenes []listingScene

	for i, loc := range locs {
		start := loc[0]
		end := len(html)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := html[start:end]

		linkM := sceneLinkRe.FindStringSubmatch(block)
		if linkM == nil {
			continue
		}
		rawURL := linkM[1]
		id := linkM[2]

		// Strip NATS tracking from URL
		cleanURL := rawURL
		if idx := strings.Index(rawURL, "?nats="); idx > 0 {
			cleanURL = rawURL[:idx+1]
		}
		cleanURL = strings.TrimSuffix(cleanURL, "?")
		if !strings.HasSuffix(cleanURL, "/") {
			cleanURL += "/"
		}

		titleM := titleRe.FindStringSubmatch(block)
		title := ""
		if titleM != nil {
			title = strings.TrimSpace(titleM[1])
		}

		modelM := modelRe.FindStringSubmatch(block)
		var performers []string
		if modelM != nil {
			for _, p := range strings.Split(modelM[1], ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					performers = append(performers, p)
				}
			}
		}

		dur := 0
		durM := durationRe.FindStringSubmatch(block)
		if durM != nil {
			mins, _ := strconv.Atoi(durM[1])
			secs, _ := strconv.Atoi(durM[2])
			dur = mins*60 + secs
		}

		thumbM := thumbRe.FindStringSubmatch(block)
		thumb := ""
		if thumbM != nil {
			thumb = thumbM[1]
		}

		scenes = append(scenes, listingScene{
			id:         id,
			url:        cleanURL,
			title:      title,
			performers: performers,
			duration:   dur,
			thumb:      thumb,
		})
	}
	return scenes
}

var pageRe = regexp.MustCompile(`page=(\d+)`)

func extractMaxPage(body []byte) int {
	matches := pageRe.FindAllSubmatch(body, -1)
	max := 1
	for _, m := range matches {
		n, _ := strconv.Atoi(string(m[1]))
		if n > max {
			max = n
		}
	}
	return max
}

func hasNextPage(body []byte, current int) bool {
	matches := pageRe.FindAllSubmatch(body, -1)
	for _, m := range matches {
		n, _ := strconv.Atoi(string(m[1]))
		if n > current {
			return true
		}
	}
	return false
}

var (
	metaDateRe = regexp.MustCompile(`(?:itemprop="uploadDate"|name="Date")\s+content="([^"]+)"`)
	ogDescRe   = regexp.MustCompile(`property="og:description"\s+content="([^"]+)"`)
	tagRe      = regexp.MustCompile(`href="[^"]*updates-tag/([^/]+)/\d+/[^"]*"[^>]*class="btn btn-ol-2[^"]*"[^>]*>\s*([^<]+)`)
)

func (s *Scraper) fetchDetail(ctx context.Context, ls listingScene, delay time.Duration) (models.Scene, error) {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return models.Scene{}, ctx.Err()
		}
	}

	body, err := s.fetchPage(ctx, ls.url)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", ls.id, err)
	}

	var date time.Time
	if m := metaDateRe.FindSubmatch(body); m != nil {
		if t, err := time.Parse(time.RFC3339, string(m[1])); err == nil {
			date = t.UTC()
		} else if t, err := time.Parse("2006-01-02T15:04:05+00:00", string(m[1])); err == nil {
			date = t.UTC()
		}
	}

	desc := ""
	if m := ogDescRe.FindSubmatch(body); m != nil {
		desc = html.UnescapeString(string(m[1]))
		desc = strings.TrimSuffix(desc, "…")
		desc = strings.TrimSpace(desc)
	}

	var tags []string
	tagMatches := tagRe.FindAllSubmatch(body, -1)
	seen := map[string]bool{}
	for _, m := range tagMatches {
		name := strings.TrimSpace(string(m[2]))
		if name != "" && !seen[name] {
			seen[name] = true
			tags = append(tags, name)
		}
	}

	return models.Scene{
		ID:          ls.id,
		SiteID:      s.Config.SiteID,
		StudioURL:   s.Config.SiteBase,
		Title:       ls.title,
		URL:         ls.url,
		Date:        date,
		Description: desc,
		Duration:    ls.duration,
		Performers:  ls.performers,
		Tags:        tags,
		Thumbnail:   ls.thumb,
		Studio:      s.Config.StudioName,
		ScrapedAt:   time.Now().UTC(),
	}, nil
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}
