package missax

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

const defaultSiteBase = "https://www.missax.com"

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

func (s *Scraper) ID() string { return "missax" }

func (s *Scraper) Patterns() []string {
	return []string{"missax.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?missax\.com`)

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

type listEntry struct {
	id         string
	title      string
	url        string
	thumbnail  string
	performers []string
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	work := make(chan listEntry, opts.Workers)
	var wg sync.WaitGroup

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range work {
				if opts.Delay > 0 {
					select {
					case <-time.After(opts.Delay):
					case <-ctx.Done():
						return
					}
				}
				scene, err := s.fetchDetail(ctx, studioURL, entry)
				select {
				case out <- scraper.SceneResult{Scene: scene, Err: err}:
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
		}

		entries, err := s.fetchPage(ctx, page)
		if err != nil {
			select {
			case out <- scraper.SceneResult{Err: fmt.Errorf("page %d: %w", page, err)}:
			case <-ctx.Done():
			}
			break
		}

		if len(entries) == 0 {
			break
		}

		if page == 1 {
			select {
			case out <- scraper.SceneResult{Total: estimateTotal(len(entries), page)}:
			case <-ctx.Done():
				break
			}
		}

		cancelled := false
		hitKnown := false
		for _, e := range entries {
			if len(opts.KnownIDs) > 0 && opts.KnownIDs[e.id] {
				hitKnown = true
				break
			}
			select {
			case work <- e:
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
				case out <- scraper.SceneResult{StoppedEarly: true}:
				case <-ctx.Done():
				}
			}
			break
		}
	}

	close(work)
	wg.Wait()
}

// estimateTotal is a rough guess since the site doesn't expose total count.
// We know ~54 pages x 12 items from analysis.
func estimateTotal(firstPageCount, _ int) int {
	return firstPageCount * 54
}

var (
	sceneBlockRe = regexp.MustCompile(`(?s)photo-thumb video-thumb">(.*?)<!-- end photo-thumb -->`)
	sceneLinkRe  = regexp.MustCompile(`href="(https?://[^"]*?/tour/trailers/[^"]+\.html)"`)
	sceneTitleRe = regexp.MustCompile(`title="([^"]+)"`)
	sceneIDRe    = regexp.MustCompile(`contentthumbs/\d+/\d+/(\d+)-`)
	thumbRe      = regexp.MustCompile(`src0_1x="([^"]+)"`)
	performersRe = regexp.MustCompile(`(?s)class="model-name">\s*(.*?)\s*</p>`)
	performerRe  = regexp.MustCompile(`>([^<]+)</a>`)
)

func (s *Scraper) fetchPage(ctx context.Context, page int) ([]listEntry, error) {
	pageURL := fmt.Sprintf("%s/tour/categories/movies_%d_d.html", s.siteBase, page)
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: pageURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	blocks := sceneBlockRe.FindAllSubmatch(body, -1)
	entries := make([]listEntry, 0, len(blocks))

	for _, block := range blocks {
		content := block[1]

		linkM := sceneLinkRe.FindSubmatch(content)
		if linkM == nil {
			continue
		}

		entry := listEntry{
			url: string(linkM[1]),
		}

		if m := sceneTitleRe.FindSubmatch(content); m != nil {
			entry.title = html.UnescapeString(string(m[1]))
		}
		if m := sceneIDRe.FindSubmatch(content); m != nil {
			entry.id = string(m[1])
		}
		if m := thumbRe.FindSubmatch(content); m != nil {
			entry.thumbnail = string(m[1])
		}
		if m := performersRe.FindSubmatch(content); m != nil {
			for _, pm := range performerRe.FindAllSubmatch(m[1], -1) {
				entry.performers = append(entry.performers, strings.TrimSpace(string(pm[1])))
			}
		}

		if entry.id == "" {
			entry.id = slugFromURL(entry.url)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

var (
	runtimeRe    = regexp.MustCompile(`Runtime:\s*(\d+:\d+(?::\d+)?)`)
	addedRe      = regexp.MustCompile(`Added:\s*(\d{2}/\d{2}/\d{4})`)
	categoriesRe = regexp.MustCompile(`(?s)Categories:\s*(.*?)</p>`)
	categoryRe   = regexp.MustCompile(`>([^<]+)</a>`)
	videoSrcRe   = regexp.MustCompile(`<video\s+src="([^"]+)"`)
	descriptionRe = regexp.MustCompile(`(?s)Video Description:\s*</p>\s*<p[^>]*>\s*(.*?)\s*</p>`)
	htmlTagRe     = regexp.MustCompile(`<[^>]+>`)
)

func (s *Scraper) fetchDetail(ctx context.Context, studioURL string, entry listEntry) (models.Scene, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: entry.url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return models.Scene{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return models.Scene{}, err
	}

	now := time.Now().UTC()

	scene := models.Scene{
		ID:         entry.id,
		SiteID:     "missax",
		StudioURL:  studioURL,
		Title:      entry.title,
		URL:        entry.url,
		Thumbnail:  entry.thumbnail,
		Performers: entry.performers,
		Studio:     "MissaX",
		ScrapedAt:  now,
	}

	if m := runtimeRe.FindSubmatch(body); m != nil {
		scene.Duration = parseDuration(string(m[1]))
	}

	if m := addedRe.FindSubmatch(body); m != nil {
		scene.Date = parseDate(string(m[1]))
	}

	if m := categoriesRe.FindSubmatch(body); m != nil {
		for _, cm := range categoryRe.FindAllSubmatch(m[1], -1) {
			scene.Tags = append(scene.Tags, strings.TrimSpace(string(cm[1])))
		}
	}

	if m := descriptionRe.FindSubmatch(body); m != nil {
		desc := htmlTagRe.ReplaceAll(m[1], []byte("\n"))
		desc = []byte(html.UnescapeString(string(desc)))
		desc = []byte(strings.TrimSpace(string(desc)))
		scene.Description = collapseWhitespace(string(desc))
	}

	if m := videoSrcRe.FindSubmatch(body); m != nil {
		src := string(m[1])
		if !strings.HasPrefix(src, "http") {
			src = s.siteBase + src
		}
		scene.Preview = src
	}

	// OG image is often higher quality than listing thumbnail
	if m := regexp.MustCompile(`og:image"\s+content="([^"]+)"`).FindSubmatch(body); m != nil {
		scene.Thumbnail = string(m[1])
	}

	return scene, nil
}

func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		total = total*60 + n
	}
	return total
}

func parseDate(s string) time.Time {
	t, err := time.Parse("01/02/2006", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func slugFromURL(u string) string {
	u = strings.TrimSuffix(u, ".html")
	if idx := strings.LastIndex(u, "/"); idx >= 0 {
		u = u[idx+1:]
	}
	return u
}

var multiNewlineRe = regexp.MustCompile(`\n{3,}`)

func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	s = strings.Join(lines, "\n")
	return multiNewlineRe.ReplaceAllString(s, "\n\n")
}

func readBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}
