package goddessfootdomination

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

// card mirrors one listing entry: the scene is linked twice, from the
// thumbnail and again from the title.
func card(id, slug, title, duration string) string {
	return fmt.Sprintf(`
<div class="co-12 col-sm-6 col-lg-3 col-xl-3 mb-3">
  <div class="ratio ratio-16x9 overflow-hidden">
    <a href="https://example.test/v/%s-%s" class="d-block text-center webp-preview">
      <img src="https://members.example.test/storage/content/video/2026-Aug-18/%s/thumbnail.webp" class="img-fluid thumb"/>
    </a>
    <small class="duration d-inline-block mx-1">
            %s
    </small>
  </div>
  <a href="https://example.test/v/%s-%s"><h3 class="fs-6 my-2 fw-normal text-truncate">%s</h3></a>
</div>`, id, slug, title, duration, id, slug, title)
}

// detailPage carries the three ld+json blocks a real scene page has. The
// VideoObject under hasPart describes the teaser, not the scene.
func detailPage(title, desc, date string, actors, genres []string, duration string) string {
	q := func(in []string) string {
		var b []string
		for _, v := range in {
			b = append(b, fmt.Sprintf("%q", v))
		}
		return strings.Join(b, ",")
	}
	var actorObjs []string
	for _, a := range actors {
		actorObjs = append(actorObjs, fmt.Sprintf(`{"@type":"Person","name":%q}`, a))
	}
	return fmt.Sprintf(`<html><head>
<script type="application/ld+json">{"@context":"https://schema.org/","@type":"WebPage","name":"listing"}</script>
<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebSite","name":"Site"}</script>
<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Movie","name":%q,"description":%q,
 "datePublished":%q,"image":["https://cdn.example.test/%s/thumbnail.webp","https://cdn.example.test/b.webp"],
 "genre":[%s],"actor":[%s],
 "productionCompany":{"@type":"Organization","name":"Example"},
 "hasPart":{"@type":"VideoObject","name":"Teaser - %s","description":"teaser only"}}
</script></head><body>
<small class="duration d-inline-block mx-1">
      %s
</small>
</body></html>`, title, desc, date, title, q(genres), strings.Join(actorObjs, ","), title, duration)
}

func testCfg(base string) SiteConfig {
	return SiteConfig{
		ID:       "goddessfootdomination",
		BaseURL:  base,
		SiteName: "Goddess Foot Domination",
		MatchRe:  regexp.MustCompile(`(?i)^https?://(?:www\.)?goddessfootdomination\.com(?:/|$)`),
	}
}

// server serves a two-page listing plus detail pages, and reports how many
// listing pages were requested.
func server(t *testing.T, pages [][]string) (*httptest.Server, *int) {
	t.Helper()
	listingHits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/all/video", func(w http.ResponseWriter, r *http.Request) {
		listingHits++
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			_, _ = fmt.Sscanf(p, "%d", &page)
		}
		if page < 1 || page > len(pages) {
			_, _ = fmt.Fprint(w, `<html><body>no results</body></html>`)
			return
		}
		_, _ = fmt.Fprint(w, "<html><body>"+strings.Join(pages[page-1], "")+"</body></html>")
	})
	mux.HandleFunc("/v/", func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimPrefix(r.URL.Path, "/v/")
		_, _ = fmt.Fprint(w, detailPage(
			"Title "+slug, "Desc "+slug, "2026-08-18T12:18:35+00:00",
			[]string{"Emma", "Goddess Emma"}, []string{"Female Domination", "Footjobs"}, "00:09:16"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &listingHits
}

func collect(t *testing.T, s *Scraper, studioURL string, opts scraper.ListOpts) ([]models.Scene, []error) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			errs = append(errs, res.Err)
		}
	}
	return scenes, errs
}

func TestListScenesWalksAndParses(t *testing.T) {
	srv, hits := server(t, [][]string{
		{card("147", "emma-sweat-sock-footjob", "Emma Sweat Sock Footjob", "00:09:16")},
		{card("146", "emma-so-close", "Emma So Close", "00:09:16")},
	})
	s := New(testCfg(srv.URL))
	s.Client = srv.Client()

	scenes, errs := collect(t, s, srv.URL+"/", scraper.ListOpts{})
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	// Walked page 1, page 2, then the empty page 3 that ends the loop.
	if *hits != 3 {
		t.Errorf("listing pages fetched = %d, want 3", *hits)
	}

	byID := map[string]models.Scene{}
	for _, sc := range scenes {
		byID[sc.ID] = sc
	}
	got, ok := byID["147"]
	if !ok {
		t.Fatalf("scene 147 missing; got %v", byID)
	}
	if want := srv.URL + "/v/147-emma-sweat-sock-footjob"; got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}
	if got.SiteID != "goddessfootdomination" {
		t.Errorf("SiteID = %q", got.SiteID)
	}
	if !strings.HasPrefix(got.Title, "Title ") {
		t.Errorf("Title = %q, want the Movie block's name", got.Title)
	}
	if got.Duration != 556 {
		t.Errorf("Duration = %d, want 556", got.Duration)
	}
	if got.Date.Format("2006-01-02") != "2026-08-18" {
		t.Errorf("Date = %v, want 2026-08-18", got.Date)
	}
	if len(got.Performers) != 2 || got.Performers[0] != "Emma" {
		t.Errorf("Performers = %v", got.Performers)
	}
	if len(got.Categories) != 2 || got.Categories[0] != "Female Domination" {
		t.Errorf("Categories = %v", got.Categories)
	}
	if got.Studio != "Goddess Foot Domination" {
		t.Errorf("Studio = %q", got.Studio)
	}
	if !strings.HasSuffix(got.Thumbnail, "thumbnail.webp") {
		t.Errorf("Thumbnail = %q, want the first image", got.Thumbnail)
	}
}

// The scene page nests a VideoObject for the teaser inside the Movie. Reading
// the first ld+json block, or preferring the VideoObject, would store the
// trailer's title and description instead of the scene's.
func TestToSceneReadsTheMovieNotTheTeaser(t *testing.T) {
	s := New(testCfg("https://example.test"))
	body := detailPage("Real Scene", "Real description", "2026-08-18T12:18:35+00:00",
		[]string{"Emma"}, []string{"Footjobs"}, "00:01:00")

	sc, err := s.toScene([]byte(body), sceneRef{id: "1", slug: "x", url: "https://example.test/v/1-x"}, "https://example.test/")
	if err != nil {
		t.Fatalf("toScene: %v", err)
	}
	if sc.Title != "Real Scene" {
		t.Errorf("Title = %q, want the Movie name", sc.Title)
	}
	if strings.Contains(sc.Description, "teaser") {
		t.Errorf("Description came from the teaser VideoObject: %q", sc.Description)
	}
}

// A card links its scene from both the thumbnail and the title, so a naive
// href sweep yields every scene twice.
func TestParseListingDedupesTheDoubleLink(t *testing.T) {
	s := New(testCfg("https://example.test"))
	body := []byte(card("147", "a-slug", "A", "00:01:00") + card("146", "b-slug", "B", "00:02:00"))

	refs := s.parseListing(body, map[string]bool{})
	if len(refs) != 2 {
		t.Fatalf("got %d refs, want 2: %+v", len(refs), refs)
	}
	if refs[0].id != "147" || refs[1].id != "146" {
		t.Errorf("ids = %s, %s", refs[0].id, refs[1].id)
	}
	if want := "https://example.test/v/147-a-slug"; refs[0].url != want {
		t.Errorf("url = %q, want %q", refs[0].url, want)
	}
}

// The listing is date-descending, so the first known ID means the rest are
// already stored.
func TestKnownIDsStopTheWalkEarly(t *testing.T) {
	srv, hits := server(t, [][]string{
		{card("147", "a", "A", "00:01:00"), card("146", "b", "B", "00:01:00")},
		{card("145", "c", "C", "00:01:00")},
	})
	s := New(testCfg(srv.URL))
	s.Client = srv.Client()

	scenes, errs := collect(t, s, srv.URL+"/", scraper.ListOpts{KnownIDs: map[string]bool{"146": true}})
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(scenes) != 1 || scenes[0].ID != "147" {
		t.Fatalf("scenes = %+v, want only 147", scenes)
	}
	if *hits != 1 {
		t.Errorf("listing pages fetched = %d, want 1 — the walk should stop on the known ID", *hits)
	}
}

func TestListingURLModes(t *testing.T) {
	s := New(testCfg("https://goddessfootdomination.com"))
	cases := []struct{ in, want string }{
		{"https://goddessfootdomination.com/", "https://goddessfootdomination.com/all/video"},
		{"https://goddessfootdomination.com", "https://goddessfootdomination.com/all/video"},
		{"https://www.goddessfootdomination.com/", "https://goddessfootdomination.com/all/video"},
		{"https://goddessfootdomination.com/all/video", "https://goddessfootdomination.com/all/video"},
		{"https://goddessfootdomination.com/category/footjobs", "https://goddessfootdomination.com/category/footjobs"},
		{"https://goddessfootdomination.com/actor/emma", "https://goddessfootdomination.com/actor/emma"},
	}
	for _, c := range cases {
		if got := s.listingURL(c.in); got != c.want {
			t.Errorf("listingURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(sites[0])
	for _, u := range []string{
		"https://goddessfootdomination.com/",
		"https://www.goddessfootdomination.com/all/video",
		"http://goddessfootdomination.com",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"https://goddessfootdomination.com.evil.invalid/",
		"https://example.com/",
	} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

// A page that loads but carries no Movie block is a parse failure, not an
// empty site — the difference decides whether an authoritative Save may
// delete scenes.
func TestMissingMovieBlockIsAParseError(t *testing.T) {
	s := New(testCfg("https://example.test"))
	_, err := s.toScene([]byte("<html><body>nothing here</body></html>"),
		sceneRef{id: "1", slug: "x", url: "https://example.test/v/1-x"}, "https://example.test/")
	if err == nil {
		t.Fatal("want an error for a page with no Movie block")
	}
}

func TestBaseURLsAreApexHosts(t *testing.T) {
	for _, cfg := range sites {
		if strings.Contains(cfg.BaseURL, "//www.") {
			t.Errorf("%s: BaseURL = %q, want the apex host (the cert has no www SAN)", cfg.ID, cfg.BaseURL)
		}
	}
}

func TestIDAndPatterns(t *testing.T) {
	s := New(sites[0])
	if s.ID() != "goddessfootdomination" {
		t.Errorf("ID() = %q", s.ID())
	}
	if len(s.Patterns()) == 0 {
		t.Error("Patterns() is empty; fss list-scrapers would show nothing")
	}
}

// A listing page that fails to load must be reported. A silent return would
// be indistinguishable from a site with nothing on it, and under --full the
// authoritative Save would then delete the catalogue.
func TestListingFetchErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := New(testCfg(srv.URL))
	s.Client = srv.Client()

	scenes, errs := collect(t, s, srv.URL+"/", scraper.ListOpts{})
	if len(errs) == 0 {
		t.Error("a failing listing page produced no error result")
	}
	if len(scenes) != 0 {
		t.Errorf("got %d scenes from a failing listing", len(scenes))
	}
}

// One bad detail page costs that scene, not the run.
func TestDetailFetchErrorIsReportedPerScene(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/all/video", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "" {
			_, _ = fmt.Fprint(w, "<html><body>empty</body></html>")
			return
		}
		_, _ = fmt.Fprint(w, "<html><body>"+
			card("1", "good", "Good", "00:01:00")+
			card("2", "bad", "Bad", "00:01:00")+"</body></html>")
	})
	mux.HandleFunc("/v/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "-bad") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = fmt.Fprint(w, detailPage("Good", "d", "2026-08-18T12:00:00+00:00",
			[]string{"A"}, []string{"C"}, "00:01:00"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := New(testCfg(srv.URL))
	s.Client = srv.Client()

	scenes, errs := collect(t, s, srv.URL+"/", scraper.ListOpts{})
	if len(errs) != 1 {
		t.Errorf("got %d errors, want 1", len(errs))
	}
	if len(scenes) != 1 || scenes[0].ID != "1" {
		t.Errorf("scenes = %+v, want just the good one", scenes)
	}
}

// A page whose Movie block carries no name is a parse failure, not a scene
// with an empty title.
func TestMovieWithoutANameIsAnError(t *testing.T) {
	s := New(testCfg("https://example.test"))
	body := `<script type="application/ld+json">{"@context":"https://schema.org","@type":"Movie","description":"d"}</script>`
	if _, err := s.toScene([]byte(body), sceneRef{id: "1", url: "u"}, "https://example.test/"); err == nil {
		t.Error("want an error when the Movie block has no name")
	}
}

// A malformed ld+json block must not stop the search: the real pages carry
// several, and only one of them is the Movie.
func TestFindMovieSkipsMalformedBlocks(t *testing.T) {
	body := []byte(`
<script type="application/ld+json">{ this is not json </script>
<script type="application/ld+json">{"@type":"WebPage","name":"x"}</script>
<script type="application/ld+json">{"@type":"Movie","name":"Real"}</script>`)
	ld := findMovie(body)
	if ld == nil || ld.Name != "Real" {
		t.Errorf("findMovie = %+v, want the Movie block", ld)
	}
	if findMovie([]byte("<html>nothing</html>")) != nil {
		t.Error("findMovie found a Movie in a page with none")
	}
}

// schema.org allows image to be a single URL or a list; both appear in the
// wild, and a strict decode would drop the thumbnail entirely.
func TestFirstImageAcceptsBothShapes(t *testing.T) {
	cases := []struct{ in, want string }{
		{``, ""},
		{`"https://a/x.webp"`, "https://a/x.webp"},
		{`["https://a/1.webp","https://a/2.webp"]`, "https://a/1.webp"},
		{`[]`, ""},
		{`{"@type":"ImageObject"}`, ""},
	}
	for _, c := range cases {
		if got := firstImage([]byte(c.in)); got != c.want {
			t.Errorf("firstImage(%s) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Cancellation must stop the walk and close the channel rather than leak the
// goroutine — the scraper sends on an unbuffered channel throughout.
func TestCancellationStopsTheScrape(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	mux := http.NewServeMux()
	mux.HandleFunc("/all/video", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>"+card("1", "a", "A", "00:01:00")+"</body></html>")
	})
	mux.HandleFunc("/v/", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := New(testCfg(srv.URL))
	s.Client = srv.Client()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, srv.URL+"/", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("channel still open after cancellation — the scraper leaks a goroutine")
	}
}

// The per-request delay is honoured on both the listing walk and the detail
// pool, so an operator's rate limit actually applies.
func TestDelayIsHonoured(t *testing.T) {
	srv, _ := server(t, [][]string{
		{card("1", "a", "A", "00:01:00")},
		{card("2", "b", "B", "00:01:00")},
	})
	s := New(testCfg(srv.URL))
	s.Client = srv.Client()

	start := time.Now()
	scenes, errs := collect(t, s, srv.URL+"/", scraper.ListOpts{Delay: 40 * time.Millisecond, Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	// Two extra listing pages plus two details, all delayed.
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("scrape took %v, too fast for a 40ms delay — it is being skipped", elapsed)
	}
}
