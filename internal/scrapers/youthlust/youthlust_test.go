package youthlust

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://youthlust.club/":                       true,
		"https://youthlust.club/products/sharon-bts":    true,
		"https://www.youthlust.club/collections/models": true,
		"https://example.com/":                          false,
		"https://youthlustfan.com/":                     false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestParsePrice(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"150.00", 150, true},
		{"9.99", 9.99, true},
		{"0.00", 0, false},
		{"", 0, false},
		{"free", 0, false},
	}
	for _, c := range cases {
		got, ok := parsePrice(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parsePrice(%q) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestStripHTML(t *testing.T) {
	got := stripHTML("<p>Enjoy the &amp; amazing  boobs</p>")
	if got != "Enjoy the & amazing boobs" {
		t.Errorf("stripHTML = %q", got)
	}
}

const fixtureProducts = `{"products":[
  {"id":1,"title":"Sharon behind the scenes","handle":"sharon-bts","body_html":"<p>Cool &amp; fun BTS</p>","published_at":"2026-06-25T20:52:34-06:00","created_at":"2026-06-25T20:50:20-06:00","vendor":"YouthLust","product_type":"Videos","tags":["solo","outdoor"],"variants":[{"price":"150.00"}],"images":[{"src":"https://cdn.shopify.com/x.png"}]},
  {"id":2,"title":"Mega Bundle","handle":"mega-bundle","product_type":"Bundles & Combos","variants":[{"price":"900.00"}],"images":[]}
]}`

func TestRunParsesVideosOnly(t *testing.T) {
	var page1Hit bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			page1Hit = true
			_, _ = fmt.Fprint(w, fixtureProducts)
			return
		}
		_, _ = fmt.Fprint(w, `{"products":[]}`)
	}))
	defer ts.Close()

	s := New()
	s.apiBase = ts.URL
	out := make(chan scraper.SceneResult)
	go s.run(context.Background(), scraper.ListOpts{}, out)

	var scenes []scraper.SceneResult
	for r := range out {
		if r.Kind == scraper.KindScene {
			scenes = append(scenes, r)
		}
	}
	if !page1Hit {
		t.Fatal("page 1 never fetched")
	}
	if len(scenes) != 1 {
		t.Fatalf("expected 1 video scene (bundle filtered out), got %d", len(scenes))
	}
	sc := scenes[0].Scene
	if sc.Title != "Sharon behind the scenes" {
		t.Errorf("title = %q", sc.Title)
	}
	if sc.ID != "1" {
		t.Errorf("id = %q", sc.ID)
	}
	if sc.Description != "Cool & fun BTS" {
		t.Errorf("description = %q", sc.Description)
	}
	if sc.Date.Format("2006-01-02") != "2026-06-26" {
		t.Errorf("date = %v (want UTC-shifted 2026-06-26)", sc.Date)
	}
	if sc.LowestPrice != 150 {
		t.Errorf("price = %v", sc.LowestPrice)
	}
	if len(sc.Tags) != 2 {
		t.Errorf("tags = %v", sc.Tags)
	}
}
