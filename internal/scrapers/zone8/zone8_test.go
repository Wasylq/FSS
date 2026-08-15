package zone8

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// ---- fixtures ----

// elPremium is the recent pay-per-view layout: an h3 carrying the title and
// model, and a tag list holding the runtime and a price in pounds.
func elPremium(id, title, model, desc string, minutes int, price string) string {
	return fmt.Sprintf(`<html><body>
<div class="shoot-listing premium-shoot">
  <video data-poster="/public/updates/vs-%s.jpg" preload="none" title="%s %s">
    <source src="/public/movies/%s/%s_trl.mp4">
  </video>
  <table class="large-update"><tr><td>
    <h3 class="title">%s - <a href="/model-%s" class="more">%s</a></h3>
    <p class="description">%s</p>
    <ul class="premium-descriptive-tags"><li>%d minute video</li><li>&pound;%s</li><li>Available to stream</li></ul>
  </td></tr></table>
  <div class="shoot-images shoot-images-group-video">
    <div class="shoot-image"><picture><img src="/public/updates/newsize/%s_1.jpg" alt="%s"/></picture></div>
  </div>
</div></body></html>`, id, model, title, id, id, title, strings.ToLower(strings.ReplaceAll(model, " ", "-")), model, desc, minutes, price, id, title)
}

// elClassic is the older layout that covers most of the catalogue: one h2 with
// the date, title and models, a bare description div, and a tag list. Note the
// anchor writes `class` BEFORE `href` here and after it on the premium page.
func elClassic(date, title, desc string, models, tags []string) string {
	var links []string
	for _, m := range models {
		links = append(links, fmt.Sprintf(`<a class="more" href="/model-%s">%s</a>`,
			strings.ToLower(strings.ReplaceAll(m, " ", "-")), m))
	}
	var tagLinks []string
	for _, tg := range tags {
		tagLinks = append(tagLinks, fmt.Sprintf(`<a class="more" href="/?tag=%s" style="font-style: italic;">%s</a>`,
			strings.ReplaceAll(tg, " ", "+"), tg))
	}
	return fmt.Sprintf(`<html><body>
<h2 style="font-size: 13px;">%s - %s - %s </h2>
</div></div><div class="shoot-header-row"></div></div>
<div style="margin-top: 4px; padding-left: 6%%;">%s</div>
<br/> <b>Tags: </b> %s<br/>
</body></html>`, date, title, strings.Join(links, ", "), desc, strings.Join(tagLinks, ", "))
}

// fymProfile is the model-profile layout: four nested spans in one h1, with the
// sport span *inside* the title span.
func fymProfile(name, sport, title, desc, published, id string) string {
	return fmt.Sprintf(`<html><body><div id="centralContent">
<h1><span class="model-profile-site">Fit Young Men:</span> <span class="model-profile-name">%s</span> <span class="model-profile-sport-relationship"><span class="model-profile-sport">- %s</span> - %s </span> </h1>
<div class="model-profile-headline"><div class="model-profile-stat-box">
  <div class="stats">Age: 19</div><div class="published">Published: %s</div>
</div></div>
<img src="/mb0%s-fit-young-men.jpg" width="200"/>
<div class="model-profile-model-description">%s</div>
</div></body></html>`, name, sport, title, published, id, desc)
}

// fymBonus is the third layout, used by the bonus-video sections. A `model-`
// URL for one of these redirects here.
func fymBonus(id, name, title, sport, published, desc, price string) string {
	return fmt.Sprintf(`<html><body><div id="centralContent" class="nearlynude">
<div class="listing-wrapper"><div class="set-image">
  <video poster="/shoots/0%s.webp" title="%s"><source src="/members/movies/trailers/0%s_trl.mp4"></video>
  <div class="set-details">
    <div class="title">%s - %s
      <div class="date mobile-only">Published %s</div>
    </div>
    <div class="age">18yo</div>
    <div class="sport">%s</div>
    <div class="date non-mobile">Published %s</div>
    <div class="description"><span>This video is free for memberships that have completed 90 days, or <a href="/member-login">log in to purchase</a>.</span> Previous members, please <a href="/join">rejoin</a> to buy this video for &pound;%s with access until your membership expires. <br/><br/> %s</div>
  </div>
  <div class="highlight-indicators"> </div>
</div></div></div></body></html>`, id, title, id, name, title, published, sport, published, price, desc)
}

func sitemapXML(base string, paths ...string) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?><urlset>`)
	for _, p := range paths {
		fmt.Fprintf(&sb, "<url><loc>%s%s</loc></url>", base, p)
	}
	sb.WriteString(`</urlset>`)
	return sb.String()
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

func newScraper(cfg SiteConfig, srv *httptest.Server) *Scraper {
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

// ---- English Lads ----

const elLiveHost = "https://www.englishlads.com"

func englishLadsSite(t *testing.T) *stubSite {
	t.Helper()
	site := &stubSite{hits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path)
		switch {
		case r.URL.Path == "/sitemap.xml":
			_, _ = fmt.Fprint(w, sitemapXML(elLiveHost,
				"/", "/models", "/join",
				"/video-2025-05-16-02979-N-recent-premium-update",
				"/video-2006-02-24-00676-N-ivo-fucks-karl",
				// Photo sets share the URL space and are not scenes.
				"/photo-2004-02-15-00070-Y-ben-and-mark-finally-get-together",
				"/model-henry-kane",
			))
		case strings.HasPrefix(r.URL.Path, "/video-2025"):
			_, _ = fmt.Fprint(w, elPremium("02979", "Recent Premium Update", "Henry Kane",
				"A premium description.", 16, "24.00"))
		case strings.HasPrefix(r.URL.Path, "/video-2006"):
			_, _ = fmt.Fprint(w, elClassic("24th Feb 2006", "Ivo fucks Karl", "A classic description.",
				[]string{"Ivo Simpson", "Karl Hammond"}, []string{"rimming", "kissing"}))
		default:
			http.NotFound(w, r)
		}
	}))
	return site
}

// Photo sets, model pages and site pages share the URL space with videos, and
// only `/video-` entries are scenes.
func TestEnglishLadsSitemapSelectsOnlyVideos(t *testing.T) {
	site := englishLadsSite(t)
	defer site.Close()

	s := newScraper(siteCfg(t, "englishlads"), site.Server)
	scenes, errs, total, _ := collect(t, s, "https://www.englishlads.com/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 2 || total != 2 {
		t.Fatalf("got %d scenes (total %d), want 2", len(scenes), total)
	}
	if site.count("/photo-2004-02-15-00070-Y-ben-and-mark-finally-get-together") != 0 {
		t.Error("a photo set was fetched as a scene")
	}
	if site.count("/model-henry-kane") != 0 {
		t.Error("a model page was fetched as a scene")
	}
}

// Both layouts have to parse. The recent one carries a price and a runtime;
// the older one — most of the catalogue — carries the date and the tags.
func TestEnglishLadsParsesBothLayouts(t *testing.T) {
	site := englishLadsSite(t)
	defer site.Close()

	s := newScraper(siteCfg(t, "englishlads"), site.Server)
	scenes, _, _, _ := collect(t, s, "https://www.englishlads.com/", scraper.ListOpts{Workers: 1})

	byID := map[string]models.Scene{}
	for _, sc := range scenes {
		byID[sc.ID] = sc
	}

	premium := byID["02979"]
	if premium.Title != "Recent Premium Update" {
		t.Errorf("premium Title = %q — the model anchor should be trimmed off", premium.Title)
	}
	if len(premium.Performers) != 1 || premium.Performers[0] != "Henry Kane" {
		t.Errorf("premium Performers = %v", premium.Performers)
	}
	if premium.Duration != 16*60 {
		t.Errorf("premium Duration = %d", premium.Duration)
	}
	if len(premium.PriceHistory) != 1 || premium.PriceHistory[0].Regular != 24 {
		t.Errorf("premium PriceHistory = %v", premium.PriceHistory)
	}
	// The date comes from the URL on this site, not the page.
	if premium.Date.Format("2006-01-02") != "2025-05-16" {
		t.Errorf("premium Date = %v", premium.Date)
	}

	classic := byID["00676"]
	if classic.Title != "Ivo fucks Karl" {
		t.Errorf("classic Title = %q — the date prefix and model suffix should both go", classic.Title)
	}
	if strings.Join(classic.Performers, ",") != "Ivo Simpson,Karl Hammond" {
		t.Errorf("classic Performers = %v", classic.Performers)
	}
	if strings.Join(classic.Tags, ",") != "rimming,kissing" {
		t.Errorf("classic Tags = %v", classic.Tags)
	}
	if classic.Description != "A classic description." {
		t.Errorf("classic Description = %q", classic.Description)
	}
	// Membership-only updates quote no price; recording a zero would make them
	// look free.
	if len(classic.PriceHistory) != 0 {
		t.Errorf("classic PriceHistory = %v, want none", classic.PriceHistory)
	}
}

// The model anchor writes its attributes in a different order on each layout,
// which is exactly the sort of thing a tighter regex silently misses — it cost
// 1462 scenes their performers.
func TestEnglishLadsModelAnchorAttributeOrder(t *testing.T) {
	premium := parseEnglishLads(elPremium("1", "T", "Henry Kane", "d", 5, "9.00"))
	if len(premium.performers) != 1 || premium.performers[0] != "Henry Kane" {
		t.Errorf("premium performers = %v", premium.performers)
	}
	classic := parseEnglishLads(elClassic("1st Jan 2010", "T", "d", []string{"Ivo Simpson"}, nil))
	if len(classic.performers) != 1 || classic.performers[0] != "Ivo Simpson" {
		t.Errorf("classic performers = %v", classic.performers)
	}
}

// ---- Fit Young Men ----

const fymLiveHost = "https://www.fityoungmen.com"

func fitYoungMenSite(t *testing.T) *stubSite {
	t.Helper()
	site := &stubSite{hits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path)
		switch {
		case r.URL.Path == "/sitemap.xml":
			_, _ = fmt.Fprint(w, sitemapXML(fymLiveHost,
				"/", "/all-models", "/sport-rugby-player",
				"/model-spencer-wood-1628-young-blond-teen",
				"/nearly-nude-1573-photoshoot-bonus-video",
				// One live entry really does end in an encoded newline.
				"/fit-and-famous-1524-social-media-star%0A",
			))
		case strings.HasPrefix(r.URL.Path, "/model-"):
			_, _ = fmt.Fprint(w, fymProfile("Spencer Wood", "Gym", "Young Blond Teen Shows off his Lean &amp; Toned Body",
				"A profile description.", "9 Aug 26", "1628"))
		case strings.HasPrefix(r.URL.Path, "/nearly-nude-"), strings.HasPrefix(r.URL.Path, "/fit-and-famous-"):
			_, _ = fmt.Fprint(w, fymBonus("1573", "Cruz Hilton", "Photoshoot Bonus Video", "Gymnast",
				"4 Jan 2026", "The real synopsis.", "8.00"))
		default:
			http.NotFound(w, r)
		}
	}))
	return site
}

// Only the shoot sections are scenes; `sport-` and the site pages are indexes.
func TestFitYoungMenSitemapSelectsOnlyShoots(t *testing.T) {
	site := fitYoungMenSite(t)
	defer site.Close()

	s := newScraper(siteCfg(t, "fityoungmen"), site.Server)
	scenes, errs, _, _ := collect(t, s, "https://www.fityoungmen.com/", scraper.ListOpts{Workers: 2})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3", len(scenes))
	}
	if site.count("/sport-rugby-player") != 0 {
		t.Error("a sport index page was fetched as a scene")
	}
}

// The h1 is four nested spans with the sport inside the title. Everything but
// the title has to be cut out, or the stored title reads
// "Fit Young Men: Spencer Wood - Gym - …".
func TestFitYoungMenProfileTitleDropsTheSiteAndModelSpans(t *testing.T) {
	d := parseFitYoungMen(fymProfile("Spencer Wood", "Gym", "Young Blond Teen Shows off his Lean &amp; Toned Body",
		"A profile description.", "9 Aug 26", "1628"))

	if d.title != "Young Blond Teen Shows off his Lean & Toned Body" {
		t.Errorf("title = %q", d.title)
	}
	if len(d.performers) != 1 || d.performers[0] != "Spencer Wood" {
		t.Errorf("performers = %v", d.performers)
	}
	if d.sport != "Gym" {
		t.Errorf("sport = %q", d.sport)
	}
	if d.date.Format("2006-01-02") != "2026-08-09" {
		t.Errorf("date = %v — the two-digit year should read as this century", d.date)
	}
	if d.description != "A profile description." {
		t.Errorf("description = %q", d.description)
	}
}

// The bonus-video layout is a third template; a `model-` URL for one of these
// redirects to it, and without it 69 shoots reported as parse failures.
func TestFitYoungMenBonusLayout(t *testing.T) {
	d := parseFitYoungMen(fymBonus("1573", "Cruz Hilton", "Photoshoot Bonus Video", "Gymnast",
		"4 Jan 2026", "The real synopsis.", "8.00"))

	if d.title != "Photoshoot Bonus Video" {
		t.Errorf("title = %q", d.title)
	}
	if len(d.performers) != 1 || d.performers[0] != "Cruz Hilton" {
		t.Errorf("performers = %v", d.performers)
	}
	if d.sport != "Gymnast" {
		t.Errorf("sport = %q", d.sport)
	}
	if d.date.Format("2006-01-02") != "2026-01-04" {
		t.Errorf("date = %v", d.date)
	}
	// The purchase boilerplate precedes the synopsis and must not be stored as
	// the description.
	if d.description != "The real synopsis." {
		t.Errorf("description = %q", d.description)
	}
	if d.price != 8 {
		t.Errorf("price = %v", d.price)
	}
}

// ---- shared behaviour ----

// One live sitemap entry ends in an encoded newline. Decoding it puts a control
// character in the path and makes the fetch URL unparseable.
func TestEncodedNewlineInASitemapEntrySurvives(t *testing.T) {
	site := fitYoungMenSite(t)
	defer site.Close()

	s := newScraper(siteCfg(t, "fityoungmen"), site.Server)
	scenes, errs, _, _ := collect(t, s, "https://www.fityoungmen.com/", scraper.ListOpts{Workers: 1})
	for _, e := range errs {
		if strings.Contains(e.Error(), "control character") {
			t.Fatalf("the encoded newline broke the fetch: %v", e)
		}
	}
	var got bool
	for _, sc := range scenes {
		if sc.ID == "1524" {
			got = true
		}
	}
	if !got {
		t.Error("the entry with the encoded newline was dropped")
	}
}

// English Lads carries the date in every URL, so the walk can be ordered before
// the KnownIDs stop is applied — otherwise the stop would fire at whatever
// position the sitemap happened to put the stored scene in.
func TestSortNewestFirstUsesTheURLDateThenTheID(t *testing.T) {
	mk := func(id, date string) listEntry {
		e := listEntry{id: id}
		if date != "" {
			e.date, _ = time.Parse("2006-01-02", date)
		}
		return e
	}
	entries := []listEntry{
		mk("00676", "2006-02-24"),
		mk("02979", "2025-05-16"),
		mk("01500", "2015-01-01"),
	}
	sortNewestFirst(entries)
	if entries[0].id != "02979" || entries[2].id != "00676" {
		t.Errorf("date order wrong: %v", []string{entries[0].id, entries[1].id, entries[2].id})
	}

	// Fit Young Men has no date in its URLs, so the sequential id stands in.
	undated := []listEntry{mk("101", ""), mk("1628", ""), mk("760", "")}
	sortNewestFirst(undated)
	if undated[0].id != "1628" || undated[2].id != "101" {
		t.Errorf("id order wrong: %v", []string{undated[0].id, undated[1].id, undated[2].id})
	}
}

func TestKnownIDStopsAfterSorting(t *testing.T) {
	site := englishLadsSite(t)
	defer site.Close()

	s := newScraper(siteCfg(t, "englishlads"), site.Server)
	scenes, _, _, stopped := collect(t, s, "https://www.englishlads.com/",
		scraper.ListOpts{Workers: 1, KnownIDs: map[string]bool{"00676": true}})

	if !stopped {
		t.Error("StoppedEarly was not reported")
	}
	if len(scenes) != 1 || scenes[0].ID != "02979" {
		t.Fatalf("got %v, want just the newer scene", scenes)
	}
}

// A single shoot URL skips the sitemap.
func TestSingleShootURLSkipsTheSitemap(t *testing.T) {
	site := englishLadsSite(t)
	defer site.Close()

	s := newScraper(siteCfg(t, "englishlads"), site.Server)
	scenes, errs, total, _ := collect(t, s,
		"https://www.englishlads.com/video-2025-05-16-02979-N-recent-premium-update", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	if site.count("/sitemap.xml") != 0 {
		t.Error("a single-shoot URL read the sitemap")
	}
}

// A sitemap that fetched cleanly and named no shoots is a URL-shape change,
// which must not read as an empty catalogue to an authoritative --full save.
func TestSitemapWithNoShootsIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, sitemapXML(elLiveHost, "/", "/models", "/join"))
	}))
	defer srv.Close()

	s := newScraper(siteCfg(t, "englishlads"), srv)
	scenes, errs, _, _ := collect(t, s, "https://www.englishlads.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("a sitemap naming no shoots reported no error")
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
		if !s.MatchesURL("https://www." + cfg.Domain + "/") {
			t.Errorf("%s does not match its own host", cfg.SiteID)
		}
		for _, other := range sites {
			if other.Domain != cfg.Domain && s.MatchesURL("https://www."+other.Domain+"/") {
				t.Errorf("%s also matches %s", cfg.SiteID, other.Domain)
			}
		}
	}
}

func TestScenesValidate(t *testing.T) {
	cases := []struct {
		id   string
		site func(*testing.T) *stubSite
	}{
		{"englishlads", englishLadsSite},
		{"fityoungmen", fitYoungMenSite},
	}
	for _, c := range cases {
		site := c.site(t)
		s := newScraper(siteCfg(t, c.id), site.Server)
		scenes, _, _, _ := collect(t, s, "https://www."+siteCfg(t, c.id).Domain+"/", scraper.ListOpts{Workers: 2})
		if len(scenes) == 0 {
			t.Errorf("%s produced no scenes to validate", c.id)
		}
		for _, sc := range scenes {
			testutil.ValidateScene(t, sc)
		}
		site.Close()
	}
}

func TestContextCancellationStopsTheWalk(t *testing.T) {
	site := englishLadsSite(t)
	defer site.Close()

	s := newScraper(siteCfg(t, "englishlads"), site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://www.englishlads.com/", scraper.ListOpts{Workers: 2, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
