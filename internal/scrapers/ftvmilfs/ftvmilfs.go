package ftvmilfs

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

const (
	defaultBase = "https://ftvmilfs.com"
	siteID      = "ftvmilfs"
)

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?ftvmilfs\.com`)

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

func (s *Scraper) ID() string { return siteID }
func (s *Scraper) Patterns() []string {
	return []string{
		"ftvmilfs.com",
		"ftvmilfs.com/modelssfw.html",
		"ftvmilfs.com/updates.html",
	}
}
func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

type listingEntry struct {
	id       string
	name     string
	date     time.Time
	duration int
	tags     []string
	thumb    string
	desc     string
}

var (
	containerRe = regexp.MustCompile(`(?s)<div class="ModelContainer">(.*?)</div><!-- ModelContainer -->`)
	listNameRe  = regexp.MustCompile(`class="ModelName"><h2>(.*?)</h2>`)
	listDateRe  = regexp.MustCompile(`class="UpdateDate"><h3>(.*?)</h3>`)
	listDurRe   = regexp.MustCompile(`(?s)class="VideoTime">.*?<h3>(\d+)\s*mins?</h3>`)
	listTagRe   = regexp.MustCompile(`updatesCategories/\w+\.png"\s*title="([^"]*)"`)
	listThumbRe = regexp.MustCompile(`ModelPhotoWide"\s*src="([^"]*tour-(\d+)\.jpg)"`)
	listBioRe   = regexp.MustCompile(`(?s)class="Bio">\s*<p>(.*?)</p>`)
	detTitleRe  = regexp.MustCompile(`<title>(.*?) on FTVMilfs\.com Released (.*?)!</title>`)
	dataTitleRe = regexp.MustCompile(`data-title="[^"]*<span>([^<]*)</span>[^"]*Age:[^"]*<span>(\d+)</span>[^"]*Figure:[^"]*<span>([^<]*)</span>[^"]*Release date:[^"]*<span>([^<]*)</span>"`)
	detBioRe    = regexp.MustCompile(`(?s)id="Bio">\s*<p>(.*?)</p>`)
	detThumbRe  = regexp.MustCompile(`id="Magazine"\s*src="([^"]*)"`)
	detHeightRe = regexp.MustCompile(`Height:</b>\s*([^<]+)`)
)

func parseDate(raw string) time.Time {
	raw = strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if t, err := time.Parse("Jan 2, 2006", raw); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func parseListingPage(body []byte) []listingEntry {
	matches := containerRe.FindAllSubmatch(body, -1)
	entries := make([]listingEntry, 0, len(matches))

	for _, m := range matches {
		block := string(m[1])
		var e listingEntry

		if nm := listNameRe.FindStringSubmatch(block); nm != nil {
			e.name = strings.TrimSpace(html.UnescapeString(nm[1]))
		}

		if dm := listDateRe.FindStringSubmatch(block); dm != nil {
			e.date = parseDate(dm[1])
		}

		if dur := listDurRe.FindStringSubmatch(block); dur != nil {
			mins, _ := strconv.Atoi(dur[1])
			e.duration = mins * 60
		}

		for _, tm := range listTagRe.FindAllStringSubmatch(block, -1) {
			parts := strings.SplitN(tm[1], " - ", 2)
			tag := strings.TrimSpace(parts[0])
			if tag != "" {
				e.tags = append(e.tags, tag)
			}
		}

		if th := listThumbRe.FindStringSubmatch(block); th != nil {
			e.thumb = th[1]
			e.id = th[2]
		}

		if bio := listBioRe.FindStringSubmatch(block); bio != nil {
			e.desc = strings.TrimSpace(html.UnescapeString(bio[1]))
		}

		if e.id == "" || e.name == "" {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

type detailData struct {
	name   string
	age    int
	height string
	figure string
	date   time.Time
	desc   string
	thumb  string
}

func parseDetailPage(body []byte) detailData {
	var d detailData
	page := string(body)

	if m := dataTitleRe.FindStringSubmatch(page); m != nil {
		d.name = strings.TrimSpace(m[1])
		d.age, _ = strconv.Atoi(m[2])
		d.figure = strings.TrimSpace(m[3])
		d.date = parseDate(m[4])
	} else if m := detTitleRe.FindStringSubmatch(page); m != nil {
		d.name = strings.TrimSpace(m[1])
		d.date = parseDate(m[2])
	}

	if m := detHeightRe.FindStringSubmatch(page); m != nil {
		d.height = strings.TrimSpace(html.UnescapeString(m[1]))
	}

	if m := detBioRe.FindStringSubmatch(page); m != nil {
		d.desc = strings.TrimSpace(html.UnescapeString(m[1]))
	}

	if m := detThumbRe.FindStringSubmatch(page); m != nil {
		d.thumb = m[1]
	}

	return d
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	workers := opts.Workers
	if workers <= 0 {
		workers = 4
	}

	body, err := s.fetchPage(ctx, s.base+"/updates.html")
	if err != nil {
		select {
		case out <- scraper.Error(fmt.Errorf("listing: %w", err)):
		case <-ctx.Done():
		}
		return
	}

	entries := parseListingPage(body)
	if len(entries) == 0 {
		select {
		case out <- scraper.Error(fmt.Errorf("no scenes found on listing page")):
		case <-ctx.Done():
		}
		return
	}

	enrichment := make(map[string]listingEntry, len(entries))
	for _, e := range entries {
		enrichment[e.id] = e
	}

	latestID, _ := strconv.Atoi(entries[0].id)
	if latestID <= 0 {
		return
	}

	select {
	case out <- scraper.Progress(latestID):
	case <-ctx.Done():
		return
	}

	work := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				scene, err := s.fetchScene(ctx, id, studioURL, enrichment, opts.Delay)
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

	go func() {
		defer close(work)
		for id := latestID; id >= 1; id-- {
			idStr := strconv.Itoa(id)
			if opts.KnownIDs[idStr] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case work <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
}

func (s *Scraper) fetchScene(ctx context.Context, id int, studioURL string, enrichment map[string]listingEntry, delay time.Duration) (models.Scene, error) {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return models.Scene{}, ctx.Err()
		}
	}

	idStr := strconv.Itoa(id)
	url := fmt.Sprintf("%s/update/x-%d.html", s.base, id)

	body, err := s.fetchPage(ctx, url)
	if err != nil {
		return models.Scene{}, fmt.Errorf("detail %d: %w", id, err)
	}

	detail := parseDetailPage(body)
	if detail.name == "" {
		return models.Scene{}, fmt.Errorf("detail %d: empty page", id)
	}

	now := time.Now().UTC()
	scene := models.Scene{
		ID:          idStr,
		SiteID:      siteID,
		StudioURL:   studioURL,
		URL:         url,
		Title:       detail.name,
		Date:        detail.date,
		Description: detail.desc,
		Thumbnail:   detail.thumb,
		Performers:  []string{detail.name},
		Studio:      "FTV MILFs",
		ScrapedAt:   now,
	}

	if le, ok := enrichment[idStr]; ok {
		scene.Duration = le.duration
		scene.Tags = le.tags
		if scene.Thumbnail == "" {
			scene.Thumbnail = le.thumb
		}
	}

	return scene, nil
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]byte, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL: url,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentChrome,
		},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}
