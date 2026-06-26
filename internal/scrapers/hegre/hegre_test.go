package hegre

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// ---- fixtures ----

// filmDetailHTML mirrors a /films/{slug} page: og:* metadata, a runtime row,
// and a record-model cast block.
const filmDetailHTML = `<html><head>
<meta property="og:title" content="Sensual Massage &amp; Light">
<meta property="og:description" content="An artful film.">
<meta property="og:image" content="https://cdn.hegre.com/films/cover.jpg">
</head><body>
<div class="stats">
  <span>Runtime:</span> <strong> 42:17 </strong>
</div>
<div class="cast">
  <a href="/models/anna" class="record-model" title="Anna">Anna</a>
  <a href="/models/mike" class="record-model" title="Mike Jones">Mike Jones</a>
  <a href="/models/anna" class="record-model" title="Anna">Anna</a>
</div>
</body></html>`

// modelPageHTML mirrors a /models/{slug} page listing that model's films.
const modelPageHTML = `<html><body>
<a href="/films/sensual-massage">Sensual Massage</a>
<a href="/films/morning-light">Morning Light</a>
<a href="/films/sensual-massage">dup link</a>
<a href="/something-else">not a film</a>
</body></html>`

// modelsIndexHTML mirrors the /models index page.
const modelsIndexHTML = `<html><body>
<a href="/models/anna">Anna</a>
<a href="/models/mike">Mike</a>
<a href="/models/anna">dup</a>
</body></html>`

// ---- parse-level tests ----

func TestFetchFilm(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/films/sensual-massage" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, filmDetailHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.client = ts.Client()
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)

	scene, ok := s.fetchFilm(context.Background(), "https://www.hegre.com", "sensual-massage", now)
	if !ok {
		t.Fatal("fetchFilm ok=false")
	}
	if scene.ID != "sensual-massage" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "hegre" || scene.Studio != "Hegre" {
		t.Errorf("SiteID/Studio = %q/%q", scene.SiteID, scene.Studio)
	}
	if scene.Title != "Sensual Massage & Light" {
		t.Errorf("Title = %q (want unescaped)", scene.Title)
	}
	if scene.Description != "An artful film." {
		t.Errorf("Description = %q", scene.Description)
	}
	if scene.Thumbnail != "https://cdn.hegre.com/films/cover.jpg" {
		t.Errorf("Thumbnail = %q", scene.Thumbnail)
	}
	if scene.URL != ts.URL+"/films/sensual-massage" {
		t.Errorf("URL = %q", scene.URL)
	}
	// 42:17 -> 2537 seconds.
	if scene.Duration != 2537 {
		t.Errorf("Duration = %d, want 2537", scene.Duration)
	}
	if len(scene.Performers) != 2 || scene.Performers[0] != "Anna" || scene.Performers[1] != "Mike Jones" {
		t.Errorf("Performers = %v, want [Anna Mike Jones]", scene.Performers)
	}
}

func TestFetchModelFilms(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models/anna" {
			_, _ = fmt.Fprint(w, modelPageHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.client = ts.Client()

	films := s.fetchModelFilms(context.Background(), "anna")
	if len(films) != 2 {
		t.Fatalf("got %d films, want 2 (deduped, films only): %v", len(films), films)
	}
	sort.Strings(films)
	if films[0] != "morning-light" || films[1] != "sensual-massage" {
		t.Errorf("films = %v", films)
	}
}

func TestFetchModelSlugs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			_, _ = fmt.Fprint(w, modelsIndexHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.client = ts.Client()

	slugs := s.fetchModelSlugs(context.Background())
	if len(slugs) != 2 {
		t.Fatalf("got %d slugs, want 2 (deduped): %v", len(slugs), slugs)
	}
	// fetchModelSlugs sorts.
	if slugs[0] != "anna" || slugs[1] != "mike" {
		t.Errorf("slugs = %v", slugs)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://www.hegre.com/":            true,
		"http://hegre.com/films/foo":        true,
		"https://www.hegre.com/models/anna": true,
		"https://www.example.com/films/foo": false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

// ---- end-to-end run() via httptest ----

// hegreServer serves the models index, two model pages, and the film detail.
func hegreServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/models":
			_, _ = fmt.Fprint(w, modelsIndexHTML)
		case "/models/anna":
			_, _ = fmt.Fprint(w, modelPageHTML)
		case "/models/mike":
			// Mike features in one shared film + one unique film.
			_, _ = fmt.Fprint(w, `<html><body>
<a href="/films/sensual-massage">x</a>
<a href="/films/solo-mike">y</a>
</body></html>`)
		default:
			// Any /films/{slug} returns the same detail shape.
			_, _ = fmt.Fprint(w, filmDetailHTML)
		}
	}))
}

func TestListScenes_fullCatalogue(t *testing.T) {
	ts := hegreServer(t)
	defer ts.Close()

	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			ids[r.Scene.ID] = true
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	// Unique films across both models: sensual-massage, morning-light, solo-mike.
	want := []string{"morning-light", "sensual-massage", "solo-mike"}
	for _, id := range want {
		if !ids[id] {
			t.Errorf("missing film %q in %v", id, ids)
		}
	}
	if len(ids) != 3 {
		t.Errorf("got %d unique films, want 3: %v", len(ids), ids)
	}
}

func TestListScenes_singleFilm(t *testing.T) {
	ts := hegreServer(t)
	defer ts.Close()

	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL+"/films/sensual-massage", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	for r := range ch {
		if r.Kind == scraper.KindScene {
			count++
			if r.Scene.ID != "sensual-massage" {
				t.Errorf("ID = %q", r.Scene.ID)
			}
		}
	}
	if count != 1 {
		t.Errorf("got %d scenes, want 1", count)
	}
}

func TestListScenes_singleModel(t *testing.T) {
	ts := hegreServer(t)
	defer ts.Close()

	old := siteBase
	siteBase = ts.URL
	defer func() { siteBase = old }()

	s := New()
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL+"/models/anna", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for r := range ch {
		if r.Kind == scraper.KindScene {
			ids[r.Scene.ID] = true
		}
	}
	if len(ids) != 2 || !ids["sensual-massage"] || !ids["morning-light"] {
		t.Errorf("anna films = %v, want sensual-massage+morning-light", ids)
	}
}
