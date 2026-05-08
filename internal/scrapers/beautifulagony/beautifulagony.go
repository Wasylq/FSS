package beautifulagony

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteBase     = "https://beautifulagony.com"
	pageSize     = 20
	defaultDelay = 500 * time.Millisecond
)

type Scraper struct {
	client *http.Client
}

func New() *Scraper {
	return &Scraper{client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return "beautifulagony" }

func (s *Scraper) Patterns() []string {
	return []string{"beautifulagony.com"}
}

var matchRe = regexp.MustCompile(`^https?://(?:www\.)?beautifulagony\.com`)

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	delay := opts.Delay
	if delay == 0 {
		delay = defaultDelay
	}

	totalSent := false
	for offset := 0; ; offset += pageSize {
		if ctx.Err() != nil {
			return
		}

		if offset > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		pageURL := fmt.Sprintf("%s/public/main.php?page=view&mode=all&offset=%d", siteBase, offset)
		scenes, err := s.fetchPage(ctx, pageURL, studioURL)
		if err != nil {
			select {
			case out <- scraper.Error(fmt.Errorf("offset %d: %w", offset, err)):
			case <-ctx.Done():
			}
			return
		}

		if len(scenes) == 0 {
			return
		}

		if !totalSent {
			select {
			case out <- scraper.Progress(0):
			case <-ctx.Done():
				return
			}
			totalSent = true
		}

		for _, scene := range scenes {
			if opts.KnownIDs[scene.ID] {
				select {
				case out <- scraper.StoppedEarly():
				case <-ctx.Done():
				}
				return
			}
			select {
			case out <- scraper.Scene(scene):
			case <-ctx.Done():
				return
			}
		}
	}
}

var (
	idRe    = regexp.MustCompile(`class="agonyid">#(\d+)</font>`)
	dateRe  = regexp.MustCompile(`class="thumb_release_date_div">\s*(\d{2}\s+\w+\s+\d{4})`)
	thumbRe = regexp.MustCompile(`src="(https://bcdn\.beautifulagony\.com/[^"]+)"`)
	hdRe    = regexp.MustCompile(`class="hdtext"`)
)

func (s *Scraper) fetchPage(ctx context.Context, pageURL, studioURL string) ([]models.Scene, error) {
	resp, err := httpx.Do(ctx, s.client, httpx.Request{
		URL:     pageURL,
		Headers: map[string]string{"User-Agent": httpx.UserAgentFirefox},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseListingPage(body, studioURL), nil
}

func parseListingPage(body []byte, studioURL string) []models.Scene {
	blocks := splitVidBlocks(body)
	now := time.Now().UTC()

	var scenes []models.Scene
	for _, block := range blocks {
		im := idRe.FindSubmatch(block)
		if im == nil {
			continue
		}
		agonyID := string(im[1])

		var date time.Time
		if dm := dateRe.FindSubmatch(block); dm != nil {
			date, _ = time.Parse("02 Jan 2006", strings.TrimSpace(string(dm[1])))
		}

		var thumbnail string
		if tm := thumbRe.FindSubmatch(block); tm != nil {
			thumbnail = string(tm[1])
		}

		var resolution string
		if hdRe.Match(block) {
			resolution = "HD"
		}

		scenes = append(scenes, models.Scene{
			ID:         agonyID,
			SiteID:     "beautifulagony",
			StudioURL:  studioURL,
			Title:      "Agony #" + agonyID,
			URL:        siteBase + "/public/main.php?page=view&mode=all",
			Date:       date.UTC(),
			Thumbnail:  thumbnail,
			Studio:     "Beautiful Agony",
			Resolution: resolution,
			ScrapedAt:  now,
		})
	}

	return scenes
}

var vidBlockRe = regexp.MustCompile(`(?s)<div class="vid">`)

func splitVidBlocks(body []byte) [][]byte {
	locs := vidBlockRe.FindAllIndex(body, -1)
	if len(locs) == 0 {
		return nil
	}

	var blocks [][]byte
	for i, loc := range locs {
		start := loc[0]
		var end int
		if i+1 < len(locs) {
			end = locs[i+1][0]
		} else {
			end = len(body)
		}
		blocks = append(blocks, body[start:end])
	}
	return blocks
}
