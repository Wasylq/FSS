package bondagejunkies

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
	return `<html><body>
<div class="pagenav"><ul><li><a href="updates?page=2">2</a></li></ul></div>

<a name="1999"></a>
<div style="float: left;"><h1 class="updatetitle">Bella Ink vs. Her Long Edge<img src="/images/update-new.png"></h1><p class="titletags"><a href="/tag/ball+gag">ball gag</a>, <a href="/tag/rope">rope</a>, <a href="/tag/teasing">teasing</a></p></div>
<div style="float: right;"><p class="byliner" style="text-align: right;">#1999&nbsp;&nbsp;2026-06-19<br />43 photos, 19 min video</p></div>
<div class="clearfix"></div>
<div class="video-preview"><video poster="/images/preview/1999-lg-1.jpg"></video></div>
<p class="updatedesc">Bella protested again that she had things to do.</p>

<a name="1987"></a>
<div style="float: left;"><h1 class="updatetitle">Bella Trix vs. Her Rope Dessert<img src="/images/update-new.png"></h1><p class="titletags"><a href="/tag/rope">rope</a>, <a href="/tag/frogtie">frogtie</a></p></div>
<div style="float: right;"><p class="byliner" style="text-align: right;">#1987&nbsp;&nbsp;2026-06-12<br />44 photos, 15 min video</p></div>
<div class="clearfix"></div>
<div class="video-preview"><video poster="/images/preview/1987-lg-1.jpg"></video></div>
<p class="updatedesc">A rope dessert for Bella.</p>
</body></html>`
}

func emptyHTML() string { return `<html><body><div class="pagenav"></div></body></html>` }

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://bondagejunkies.com/updates?page=1", true},
		{"https://www.bondagejunkies.com/", true},
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
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "1999" {
		t.Errorf("ID = %q, want 1999", sc.ID)
	}
	if sc.Title != "Bella Ink vs. Her Long Edge" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.SiteID != siteID {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.Studio != "Bondage Junkies" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.URL != siteBase+"/updates#1999" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != siteBase+"/images/preview/1999-lg-1.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	wantDate := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if sc.Duration != 19*60 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 19*60)
	}
	if strings.Join(sc.Tags, ",") != "ball gag,rope,teasing" {
		t.Errorf("Tags = %v", sc.Tags)
	}
	if sc.Description != "Bella protested again that she had things to do." {
		t.Errorf("Description = %q", sc.Description)
	}
}

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
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
	if got["1999"] != "Bella Ink vs. Her Long Edge" {
		t.Errorf("scenes = %v", got)
	}
}
