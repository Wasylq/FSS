package shinybound

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

func siteCfg(t *testing.T, id string) SiteConfig {
	t.Helper()
	for _, c := range sites {
		if c.SiteID == id {
			return c
		}
	}
	t.Fatalf("site %q not registered", id)
	return SiteConfig{}
}

// card reproduces one listing tile. Only the listing carries the numeric id —
// a related-rail card has none — and only the listing carries the price.
func card(id int, slug, title, model, date string, withID, withPrice bool) string {
	idAttr := ""
	if withID {
		idAttr = fmt.Sprintf(` id="%d"`, id)
	}
	price := ""
	if withPrice {
		price = `<li id="cart-item-1" class="cart-item" data-price="$17.99" data-context="content-info">
      <a href="javascript:void(0);" class="cart-action add">$17.99</a></li>`
	}
	return fmt.Sprintf(`<div class=" videoBlock">
  <div class="videoPic">
    <a href="https://shinybound.com/updates/%s">
      <img%s src="https://shinybound.com/content/thumbs/%d/preview-09.jpg" alt="">
    </a>
  </div>
  <h3><a href="https://shinybound.com/updates/%s">%s</a></h3>
  <div class="modelName"><a href="https://shinybound.com/models/x">%s</a></div>
  <ul class="contentInfo">
    <li><i class="fa-solid fa-clock"></i>27:30</li>
    <li><i class="fas fa-camera" title="32 photos"></i> 32</li>
    <li><i class="fas fa-calendar"></i>%s</li>
    %s
  </ul>
</div>`, slug, idAttr, id, slug, title, model, date, price)
}

// detailPage puts the scene's own block before a "related videos" rail built
// from the same card markup — which is what the live pages do.
func detailPage(title, desc string, models, tags []string) string {
	var modelHTML, tagHTML strings.Builder
	for _, m := range models {
		fmt.Fprintf(&modelHTML, `<li><a href="/models/x"><i class="fa-solid fa-user"></i>%s</a></li>`, m)
	}
	for _, tg := range tags {
		fmt.Fprintf(&tagHTML, `<li><a href="/tags/x"><i class="fa-solid fa-tag"></i>%s</a></li>`, tg)
	}
	return fmt.Sprintf(`<html><body>
<h1>%s</h1>
<div class="description"><p>%s</p></div>
<div class="models"><h3>Models:</h3><ul>%s</ul></div>
<div class="tags"><h3>Tags:</h3><ul>%s</ul></div>
<div class="relatedVideos"><div class="subTitle">related videos</div>
  %s
</div>
</body></html>`, title, desc, modelHTML.String(), tagHTML.String(),
		card(999, "a-related-scene", "A RELATED SCENE", "Someone Else", "Jan 01, 2001", false, false))
}

type stubSite struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string]int
}

func (s *stubSite) hit(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits[p]++
}

func (s *stubSite) count(p string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[p]
}

func newSite(t *testing.T, pages, per int) *stubSite {
	t.Helper()
	site := &stubSite{hits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path)
		switch {
		case r.URL.Path == "/videos":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 {
				page = 1
			}
			if page > pages {
				_, _ = fmt.Fprint(w, `<html><body><div class="allVideos"></div></body></html>`)
				return
			}
			var sb strings.Builder
			sb.WriteString(`<html><body>`)
			for i := 0; i < per; i++ {
				id := 42769 - ((page-1)*per + i)
				sb.WriteString(card(id, fmt.Sprintf("scene-%d", id), fmt.Sprintf("Scene %d", id),
					"Claire Irons", "Jun 17, 2026", true, true))
			}
			sb.WriteString(`</body></html>`)
			_, _ = fmt.Fprint(w, sb.String())
		case strings.HasPrefix(r.URL.Path, "/updates/"):
			slug := strings.TrimPrefix(r.URL.Path, "/updates/")
			_, _ = fmt.Fprint(w, detailPage("Detail "+slug, "A real synopsis.",
				[]string{"Claire Irons", "Kendra James"}, []string{"Bdsm", "Leather"}))
		default:
			http.NotFound(w, r)
		}
	}))
	return site
}

func newTestScraper(cfg SiteConfig, srv *httptest.Server) *Scraper {
	s := New(cfg)
	s.Client = srv.Client()
	s.baseOverride = srv.URL
	return s
}

func collect(t *testing.T, s *Scraper, studioURL string, opts scraper.ListOpts) ([]models.Scene, []error, int, bool) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	total := 0
	stopped := false
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			errs = append(errs, res.Err)
		case scraper.KindTotal:
			total = res.Total
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	return scenes, errs, total, stopped
}

func TestWalksEveryListingPage(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(siteCfg(t, "shinybound"), site.Server)
	scenes, errs, total, _ := collect(t, s, "https://shinybound.com/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 12 || total != 12 {
		t.Fatalf("got %d scenes (total %d), want 12", len(scenes), total)
	}
}

// The listing card is almost the whole record; the detail adds the synopsis and
// the full tag list.
func TestCardAndDetailCombine(t *testing.T) {
	site := newSite(t, 1, 1)
	defer site.Close()

	s := newTestScraper(siteCfg(t, "shinybound"), site.Server)
	scenes, _, _, _ := collect(t, s, "https://shinybound.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	got := scenes[0]

	if got.ID != "42769" {
		t.Errorf("ID = %q, want the listing's numeric id", got.ID)
	}
	if got.Duration != 27*60+30 {
		t.Errorf("Duration = %d", got.Duration)
	}
	if got.Date.Format("2006-01-02") != "2026-06-17" {
		t.Errorf("Date = %v", got.Date)
	}
	if len(got.PriceHistory) != 1 || got.PriceHistory[0].Regular != 17.99 {
		t.Errorf("PriceHistory = %v", got.PriceHistory)
	}
	if got.Description != "A real synopsis." {
		t.Errorf("Description = %q", got.Description)
	}
	if strings.Join(got.Tags, ",") != "Bdsm,Leather" {
		t.Errorf("Tags = %v", got.Tags)
	}
	// The detail's model list wins over the card's single credit.
	if strings.Join(got.Performers, ",") != "Claire Irons,Kendra James" {
		t.Errorf("Performers = %v", got.Performers)
	}
}

// Every detail page ends with a "related videos" rail built from the same card
// markup as the listing, so nothing may pick up another scene's title, cast or
// tags from it.
func TestRelatedRailDoesNotLeakIntoTheScene(t *testing.T) {
	d := parseDetail(detailPage("Real Title", "A real synopsis.",
		[]string{"Claire Irons"}, []string{"Bdsm"}))

	if d.title != "Real Title" {
		t.Errorf("title = %q", d.title)
	}
	for _, p := range d.performers {
		if p == "Someone Else" {
			t.Error("the related rail's performer was credited to this scene")
		}
	}
	if strings.Join(d.tags, ",") != "Bdsm" {
		t.Errorf("tags = %v", d.tags)
	}
}

// Only the listing card carries the numeric id; the slug is the fallback.
func TestCardWithoutANumericIDFallsBackToTheSlug(t *testing.T) {
	cards := parseCards(card(999, "a-scene", "A Scene", "Someone", "Jan 01, 2001", false, false))
	if len(cards) != 1 {
		t.Fatalf("got %d cards", len(cards))
	}
	if cards[0].id != "a-scene" {
		t.Errorf("id = %q, want the slug", cards[0].id)
	}
}

func TestKnownIDStopsTheWalkEarly(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(siteCfg(t, "shinybound"), site.Server)
	scenes, _, _, stopped := collect(t, s, "https://shinybound.com/",
		scraper.ListOpts{Workers: 1, KnownIDs: map[string]bool{"42767": true}})

	if !stopped {
		t.Error("StoppedEarly was not reported")
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want the 2 ahead of the known id", len(scenes))
	}
}

func TestSingleSceneURLSkipsTheListing(t *testing.T) {
	site := newSite(t, 3, 4)
	defer site.Close()

	s := newTestScraper(siteCfg(t, "shinybound"), site.Server)
	scenes, errs, total, _ := collect(t, s,
		"https://shinybound.com/updates/silky-spandex", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	if site.count("/videos") != 0 {
		t.Error("a single-scene URL walked the listing")
	}
}

func TestEmptyFirstListingPageIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(siteCfg(t, "shinybound"), srv)
	scenes, errs, _, _ := collect(t, s, "https://shinybound.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("an unparseable listing reported no error")
	}
	if k := scraper.Classify(errs[0]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

func TestEachSiteMatchesOnlyItsOwnHost(t *testing.T) {
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}
	for _, cfg := range sites {
		s := New(cfg)
		if !s.MatchesURL("https://" + cfg.Domain + "/videos") {
			t.Errorf("%s does not match its own host", cfg.SiteID)
		}
		for _, other := range sites {
			if other.Domain != cfg.Domain && s.MatchesURL("https://"+other.Domain+"/videos") {
				t.Errorf("%s also matches %s", cfg.SiteID, other.Domain)
			}
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t, 1, 3)
	defer site.Close()

	s := newTestScraper(siteCfg(t, "shinybound"), site.Server)
	scenes, _, _, _ := collect(t, s, "https://shinybound.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := newSite(t, 40, 10)
	defer site.Close()

	s := newTestScraper(siteCfg(t, "shinybound"), site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://shinybound.com/", scraper.ListOpts{Workers: 3, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
