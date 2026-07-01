package wifeysworld

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
<div class="grid">
<div class="update_details grid__item" data-setid="3454">
	<a class="card-custom" title="" href="https://join.wifeysworld.com/signup">
		<span class="card-custom__wrap-img">
			<img class="stdimage" src0_1x="/v3/tour/content//contentthumbs/55/93/15593-1x.jpg">
		</span>
		<span class="card-custom__title">Wifey&#039;s Summer Facial!</span>
	</a>
	<span class="card-section-date">06/22/2026</span>
</div>
<div class="update_details grid__item" data-setid="3453">
	<a class="card-custom" title="" href="https://join.wifeysworld.com/signup">
		<img class="stdimage" src0_1x="/v3/tour/content//contentthumbs/55/87/15587-1x.jpg">
		<span class="card-custom__title">YoungGun Cums 2X!</span>
	</a>
	<span class="card-section-date">06/15/2026</span>
</div>
</div>
</body></html>`
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://wifeysworld.com/v3/tour/categories/updates_1_d.html", true},
		{"https://www.wifeysworld.com/", true},
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
	orig := baseURL
	defer func() { baseURL = orig }()
	baseURL = "https://wifeysworld.com"

	scenes := parseListing([]byte(listingHTML()), "studioURL", time.Now().UTC())
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	s0 := scenes[0]
	if s0.ID != "3454" {
		t.Errorf("ID = %q, want 3454", s0.ID)
	}
	if s0.Title != "Wifey's Summer Facial!" {
		t.Errorf("Title = %q", s0.Title)
	}
	if s0.Studio != studio {
		t.Errorf("Studio = %q", s0.Studio)
	}
	if s0.SiteID != siteID {
		t.Errorf("SiteID = %q", s0.SiteID)
	}
	if y, m, d := s0.Date.Date(); y != 2026 || m != 6 || d != 22 {
		t.Errorf("Date = %v, want 2026-06-22", s0.Date)
	}
	if s0.Thumbnail != "https://wifeysworld.com/v3/tour/content//contentthumbs/55/93/15593-1x.jpg" {
		t.Errorf("Thumbnail = %q", s0.Thumbnail)
	}
	if s0.URL != "https://wifeysworld.com/v3/tour/" {
		t.Errorf("URL = %q", s0.URL)
	}
}

func TestListScenes(t *testing.T) {
	orig := baseURL
	defer func() { baseURL = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "updates_1_d.html") {
			_, _ = fmt.Fprint(w, listingHTML())
			return
		}
		// later pages are empty -> Paginate stops
		_, _ = fmt.Fprint(w, "<html></html>")
	}))
	defer ts.Close()
	baseURL = ts.URL

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
	if got["3454"] != "Wifey's Summer Facial!" || got["3453"] != "YoungGun Cums 2X!" {
		t.Errorf("scenes = %v", got)
	}
}
