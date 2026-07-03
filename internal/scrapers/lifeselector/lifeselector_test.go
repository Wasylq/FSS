package lifeselector

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

func card(id, slug, title string, actors []string) string {
	var ab strings.Builder
	for i, a := range actors {
		if i > 0 {
			ab.WriteString(", ")
		}
		slug := strings.ToLower(strings.ReplaceAll(a, " ", "-"))
		fmt.Fprintf(&ab, `<a href="/model/%s">%s</a>`, slug, a)
	}
	return fmt.Sprintf(`<div class="story thumbnail ">
<a href="/game/%s/%s" class="thumb"><picture>
<source data-srcset="https://cdn.example/pics/%s_1_storycoversoft_1_360_247.webp 1x, https://cdn.example/pics/%s_2x.webp 2x"></picture></a>
<div class="textual">
<a href="/game/%s/%s" class="title truncate">
            %s        </a>
<div class="actors truncate">%s</div>
</div></div>`, id, slug, id, id, id, slug, title, ab.String())
}

func listingHTML(cards ...string) string {
	return `<html><body><div class="grid">` + strings.Join(cards, "\n") + `</div></body></html>`
}

func detailHTML(desc string) string {
	return fmt.Sprintf(`<html><head>
<meta property="og:title" content="A game | LifeSelector">
<meta property="og:description" content="%s">
</head><body>full model directory <a href="/model/zzz-unrelated">ZZZ</a></body></html>`, desc)
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://lifeselector.com/games?page=1", true},
		{"https://www.lifeselector.com/game/1/x", true},
		{"https://21roles.com/games", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestParseCard ----

func TestParseCard(t *testing.T) {
	c := card("85500", "a-day-with-riley-reid", "A day with Riley Reid", []string{"Riley Reid", "Sydney Cole"})
	it, ok := parseCard(c)
	if !ok {
		t.Fatal("parseCard ok=false")
	}
	if it.id != "85500" || it.slug != "a-day-with-riley-reid" {
		t.Errorf("id/slug = %q/%q", it.id, it.slug)
	}
	if it.title != "A day with Riley Reid" {
		t.Errorf("title = %q", it.title)
	}
	if strings.Join(it.performers, ",") != "Riley Reid,Sydney Cole" {
		t.Errorf("performers = %v", it.performers)
	}
	if it.thumb != "https://cdn.example/pics/85500_1_storycoversoft_1_360_247.webp" {
		t.Errorf("thumb = %q", it.thumb)
	}

	if _, ok := parseCard(`<div>no game link</div>`); ok {
		t.Error("parseCard should reject card without game link")
	}
}

// ---- TestToScene (detail description merge) ----

func TestToScene(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML("Riley Reid is special &amp; hot."))
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	it := listItem{id: "85500", slug: "a-day-with-riley-reid", title: "A day with Riley Reid", thumb: "https://cdn/x.webp", performers: []string{"Riley Reid"}}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	sc := s.toScene(context.Background(), "studioURL", it, now)

	if sc.ID != "85500" || sc.SiteID != "lifeselector" {
		t.Errorf("identity = %q/%q", sc.ID, sc.SiteID)
	}
	if sc.URL != siteBase+"/game/85500/a-day-with-riley-reid" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Description != "Riley Reid is special & hot." {
		t.Errorf("Description = %q", sc.Description)
	}
	if strings.Join(sc.Performers, ",") != "Riley Reid" {
		t.Errorf("Performers = %v (must come from listing, not detail directory)", sc.Performers)
	}
	if sc.Studio != "Life Selector" {
		t.Errorf("Studio = %q", sc.Studio)
	}
}

// ---- TestListScenes (end-to-end + pagination stop) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/game/") {
			_, _ = fmt.Fprint(w, detailHTML("a synopsis"))
			return
		}
		switch r.URL.Query().Get("page") {
		case "1":
			_, _ = fmt.Fprint(w, listingHTML(
				card("100", "alpha", "Alpha", []string{"Ann"}),
				card("101", "beta", "Beta", []string{"Bea"}),
			))
		default:
			_, _ = fmt.Fprint(w, listingHTML())
		}
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), "studioURL", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	got := map[string]string{}
	for r := range ch {
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r.Scene.Title
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["100"] != "Alpha" || got["101"] != "Beta" {
		t.Errorf("scenes = %v", got)
	}
}
