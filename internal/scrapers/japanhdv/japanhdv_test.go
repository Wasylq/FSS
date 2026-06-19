package japanhdv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestMatchesURL(t *testing.T) {
	s := New()
	good := []string{
		"https://japanhdv.com/",
		"https://www.japanhdv.com/japan-porn/page/2/",
		"http://japanhdv.com/wife-for-rent-runa-nanami-scene1/",
	}
	for _, u := range good {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://example.com/", "https://notjapanhdv.org/"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestParseListing(t *testing.T) {
	items := parseListing(loadFixture(t, "listing.html"))
	if len(items) == 0 {
		t.Fatal("parseListing returned no items")
	}

	var found *listItem
	for i := range items {
		if items[i].id == "wife-for-rent-runa-nanami-scene1" {
			found = &items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected scene wife-for-rent-runa-nanami-scene1 in %d items", len(items))
	}
	if found.title != "Rental Wife Runa Nanami Satisfies Horny Client" {
		t.Errorf("title = %q", found.title)
	}
	if found.url != "https://japanhdv.com/wife-for-rent-runa-nanami-scene1/" {
		t.Errorf("url = %q", found.url)
	}
	if found.duration != 54*60+52 {
		t.Errorf("duration = %d, want %d", found.duration, 54*60+52)
	}
	if len(found.performers) != 1 || found.performers[0] != "Runa Nanami" {
		t.Errorf("performers = %v", found.performers)
	}
	if !strings.HasPrefix(found.thumbnail, "https://") {
		t.Errorf("thumbnail not absolute https: %q", found.thumbnail)
	}
}

func TestMaxPageNum(t *testing.T) {
	if got := maxPageNum(loadFixture(t, "listing.html")); got != 50 {
		t.Errorf("maxPageNum = %d, want 50", got)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail(loadFixture(t, "detail.html"))
	if !strings.Contains(d.description, "Runa Nanami has a new business") {
		t.Errorf("description = %q", d.description)
	}
	if len(d.performers) == 0 || d.performers[0] != "Runa Nanami" {
		t.Errorf("performers = %v", d.performers)
	}
	wantTags := map[string]bool{"Big Tits": false, "Creampie": false, "Housewife": false}
	for _, c := range d.categories {
		if _, ok := wantTags[c]; ok {
			wantTags[c] = true
		}
	}
	for tag, seen := range wantTags {
		if !seen {
			t.Errorf("category %q not parsed; got %v", tag, d.categories)
		}
	}
}

func TestRunEndToEnd(t *testing.T) {
	listing := loadFixture(t, "listing.html")
	detail := loadFixture(t, "detail.html")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/japan-porn/page/1/") {
			// Rewrite absolute scene/thumbnail links to point at this server so
			// detail fetches stay offline.
			body := strings.ReplaceAll(string(listing), "https://japanhdv.com", "http://"+r.Host)
			_, _ = w.Write([]byte(body))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/japan-porn/page/") {
			// Only one page in the fixture; later pages are empty.
			_, _ = w.Write([]byte("<html></html>"))
			return
		}
		// Any scene detail path.
		_, _ = w.Write(detail)
	}))
	defer ts.Close()

	s := New()
	s.base = ts.URL
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var scenes int
	var sawDesc bool
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes++
			sc := res.Scene
			if sc.ID == "" || sc.Title == "" || sc.URL == "" {
				t.Errorf("incomplete scene: %+v", sc)
			}
			if sc.SiteID != "japanhdv" || sc.Studio != "Japan HDV" {
				t.Errorf("wrong site/studio: %q / %q", sc.SiteID, sc.Studio)
			}
			if sc.ScrapedAt.IsZero() {
				t.Errorf("scene %s has zero ScrapedAt", sc.ID)
			}
			if strings.Contains(sc.Description, "Runa Nanami has a new business") {
				sawDesc = true
			}
		case scraper.KindError:
			t.Errorf("unexpected error result: %v", res.Err)
		}
	}
	if scenes == 0 {
		t.Fatal("no scenes scraped")
	}
	if !sawDesc {
		t.Error("expected at least one scene enriched with og:description from detail page")
	}
}

func TestKnownIDsEarlyStop(t *testing.T) {
	listing := loadFixture(t, "listing.html")
	detail := loadFixture(t, "detail.html")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/japan-porn/page/1/") {
			body := strings.ReplaceAll(string(listing), "https://japanhdv.com", "http://"+r.Host)
			_, _ = w.Write([]byte(body))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/japan-porn/page/") {
			_, _ = w.Write([]byte("<html></html>"))
			return
		}
		_, _ = w.Write(detail)
	}))
	defer ts.Close()

	s := New()
	s.base = ts.URL
	s.client = ts.Client()

	// First item on page 1 is known -> early stop, zero scenes emitted.
	known := map[string]bool{"man-rice-anri-shimamura-scene1": true}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/", scraper.ListOpts{Workers: 2, KnownIDs: known})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	var stopped bool
	for res := range ch {
		if res.Kind == scraper.KindStoppedEarly {
			stopped = true
		}
	}
	if !stopped {
		t.Error("expected StoppedEarly when first card is a known ID")
	}
}
