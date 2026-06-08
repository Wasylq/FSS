package britishbratz

import (
	"testing"
	"time"
)

const fixture = `<html><body>
<div class="row">
<div class="col-sm-4 single_update">
  <a href="/join">
    <img class="img-responsive" src="https://vz-1c30c71e-eb5.b-cdn.net/aa4ca853-ce32-4840-960b-0e5f28ca32a5/preview.webp?token=abc" alt="Pvc Pleasure And Poppers" />
  </a>
  <div class="col-xs-6 update_bottom">
    <h1><a href="/join">Pvc Pleasure And Poppers</a></h1>
  </div>
  <div class="col-xs-6 update_bottom update_time">
    <p><time>30 October 2025</time></p>
  </div>
  <div class="col-xs-12 update_bottom">
    <div class="update_description"><p>Description text.</p></div>
    <h2>CATEGORIES: <a href="/sub-category/mind-fuck">Mind Fuck</a></h2>
  </div>
<div class="clearfix"></div>
</div>
<div class="col-sm-4 single_update">
  <a href="/join">
    <img class="img-responsive" src="/site_resources/core_images/admin/video_bg_small.jpg" alt="Denied Scene" />
  </a>
  <div class="col-xs-6 update_bottom">
    <h1><a href="/join">Denied Scene</a></h1>
  </div>
  <div class="col-xs-6 update_bottom update_time">
    <p><time>24 April 2026</time></p>
  </div>
  <div class="col-xs-12 update_bottom">
    <div class="update_description"></div>
  </div>
<div class="clearfix"></div>
</div>
</div>
<ul class="pagination">
  <li><a href="/updates/videos/1">1</a></li>
  <li><a href="/updates/videos/2">2</a></li>
  <li><a href="/updates/videos/56">56</a></li>
</ul>
</body></html>`

func TestParseListingPage(t *testing.T) {
	scenes := parseListingPage([]byte(fixture), "https://www.britishbratz.com/")

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != "aa4ca853-ce32-4840-960b-0e5f28ca32a5" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "Pvc Pleasure And Poppers" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Thumbnail == "" || sc.Thumbnail == "/site_resources/core_images/admin/video_bg_small.jpg" {
		t.Errorf("Thumbnail = %q (should be CDN URL)", sc.Thumbnail)
	}
	want := time.Date(2025, 10, 30, 0, 0, 0, 0, time.UTC)
	if sc.Date != want {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}
	if len(sc.Tags) != 1 || sc.Tags[0] != "Mind Fuck" {
		t.Errorf("Tags = %v", sc.Tags)
	}

	sc2 := scenes[1]
	if sc2.Title != "Denied Scene" {
		t.Errorf("Title = %q", sc2.Title)
	}
	if sc2.Thumbnail != "" {
		t.Errorf("Thumbnail = %q (placeholder should be empty)", sc2.Thumbnail)
	}
	if sc2.ID != "denied-scene" {
		t.Errorf("ID = %q (should be slugified title as fallback)", sc2.ID)
	}
}

func TestEstimateTotal(t *testing.T) {
	total := estimateTotal([]byte(fixture))
	if total != 56*20 {
		t.Errorf("total = %d, want %d", total, 56*20)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.britishbratz.com/", true},
		{"https://britishbratz.com/updates", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
