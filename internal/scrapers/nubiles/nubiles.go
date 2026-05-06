package nubiles

import (
	"context"
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

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() {
	scraper.Register(New())
}

func (s *Scraper) ID() string { return "nubiles" }

func (s *Scraper) Patterns() []string {
	return []string{
		"nubiles-porn.com",
		"nubiles-porn.com/model/profile/{id}/{slug}",
		"nubiles-porn.com/video/category/{id}/{slug}",
		"nubiles.net",
		"momsteachsex.com",
		"stepsiblingscaught.com",
		"myfamilypies.com",
		"princesscum.com",
		"badteenspunished.com",
		"nubileset.com",
		"petitehdporn.com",
		"cumswappingsis.com",
		"familyswap.xxx",
		"caughtmycoach.com",
		"detentiongirls.com",
		"realitysis.com",
		"shesbreedingmaterial.com",
		"youngermommy.com",
		"petiteballerinasfucked.com",
		"nubiles-casting.com",
		"anilos.com",
		"brattymilf.com",
		"brattysis.com",
		"daddyslilangel.com",
		"datingmystepson.com",
		"girlsonlyporn.com",
		"hotcrazymess.com",
		"imnotyourmommy.com",
		"momlover.com",
		"momsboytoy.com",
		"momsfamilysecrets.com",
		"momstight.com",
		"momswapped.com",
		"momwantscreampie.com",
		"momwantstobreed.com",
		"nfbusty.com",
		"nubilefilms.com",
		"teacherfucksteens.com",
		"thatsitcomshow.com",
		"bountyhunterporn.com",
		"cheatingmommy.com",
		"cheatingsis.com",
		"deeplush.com",
		"doublepies.com",
		"driverxxx.com",
		"glowingdesire.com",
		"lilsis.com",
		"milfcoach.com",
		"nubilesunscripted.com",
		"smashed.xxx",
		"thepovgod.com",
	}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?(?:` +
	`nubiles-porn\.com|nubiles\.net|nubiles-casting\.com|` +
	`momsteachsex\.com|stepsiblingscaught\.com|myfamilypies\.com|` +
	`princesscum\.com|badteenspunished\.com|nubileset\.com|` +
	`petitehdporn\.com|cumswappingsis\.com|familyswap\.xxx|` +
	`caughtmycoach\.com|detentiongirls\.com|realitysis\.com|` +
	`shesbreedingmaterial\.com|youngermommy\.com|` +
	`petiteballerinasfucked\.com|` +
	`anilos\.com|brattymilf\.com|brattysis\.com|` +
	`daddyslilangel\.com|datingmystepson\.com|girlsonlyporn\.com|` +
	`hotcrazymess\.com|imnotyourmommy\.com|momlover\.com|` +
	`momsboytoy\.com|momsfamilysecrets\.com|momstight\.com|` +
	`momswapped\.com|momwantscreampie\.com|momwantstobreed\.com|` +
	`nfbusty\.com|nubilefilms\.com|teacherfucksteens\.com|` +
	`thatsitcomshow\.com|bountyhunterporn\.com|cheatingmommy\.com|` +
	`cheatingsis\.com|deeplush\.com|doublepies\.com|driverxxx\.com|` +
	`glowingdesire\.com|lilsis\.com|milfcoach\.com|` +
	`nubilesunscripted\.com|smashed\.xxx|thepovgod\.com)`)

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

type filterMode int

const (
	filterAll filterMode = iota
	filterModel
	filterCategory
)

type filter struct {
	mode filterMode
	id   string
	slug string
}

var (
	modelRe    = regexp.MustCompile(`/model/profile/(\d+)/([^/?#]+)`)
	categoryRe = regexp.MustCompile(`/video/category/(\d+)/([^/?#]+)`)
)

func parseFilter(rawURL string) filter {
	if m := modelRe.FindStringSubmatch(rawURL); m != nil {
		return filter{mode: filterModel, id: m[1], slug: m[2]}
	}
	if m := categoryRe.FindStringSubmatch(rawURL); m != nil {
		return filter{mode: filterCategory, id: m[1], slug: m[2]}
	}
	return filter{mode: filterAll}
}

var baseURLRe = regexp.MustCompile(`^(https?://[^/]+)`)

func baseURL(rawURL string) string {
	m := baseURLRe.FindString(rawURL)
	if m == "" {
		return rawURL
	}
	return m
}

func listingURL(base string, f filter, offset int) string {
	switch f.mode {
	case filterModel:
		if offset == 0 {
			return fmt.Sprintf("%s/video/model/%s/%s", base, f.id, f.slug)
		}
		return fmt.Sprintf("%s/video/model/%s/%s/%d", base, f.id, f.slug, offset)
	case filterCategory:
		if offset == 0 {
			return fmt.Sprintf("%s/video/category/%s/%s", base, f.id, f.slug)
		}
		return fmt.Sprintf("%s/video/category/%s/%s/%d", base, f.id, f.slug, offset)
	default:
		if offset == 0 {
			return fmt.Sprintf("%s/video/gallery", base)
		}
		return fmt.Sprintf("%s/video/gallery/%d", base, offset)
	}
}

type listEntry struct {
	id         string
	title      string
	url        string
	thumbnail  string
	preview    string
	performers []string
	subSite    string
	date       string
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	f := parseFilter(studioURL)
	base := baseURL(studioURL)

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

	sentTotal := false
	for offset := 0; ; offset += 12 {
		if ctx.Err() != nil {
			break
		}
		if offset > 0 && opts.Delay > 0 {
			select {
			case <-time.After(opts.Delay):
			case <-ctx.Done():
				break
			}
			if ctx.Err() != nil {
				break
			}
		}

		pageURL := listingURL(base, f, offset)
		entries, totalPages, err := s.fetchListing(ctx, pageURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("offset %d: %w", offset, err)):
			case <-ctx.Done():
			}
			break
		}

		if len(entries) == 0 {
			break
		}

		if !sentTotal && totalPages > 0 {
			sentTotal = true
			select {
			case out <- scraper.Progress(totalPages * 12):
			case <-ctx.Done():
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
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
			}
			break
		}

		currentPage := offset/12 + 1
		if currentPage >= totalPages {
			break
		}
	}

	close(work)
	wg.Wait()
}

var (
	gridItemRe   = regexp.MustCompile(`(?s)<figure\b[^>]*>.*?</figcaption>\s*</figure>`)
	watchLinkRe  = regexp.MustCompile(`/video/watch/(\d+)/([^"]+)`)
	imgSrcsetRe  = regexp.MustCompile(`data-srcset="([^"]+)"`)
	imgSrcRe     = regexp.MustCompile(`data-src="([^"]+)"`)
	previewSrcRe = regexp.MustCompile(`data-preview-src="([^"]+)"`)
	titleRe      = regexp.MustCompile(`(?s)<span class="title">\s*<a[^>]*>\s*(.*?)\s*</a>`)
	modelLinkRe  = regexp.MustCompile(`class="model"[^>]*>([^<]+)</a>`)
	siteLinkRe   = regexp.MustCompile(`class="site-link"[^>]*>([^<]+)</a>`)
	dateRe       = regexp.MustCompile(`class="date">([^<]+)</span>`)
	paginationRe = regexp.MustCompile(`(\d+)\s+of\s+(\d+)`)
)

func (s *Scraper) fetchListing(ctx context.Context, pageURL string) ([]listEntry, int, error) {
	body, err := s.fetchHTML(ctx, pageURL)
	if err != nil {
		return nil, 0, err
	}

	// Parse total pages from the LAST "N of M" match (the main grid, not featured)
	totalPages := 0
	if matches := paginationRe.FindAllSubmatch(body, -1); len(matches) > 0 {
		last := matches[len(matches)-1]
		totalPages, _ = strconv.Atoi(string(last[2]))
	}

	figures := gridItemRe.FindAll(body, -1)
	entries := make([]listEntry, 0, len(figures))
	seen := make(map[string]bool)

	for _, fig := range figures {
		wm := watchLinkRe.FindSubmatch(fig)
		if wm == nil {
			continue
		}
		id := string(wm[1])
		if seen[id] {
			continue
		}
		seen[id] = true

		e := listEntry{id: id}

		e.url = fmt.Sprintf("/video/watch/%s/%s", id, string(wm[2]))

		if m := titleRe.FindSubmatch(fig); m != nil {
			e.title = strings.TrimSpace(html.UnescapeString(string(m[1])))
		}

		if m := imgSrcsetRe.FindSubmatch(fig); m != nil {
			e.thumbnail = bestSrcset(html.UnescapeString(string(m[1])))
		} else if m := imgSrcRe.FindSubmatch(fig); m != nil {
			e.thumbnail = html.UnescapeString(string(m[1]))
		}

		if m := previewSrcRe.FindSubmatch(fig); m != nil {
			e.preview = html.UnescapeString(string(m[1]))
		}

		for _, m := range modelLinkRe.FindAllSubmatch(fig, -1) {
			e.performers = append(e.performers, strings.TrimSpace(string(m[1])))
		}

		if m := siteLinkRe.FindSubmatch(fig); m != nil {
			e.subSite = strings.TrimSpace(string(m[1]))
		}

		if m := dateRe.FindSubmatch(fig); m != nil {
			e.date = strings.TrimSpace(string(m[1]))
		}

		entries = append(entries, e)
	}

	return entries, totalPages, nil
}

func bestSrcset(srcset string) string {
	var best string
	var bestW int
	for _, part := range strings.Split(srcset, ",") {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		url := fields[0]
		ws := strings.TrimSuffix(fields[1], "w")
		w, _ := strconv.Atoi(ws)
		if w > bestW {
			bestW = w
			best = url
		}
	}
	return best
}

var (
	metaDescRe     = regexp.MustCompile(`<meta\s+name="description"\s+content="([^"]*)"`)
	metaKeywordsRe = regexp.MustCompile(`<meta\s+name="keywords"\s+content="([^"]*)"`)
	ogImageRe      = regexp.MustCompile(`<meta\s+property="og:image"\s+content="([^"]*)"`)
)

func (s *Scraper) fetchDetail(ctx context.Context, studioURL string, entry listEntry) (models.Scene, error) {
	base := baseURL(studioURL)
	detailURL := base + entry.url

	body, err := s.fetchHTML(ctx, detailURL)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %s: %w", entry.id, err)
	}

	now := time.Now().UTC()
	scene := models.Scene{
		ID:         entry.id,
		SiteID:     "nubiles",
		StudioURL:  studioURL,
		Title:      entry.title,
		URL:        detailURL,
		Thumbnail:  entry.thumbnail,
		Preview:    entry.preview,
		Performers: entry.performers,
		Studio:     entry.subSite,
		ScrapedAt:  now,
	}

	if entry.date != "" {
		if t, err := time.Parse("Jan 2, 2006", entry.date); err == nil {
			scene.Date = t.UTC()
		}
	}

	if m := metaDescRe.FindSubmatch(body); m != nil {
		scene.Description = html.UnescapeString(string(m[1]))
	}

	if m := metaKeywordsRe.FindSubmatch(body); m != nil {
		for _, tag := range strings.Split(string(m[1]), ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				scene.Tags = append(scene.Tags, tag)
			}
		}
	}

	if m := ogImageRe.FindSubmatch(body); m != nil {
		scene.Thumbnail = html.UnescapeString(string(m[1]))
	}

	return scene, nil
}

func (s *Scraper) fetchHTML(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.ReadBody(resp.Body)
}
