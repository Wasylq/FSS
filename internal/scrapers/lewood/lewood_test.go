package lewood

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// ---- fixtures ----

// oneCard renders a single listing grid card matching the production markup
// (item-grid item-grid-scene wrapper, scene-title anchor, scene-performer-names).
func oneCard(id, href, title, thumb, perfBlock string) string {
	return fmt.Sprintf(`
<div class="grid-item" id="ascene_%s">
  <div class="item-grid item-grid-scene">
    <a class="scene-title" href="%s" title="%s">
      <img class="lazy" data-srcset="%s 700w, %s 1024w" />
    </a>
    <h6 class="scene-title-text"><a class="scene-title" href="%s">%s</a></h6>
    <p class="scene-performer-names">%s</p>
  </div>
</div>`, id, href, title, thumb, thumb, href, title, perfBlock)
}

func listingHTML(cards ...string) string {
	var sb strings.Builder
	sb.WriteString(`<html><body><div class="grid-row">`)
	for _, c := range cards {
		sb.WriteString(c)
	}
	// Trailing recommendation widget that must be ignored (no scene-title).
	sb.WriteString(`<div class="grid-item" id="ascene_999"><p>related</p></div>`)
	sb.WriteString(`</div></body></html>`)
	return sb.String()
}

const detailHTML = `<html><head>
<meta name="og:description" content="A steamy &amp; intense LeWood scene." />
</head><body>
<div class="scene-meta"><span class="label">Released:</span>Jun 03, 2026</div>
</body></html>`

func card1() string {
	return oneCard(
		"1788734",
		"/1788734/anal-queens-streaming-scene-video.html",
		"Anal Queens",
		"https://imgs.lewood.com/1788734/700w.jpg",
		`<a href="/p/dee-williams">Dee Williams</a> &amp; <a href="/p/manuel-ferrara">Manuel Ferrara</a>`,
	)
}

func card2() string {
	return oneCard(
		"1799001",
		"/1799001/double-trouble-streaming-scene-video.html",
		"Double Trouble &amp; More",
		"https://imgs.lewood.com/1799001/700w.jpg",
		`<a href="/p/kira-noir">Kira Noir</a>`,
	)
}

// ---- MatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.lewood.com/", true},
		{"http://lewood.com/watch-newest-lewood-clips-and-scenes.html?page=2", true},
		{"https://www.lewood.com/1788734/anal-queens-streaming-scene-video.html", true},
		{"https://www.evilangel.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- listing parser ----

func TestParseCard(t *testing.T) {
	it, ok := parseCard(card1())
	if !ok {
		t.Fatal("parseCard returned ok=false")
	}
	if it.id != "1788734" {
		t.Errorf("id = %q, want 1788734", it.id)
	}
	if it.title != "Anal Queens" {
		t.Errorf("title = %q", it.title)
	}
	if it.url != "https://www.lewood.com/1788734/anal-queens-streaming-scene-video.html" {
		t.Errorf("url = %q", it.url)
	}
	if it.thumb != "https://imgs.lewood.com/1788734/700w.jpg" {
		t.Errorf("thumb = %q", it.thumb)
	}
	want := []string{"Dee Williams", "Manuel Ferrara"}
	if len(it.performers) != 2 || it.performers[0] != want[0] || it.performers[1] != want[1] {
		t.Errorf("performers = %v, want %v", it.performers, want)
	}
}

func TestFetchListingIgnoresNonSceneCards(t *testing.T) {
	s := &Scraper{}
	body := []byte(listingHTML(card1(), card2()))
	// fetchListing does a GET; exercise the split/filter logic via parseCard
	// over the same fixture to confirm the trailing widget is excluded.
	blocks := cardSplitRe.Split(string(body), -1)
	got := 0
	for i := 1; i < len(blocks); i++ {
		if !strings.Contains(blocks[i], `class="scene-title"`) {
			continue
		}
		if _, ok := parseCard(blocks[i]); ok {
			got++
		}
	}
	if got != 2 {
		t.Errorf("parsed %d scene cards, want 2", got)
	}
	_ = s
}

func TestParseCardHTMLEntities(t *testing.T) {
	it, ok := parseCard(card2())
	if !ok {
		t.Fatal("parseCard returned ok=false")
	}
	if it.title != "Double Trouble & More" {
		t.Errorf("title = %q, want decoded entity", it.title)
	}
	if len(it.performers) != 1 || it.performers[0] != "Kira Noir" {
		t.Errorf("performers = %v", it.performers)
	}
}

// ---- detail parser ----

func TestDetailRegexes(t *testing.T) {
	m := releasedRe.FindStringSubmatch(detailHTML)
	if m == nil {
		t.Fatal("releasedRe did not match")
	}
	if strings.TrimSpace(m[1]) != "Jun 03, 2026" {
		t.Errorf("released date = %q", m[1])
	}
	d := metaDescRe.FindStringSubmatch(detailHTML)
	if d == nil {
		t.Fatal("metaDescRe did not match")
	}
	if got := cleanText(d[1]); got != "A steamy & intense LeWood scene." {
		t.Errorf("description = %q", got)
	}
}

// ---- end-to-end ----

func TestListScenesEndToEnd(t *testing.T) {
	prev := siteBase
	t.Cleanup(func() { siteBase = prev })

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case browsePath:
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprint(w, listingHTML(card1(), card2()))
				return
			}
			_, _ = fmt.Fprint(w, `<html><body></body></html>`)
		default:
			_, _ = fmt.Fprint(w, detailHTML)
		}
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}

	scenes := map[string]bool{}
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes[r.Scene.ID] = true
			if r.Scene.SiteID != siteID {
				t.Errorf("SiteID = %q", r.Scene.SiteID)
			}
			if r.Scene.Studio != studioName {
				t.Errorf("Studio = %q", r.Scene.Studio)
			}
			wantDate := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
			if !r.Scene.Date.Equal(wantDate) {
				t.Errorf("Date = %v, want %v", r.Scene.Date, wantDate)
			}
			if r.Scene.Description != "A steamy & intense LeWood scene." {
				t.Errorf("Description = %q", r.Scene.Description)
			}
		case scraper.KindError:
			t.Errorf("unexpected error: %v", r.Err)
		}
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(scenes), scenes)
	}
	if !scenes["1788734"] || !scenes["1799001"] {
		t.Errorf("missing expected scene IDs: %v", scenes)
	}
}
