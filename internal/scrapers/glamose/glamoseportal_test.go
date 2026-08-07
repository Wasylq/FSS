package glamose

import (
	"testing"
	"time"
)

const portalFixture = `
<html><body>
<div class="box">
  <a href="/?update_id=1234"><img data-src="https://cdn.glamose.com/th/1234.jpg" class="play-icon"></a>
  <div class="info">
    <a href="/model/anna-x">Anna &amp; Eve</a>
    <span class="site">Glamose Classic</span>
    <span class="date">3rd Jan 2024</span>
  </div>
</div>
<div class="box">
  <a href="/?update_id=99"><img src="https://cdn.glamose.com/th/99.jpg"></a>
  <div class="info">
    <a href="/model/bella">Bella</a>
    <span class="date">15 January 2023</span>
  </div>
</div>
<div class="box">
  <div class="info"><a href="/model/nobody">No Update Id</a></div>
</div>
</body></html>`

func TestParsePortalPage(t *testing.T) {
	const studioURL = "https://www.glamose.com/"
	scenes := parsePortalPage([]byte(portalFixture), studioURL)

	// The third box carries no update_id and is the scene's only identity, so
	// it must be dropped rather than stored under an empty ID.
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	first := scenes[0]
	if first.ID != "1234" {
		t.Errorf("ID = %q, want 1234", first.ID)
	}
	if first.SiteID != "glamose" || first.Studio != "Glamose" {
		t.Errorf("SiteID/Studio = %q/%q", first.SiteID, first.Studio)
	}
	if first.StudioURL != studioURL {
		t.Errorf("StudioURL = %q, want %q", first.StudioURL, studioURL)
	}
	if want := "https://www.glamose.com/?update_id=1234"; first.URL != want {
		t.Errorf("URL = %q, want %q", first.URL, want)
	}
	if first.Title != "Anna & Eve" {
		t.Errorf("Title = %q, want the unescaped model name", first.Title)
	}
	if len(first.Performers) != 1 || first.Performers[0] != "Anna & Eve" {
		t.Errorf("Performers = %v", first.Performers)
	}
	if first.Series != "Glamose Classic" {
		t.Errorf("Series = %q, want Glamose Classic", first.Series)
	}
	if want := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC); !first.Date.Equal(want) {
		t.Errorf("Date = %v, want %v (ordinal suffix stripped)", first.Date, want)
	}
	if first.Thumbnail != "https://cdn.glamose.com/th/1234.jpg" {
		t.Errorf("Thumbnail = %q", first.Thumbnail)
	}
	if len(first.Tags) != 1 || first.Tags[0] != "Video" {
		t.Errorf("Tags = %v, want [Video] for a box with a play icon", first.Tags)
	}

	second := scenes[1]
	if second.ID != "99" {
		t.Errorf("ID = %q, want 99", second.ID)
	}
	if want := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC); !second.Date.Equal(want) {
		t.Errorf("Date = %v, want %v (full month layout)", second.Date, want)
	}
	// src= is as valid as data-src=.
	if second.Thumbnail != "https://cdn.glamose.com/th/99.jpg" {
		t.Errorf("Thumbnail = %q", second.Thumbnail)
	}
	if second.Series != "" {
		t.Errorf("Series = %q, want empty when the box has no site span", second.Series)
	}
	if len(second.Tags) != 0 {
		t.Errorf("Tags = %v, want none without a play icon", second.Tags)
	}
}

func TestParsePortalPageEmpty(t *testing.T) {
	if got := parsePortalPage([]byte("<html><body>no boxes</body></html>"), "https://www.glamose.com/"); len(got) != 0 {
		t.Errorf("got %d scenes, want 0", len(got))
	}
}

func TestPortalMatchesURL(t *testing.T) {
	s := &portalScraper{}
	for _, u := range []string{
		"https://www.glamose.com/",
		"https://glamose.com",
		"http://www.glamose.com/?update_id=1",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"https://www.glamosetour.com/",
		"https://example.com/glamose.com",
	} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}
