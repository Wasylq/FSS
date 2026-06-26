package abbywinters

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// card renders one <article> listing card. host should be an absolute origin so
// the detail href points somewhere fetchable in end-to-end tests.
func card(host, shootID, slug, title, category, pubDate string) string {
	return fmt.Sprintf(`<article class="item item-shoot">
  <a href="%s/amateurs/shoots/%s" class="card-thumb">
    <img data-src="https://cdn.abbywinters.com/%s/thumb.jpg?w=300" />
  </a>
  <h2 class="card-title">%s <span class="pull-right text-cap">%s</span></h2>
  <div data-publishdate="%s"></div>
</article>`, host, slug, shootID, title, category, pubDate)
}

func listingPage(host string, total string, cards ...string) string {
	body := `<html><body>`
	if total != "" {
		body += `<span id="browse-total-count">` + total + `</span>`
	}
	body += `<div class="browse-grid">`
	for _, c := range cards {
		body += c
	}
	return body + `</div></body></html>`
}

const detailPage = `<html><body>
<table class="shoot-meta">
  <tr><th>Release date</th><td>18 Jun 2026</td></tr>
  <tr><th>Girls in this Scene</th><td>
    <a href="/amateurs/models/jane">Jane</a>
    <a href="/amateurs/models/amy">Amy</a>
    <a href="/amateurs/models/jane">Jane</a>
  </td></tr>
</table>
</body></html>`

func TestParseCards(t *testing.T) {
	page := listingPage("https://www.abbywinters.com", "1,234",
		card("https://www.abbywinters.com", "12345", "jane-and-amy-solo", "Jane &amp; Amy", "Girl-Girl", "202606"),
		card("https://www.abbywinters.com", "67890", "single-girl", "Single Girl", "Solo", "202605"),
		// handlebars stub with no cdn id must be skipped
		`<article class="item template"><h2 class="card-title">stub</h2></article>`,
	)
	items := parseCards([]byte(page))
	if len(items) != 2 {
		t.Fatalf("got %d cards, want 2", len(items))
	}
	first := items[0]
	if first.id != "12345" {
		t.Errorf("id = %q", first.id)
	}
	if first.title != "Jane & Amy" {
		t.Errorf("title = %q", first.title)
	}
	if first.category != "Girl-Girl" {
		t.Errorf("category = %q", first.category)
	}
	if first.url != "https://www.abbywinters.com/amateurs/shoots/jane-and-amy-solo" {
		t.Errorf("url = %q", first.url)
	}
	if first.thumbnail != "https://cdn.abbywinters.com/12345/thumb.jpg" {
		t.Errorf("thumbnail = %q (query should be stripped)", first.thumbnail)
	}
	if first.pubDate != "202606" {
		t.Errorf("pubDate = %q", first.pubDate)
	}
}

func TestTotalCount(t *testing.T) {
	page := listingPage("https://www.abbywinters.com", "1,234",
		card("https://www.abbywinters.com", "1", "a", "A", "Solo", "202606"))
	m := totalRe.FindSubmatch([]byte(page))
	if m == nil {
		t.Fatal("totalRe did not match")
	}
	if got := parseInt(string(m[1])); got != 1234 {
		t.Errorf("parseInt = %d, want 1234", got)
	}
}

func TestSlugFromURL(t *testing.T) {
	cases := map[string]string{
		"https://x.com/amateurs/shoots/jane-solo":  "jane-solo",
		"https://x.com/amateurs/shoots/jane-solo/": "jane-solo",
		"noslash": "noslash",
	}
	for in, want := range cases {
		if got := slugFromURL(in); got != want {
			t.Errorf("slugFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.abbywinters.com/amateurs/shoots", true},
		{"http://abbywinters.com/", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v", c.url, got)
		}
	}
}

func TestID(t *testing.T) {
	if New().ID() != "abbywinters" {
		t.Errorf("ID = %q", New().ID())
	}
}

func TestListScenesEndToEnd(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/amateurs/shoots":
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprint(w, listingPage(ts.URL, "2",
					card(ts.URL, "12345", "jane-and-amy", "Jane &amp; Amy", "Girl-Girl", "202606"),
					card(ts.URL, "67890", "single-girl", "Single Girl", "Solo", "202605"),
				))
				return
			}
			// Page 2 repeats earlier content; run() dedupes and stops.
			_, _ = fmt.Fprint(w, listingPage(ts.URL, "2",
				card(ts.URL, "12345", "jane-and-amy", "Jane &amp; Amy", "Girl-Girl", "202606"),
			))
		case "/amateurs/shoots/jane-and-amy", "/amateurs/shoots/single-girl":
			_, _ = fmt.Fprint(w, detailPage)
		default:
			_, _ = fmt.Fprint(w, listingPage(ts.URL, "2"))
		}
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := New()
	s.client = ts.Client()
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			if r.Scene.Date.Year() != 2026 || r.Scene.Date.Month() != 6 || r.Scene.Date.Day() != 18 {
				t.Errorf("Date = %v, want 2026-06-18 (release date from detail)", r.Scene.Date)
			}
			if len(r.Scene.Performers) != 2 {
				t.Errorf("Performers = %v, want 2 (deduped)", r.Scene.Performers)
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if scenes != 2 {
		t.Errorf("got %d scenes, want 2", scenes)
	}
}
