package spunkworthy

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

func listingHTML() string {
	card := func(id, title, slug, date string) string {
		return fmt.Sprintf(`<div class="vid">
  <p><a href="/preview/view_video/%s?page=1">%s</a></p>
  <a href="/preview/view_video/%s?page=1"><img src="/images/hd.png" class="play_butt"></a>
  <a href="/preview/view_video/%s?page=1"><img src="/preview/videos/%s/poster_tn.jpg" width="300" height="225" /></a>
  <!-- <span class="date">%s</span> -->
</div>`, id, title, id, id, slug, date)
	}
	return "<html><body>" +
		card("50832", "Niles", "niles", "19 Jun 26") +
		card("50343", "Brody &amp; Chance: JO buddies", "brody-chance", "12 Jun 26") +
		card("50832", "Niles", "niles", "19 Jun 26") + // dup
		"</body></html>"
}

func detailHTML() string {
	return `<html><body>
<p class="sec_nav">Previous | <a href="/preview/videos?page=1">Videos :: Page 1</a> | <a href="/preview/view_video/50343">Next &gt;&gt;</a></p>
<div class="content">
  <div class="video_synopsis hd">
    <p class="vid_pitch center"><span class="h3">Watch the full scene:</span><a href="/join">Join Now!</a></p>
    <div class="vid_text">
      <div class="scene_models">
        <div class="hs center">
          <a href="/preview/view_guy/375"><img src="/preview/guys/niles_tn.jpg" /></a>
          <p><a href="/preview/view_guy/375">More of Niles</a></p>
          <p>&nbsp;</p>
          <a href="/preview/view_photos/760" class="butt_plain"><span> See Related Photos </span></a>
        </div>
      </div>
      <p>Niles grew up in So Cal, living the beach life.</p>
      <p>He hadn't given porn much thought until recently.</p>
      <p>Tags: <a href="/preview/videos?show=Big Cumshot">Big Cumshot</a></p>
    </div>
  </div>
</div>
</body></html>`
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.spunkworthy.com/preview/videos?page=1", true},
		{"https://spunkworthy.com/preview/view_video/50832", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestFetchListing ----

func TestFetchListing(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML())
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	items, err := s.fetchListing(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (dup dropped): %+v", len(items), items)
	}
	if items[0].id != "50832" || items[0].title != "Niles" {
		t.Errorf("item0 = %+v", items[0])
	}
	if items[1].title != "Brody & Chance: JO buddies" {
		t.Errorf("item1 title = %q", items[1].title)
	}
	if items[0].date != "19 Jun 26" {
		t.Errorf("item0 date = %q", items[0].date)
	}
	if items[0].poster != siteBase+"/preview/videos/niles/poster_tn.jpg" {
		t.Errorf("item0 poster = %q", items[0].poster)
	}
}

// ---- TestToScene ----

func TestToScene(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML())
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	it := listItem{id: "50832", title: "Niles", poster: siteBase + "/preview/videos/niles/poster_tn.jpg", date: "19 Jun 26"}
	now := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	sc := s.toScene(context.Background(), "studioURL", it, now)

	if sc.ID != "50832" || sc.SiteID != siteID || sc.Studio != studioName {
		t.Errorf("identity = %+v", sc)
	}
	if sc.URL != siteBase+"/preview/view_video/50832" {
		t.Errorf("URL = %q", sc.URL)
	}
	wantDate := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if strings.Join(sc.Performers, ",") != "Niles" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if !strings.Contains(sc.Description, "So Cal") || strings.Contains(sc.Description, "Tags:") || strings.Contains(sc.Description, "More of") {
		t.Errorf("Description = %q", sc.Description)
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/preview/view_video/"):
			_, _ = fmt.Fprint(w, detailHTML())
		case r.URL.Query().Get("page") == "1":
			_, _ = fmt.Fprint(w, listingHTML())
		default:
			// page 2+ -> empty -> Done.
			_, _ = fmt.Fprint(w, "<html></html>")
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
	if got["50832"] != "Niles" || got["50343"] != "Brody & Chance: JO buddies" {
		t.Errorf("scenes = %v", got)
	}
}
