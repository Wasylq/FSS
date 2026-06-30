package bondagecafe

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

func listingHTML() string {
	return `<html><body><ul>
<li>
<img alt="1100-1199/wmbcv-1124" class="stdimage " src="content/1100-1199/wmbcv-1124/8.jpg" src0_1x="content/1100-1199/wmbcv-1124/8.jpg" src0_2x="content/1100-1199/wmbcv-1124/8-2x.jpg" src0_3x="content/1100-1199/wmbcv-1124/8-3x.jpg" />
<div class="caption"><div class="centerwrap">
<h3><a onclick="tload('/trailers/1100-1199_wmbcv-1124.mp4')">wmbcv-1124: Cherie DeVille - Sheer Restraint</a></h3>
<p><span class="tour_update_models"><a href="https://www.bondagecafe.com/models/CherieDeVille.html">Cherie DeVille</a></span></p>
</div></div>
</li>
<li>
<img alt="0500-0599/wmbcv-0532" class="stdimage " src="content/0500-0599/wmbcv-0532/8.jpg" src0_3x="content/0500-0599/wmbcv-0532/8-3x.jpg" />
<div class="caption"><div class="centerwrap">
<h3><a onclick="tload('/trailers/0500-0599_wmbcv-0532.mp4')">wmbcv-0532: Kendra James and Randy Moore - Sexy</a></h3>
<p><span class="tour_update_models"><a href="/models/KendraJames.html">Kendra James</a><a href="/models/RandyMoore.html">Randy Moore</a></span></p>
</div></div>
</li>
<li>
<div class="caption"><div class="centerwrap">
<h3><a href="/signup.html">Click here for unlimited access to all our featured movies.</a></h3>
</div></div>
</li>
</ul></body></html>`
}

func emptyHTML() string {
	return `<html><body><ul><li><div class="caption"><h3><a href="/signup.html">Click here for unlimited access.</a></h3></div></li></ul></body></html>`
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.bondagecafe.com/updates/page_1.html", true},
		{"https://bondagecafe.com/", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestParseListing(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	scenes := parseListing(listingHTML(), "studioURL", now)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (CTA dropped): %+v", len(scenes), scenes)
	}
	sc := scenes[0]
	if sc.ID != "wmbcv-1124" {
		t.Errorf("ID = %q, want wmbcv-1124", sc.ID)
	}
	if sc.Title != "Cherie DeVille - Sheer Restraint" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Studio != "Bondage Cafe" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.URL != siteBase+"/trailers/1100-1199_wmbcv-1124.mp4" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != siteBase+"/content/1100-1199/wmbcv-1124/8-3x.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if strings.Join(sc.Performers, ",") != "Cherie DeVille" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if strings.Join(scenes[1].Performers, ",") != "Kendra James,Randy Moore" {
		t.Errorf("scene1 Performers = %v", scenes[1].Performers)
	}
}

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "page_1.html") {
			_, _ = fmt.Fprint(w, listingHTML())
			return
		}
		_, _ = fmt.Fprint(w, emptyHTML())
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
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r.Scene.Title
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["wmbcv-1124"] != "Cherie DeVille - Sheer Restraint" {
		t.Errorf("scenes = %v", got)
	}
}
