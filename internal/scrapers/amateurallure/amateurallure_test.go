package amateurallure

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// listingCard renders one server-rendered listing card. host should be an
// absolute origin (e.g. https://www.amateurallure.com or an httptest URL) so
// the embedded detail href satisfies sceneURLRe's https? requirement.
func listingCard(host, slug, title, date string) string {
	return fmt.Sprintf(`<div class="update_details">
  <a href="%s/tour/scenes/%s_vids.html">
    <img src0_1x="/tour/content/thumbs/%s-1x.jpg" alt="%s" />
  </a>
  <div class="meta"><strong>Added:</strong> %s</div>
</div>`, host, slug, slug, title, date)
}

func listingPage(host string, cards ...string) string {
	body := `<html><body><div class="updates">`
	for _, c := range cards {
		body += c
	}
	return body + `</div></body></html>`
}

const detailPage = `<html><head>
<meta property="og:title" content="Jane Doe Allure Casting &amp; More" />
<meta property="og:description" content="Jane shows off her skills." />
<meta property="og:image" content="https://www.amateurallure.com/img/jane.jpg" />
</head><body>
<a href="/tour/models/Jane-Doe.html">Jane Doe</a>
<a href="/tour/models/Mark-Smith-xxx-videos.html">Mark Smith</a>
<a href="/tour/models/models.html">All Models</a>
</body></html>`

func TestFetchListing(t *testing.T) {
	page := listingPage("https://www.amateurallure.com",
		listingCard("https://www.amateurallure.com", "jane-doe", "Jane Doe Allure", "06/18/2026"),
		listingCard("https://www.amateurallure.com", "amy-roe", "Amy Roe &amp; Friend", "05/10/2026"),
		// duplicate slug must be deduped
		listingCard("https://www.amateurallure.com", "jane-doe", "Jane Doe Allure", "06/18/2026"),
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, page)
	}))
	defer ts.Close()

	s := New()
	s.Client = ts.Client()
	items, err := s.fetchListing(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (deduped)", len(items))
	}
	first := items[0]
	if first.id != "jane-doe" {
		t.Errorf("id = %q", first.id)
	}
	if first.url != "https://www.amateurallure.com/tour/scenes/jane-doe_vids.html" {
		t.Errorf("url = %q", first.url)
	}
	if first.title != "Jane Doe Allure" {
		t.Errorf("title = %q", first.title)
	}
	if first.date != "06/18/2026" {
		t.Errorf("date = %q", first.date)
	}
	if first.thumb != "https://www.amateurallure.com/tour/content/thumbs/jane-doe-1x.jpg" {
		t.Errorf("thumb = %q", first.thumb)
	}
	if items[1].title != "Amy Roe & Friend" {
		t.Errorf("second title = %q", items[1].title)
	}
}

func TestParsePerformers(t *testing.T) {
	got := parsePerformers([]byte(detailPage))
	if len(got) != 2 {
		t.Fatalf("got %v, want 2 performers", got)
	}
	if got[0] != "Jane Doe" {
		t.Errorf("performer[0] = %q", got[0])
	}
	// -xxx-videos SEO suffix must be stripped.
	if got[1] != "Mark Smith" {
		t.Errorf("performer[1] = %q", got[1])
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.amateurallure.com/", true},
		{"http://amateurallure.com/tour/updates/page_1.html", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v", c.url, got)
		}
	}
}

func TestID(t *testing.T) {
	if New().ID() != "amateurallure" {
		t.Errorf("ID = %q", New().ID())
	}
}

func TestListScenesEndToEnd(t *testing.T) {
	origBase, origListing := siteBase, listingURL
	defer func() { siteBase, listingURL = origBase, origListing }()

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tour/updates/page_1.html":
			_, _ = fmt.Fprint(w, listingPage(ts.URL,
				listingCard(ts.URL, "jane-doe", "Jane Doe Allure", "06/18/2026"),
				listingCard(ts.URL, "amy-roe", "Amy Roe", "05/10/2026"),
			))
		case "/tour/scenes/jane-doe_vids.html", "/tour/scenes/amy-roe_vids.html":
			_, _ = fmt.Fprint(w, detailPage)
		default:
			_, _ = fmt.Fprint(w, listingPage(ts.URL))
		}
	}))
	defer ts.Close()
	siteBase = ts.URL
	listingURL = siteBase + "/tour/updates/page_%d.html"

	s := New()
	s.Client = ts.Client()
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			// Detail enrichment overrides the listing title via og:title.
			if r.Scene.Title != "Jane Doe Allure Casting & More" {
				t.Errorf("Title = %q (og:title not applied?)", r.Scene.Title)
			}
			if r.Scene.Description != "Jane shows off her skills." {
				t.Errorf("Description = %q", r.Scene.Description)
			}
			if len(r.Scene.Performers) != 2 {
				t.Errorf("Performers = %v", r.Scene.Performers)
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}
