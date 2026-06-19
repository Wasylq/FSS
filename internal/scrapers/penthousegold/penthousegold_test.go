package penthousegold

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://www.penthousegold.com/":                       true,
		"https://penthousegold.com/scenes/Sex-Opera_vids.html": true,
		"http://www.penthousegold.com/models/eva-hot.html":     true,
		"https://example.com/penthousegold.com":                false,
		"https://penthouse.com/":                               false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	sitemap, err := os.ReadFile("testdata/sitemap.xml")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := os.ReadFile("testdata/sex-opera.html")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			body := strings.ReplaceAll(string(sitemap), "BASEURL", "http://"+r.Host)
			_, _ = fmt.Fprint(w, body)
		case "/scenes/Sex-Opera_vids.html":
			_, _ = fmt.Fprint(w, string(detail))
		case "/":
			// Homepage repeats the Sex Opera link (relative) plus a gallery link.
			_, _ = fmt.Fprint(w, `<html><body>
<a href="/scenes/Sex-Opera_vids.html">Sex Opera</a>
<a href="/scenes/Lexus-Locklear-Extra_highres.html">gallery</a>
</body></html>`)
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(mux)
}

func collect(t *testing.T, s *Scraper, opts scraper.ListOpts) ([]models.Scene, []error) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), s.base+"/", opts)
	if err != nil {
		t.Fatal(err)
	}
	var scenes []models.Scene
	var errs []error
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		case scraper.KindError:
			errs = append(errs, r.Err)
		}
	}
	return scenes, errs
}

func TestScrapeParsesDetail(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New()
	s.base = ts.URL

	scenes, errs := collect(t, s, scraper.ListOpts{})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Sitemap has Sex-Opera (vids) + Kitana (vids) + Lexus (highres, skipped);
	// homepage repeats Sex-Opera. Kitana has no detail handler -> dropped.
	var sexOpera *models.Scene
	for i := range scenes {
		if scenes[i].ID == "Sex-Opera" {
			sexOpera = &scenes[i]
		}
		if strings.Contains(strings.ToLower(scenes[i].ID), "highres") {
			t.Errorf("gallery/highres scene leaked: %q", scenes[i].ID)
		}
	}
	if sexOpera == nil {
		t.Fatalf("Sex-Opera scene not found, got %d scenes", len(scenes))
	}

	sc := *sexOpera
	if sc.Title != "Sex Opera" {
		t.Errorf("Title = %q, want %q", sc.Title, "Sex Opera")
	}
	if sc.SiteID != "penthousegold" {
		t.Errorf("SiteID = %q, want penthousegold", sc.SiteID)
	}
	if sc.Studio != "Penthouse Gold" {
		t.Errorf("Studio = %q, want Penthouse Gold", sc.Studio)
	}
	if sc.URL != s.base+"/scenes/Sex-Opera_vids.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Date.Format("2006-01-02") != "2026-06-19" {
		t.Errorf("Date = %v, want 2026-06-19", sc.Date)
	}
	if !strings.HasPrefix(sc.Description, "A vintage Penthouse drama") {
		t.Errorf("Description = %q", sc.Description)
	}
	if sc.Thumbnail == "" {
		t.Error("Thumbnail empty")
	}
	wantPerf := []string{"Eva Hot", "Lynn Stone", "Dora Venter"}
	if strings.Join(sc.Performers, ",") != strings.Join(wantPerf, ",") {
		t.Errorf("Performers = %v, want %v", sc.Performers, wantPerf)
	}
	wantTags := []string{"18+", "Blondes", "full vintage movies"}
	if strings.Join(sc.Tags, ",") != strings.Join(wantTags, ",") {
		t.Errorf("Tags = %v, want %v", sc.Tags, wantTags)
	}
}

func TestScrapeKnownIDStopsEarly(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	s := New()
	s.base = ts.URL

	scenes, _ := collect(t, s, scraper.ListOpts{KnownIDs: map[string]bool{"Sex-Opera": true}})
	for _, sc := range scenes {
		if sc.ID == "Sex-Opera" {
			t.Errorf("known ID Sex-Opera should have been skipped")
		}
	}
}
