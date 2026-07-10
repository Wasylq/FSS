package xxcelutil

import "testing"

func newXXCel() *Scraper {
	return New(SiteConfig{SiteID: "xxcel", Domain: "xx-cel.com", Host: "https://xx-cel.com", StudioName: "XX-Cel"})
}

const listingFixture = `
<div class="grid">
  <a href="/movies/video-megara-steele-video-8">
    <div class="image-wrapper">
      <video preload="none" poster="//media.xx-cel.com/content/movies/video-megara-steele-video-8/cover/hd.jpg"></video>
    </div>
  </a>
  <a href="/movies/video-another-scene">
    <video poster="//media.xx-cel.com/content/movies/video-another-scene/screenshots/video-another-scene_screen97.jpg"></video>
  </a>
  <a href="/movies/page-2/?sort=recent">Next</a>
  <a href="/movies/page-30/?sort=recent">Last</a>
</div>
`

const detailFixtureXC = `
<h1>Megara Steele video 8</h1>
<div class="vid-details">
  <span class="released title"> starring: <a href='/models/megara-steele'>Megara Steele</a> </span>
  <span class="released title"> released on: <strong>Feb 26, 2024</strong> </span>
  <span class="duration title"> duration: <strong>10:21</strong> </span>
</div>
`

const detailFixtureHH = `
<div class="vid-details text-center-mobile">
  <span class="feature title"> <strong><a href='/models/roxy-rush'>Roxy Rush</a></strong> </span>
  <span class="released title"> released on: <strong>Jan 26, 2024</strong> </span>
  <span class="duration title"> duration: <strong>45:09</strong> </span>
</div>
`

func TestParseListing(t *testing.T) {
	s := newXXCel()
	scenes := s.parseListing([]byte(listingFixture))
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes (page- links skipped), got %d", len(scenes))
	}
	if scenes[0].slug != "video-megara-steele-video-8" {
		t.Errorf("slug = %q", scenes[0].slug)
	}
	if scenes[0].url != "https://xx-cel.com/movies/video-megara-steele-video-8" {
		t.Errorf("url = %q", scenes[0].url)
	}
	if scenes[0].thumb != "https://media.xx-cel.com/content/movies/video-megara-steele-video-8/cover/hd.jpg" {
		t.Errorf("thumb = %q", scenes[0].thumb)
	}
	// second card uses a screenshots-style poster (heavyonhotties layout)
	if scenes[1].thumb != "https://media.xx-cel.com/content/movies/video-another-scene/screenshots/video-another-scene_screen97.jpg" {
		t.Errorf("thumb[1] = %q", scenes[1].thumb)
	}
}

func TestEstimateTotal(t *testing.T) {
	if got := estimateTotal([]byte(listingFixture), 24); got != 30*24 {
		t.Errorf("estimateTotal = %d, want %d", got, 30*24)
	}
}

func TestParseDetail(t *testing.T) {
	dx := parseDetail([]byte(detailFixtureXC))
	if dx.date.Format("2006-01-02") != "2024-02-26" {
		t.Errorf("XC date = %v", dx.date)
	}
	if dx.duration != 10*60+21 {
		t.Errorf("XC duration = %d, want %d", dx.duration, 10*60+21)
	}
	if len(dx.performers) != 1 || dx.performers[0] != "Megara Steele" {
		t.Errorf("XC performers = %v", dx.performers)
	}

	dh := parseDetail([]byte(detailFixtureHH))
	if dh.date.Format("2006-01-02") != "2024-01-26" {
		t.Errorf("HH date = %v", dh.date)
	}
	if dh.duration != 45*60+9 {
		t.Errorf("HH duration = %d", dh.duration)
	}
	if len(dh.performers) != 1 || dh.performers[0] != "Roxy Rush" {
		t.Errorf("HH performers = %v", dh.performers)
	}
}

func TestSlugToTitle(t *testing.T) {
	cases := map[string]string{
		"video-megara-steele-video-8": "Megara Steele Video 8",
		"i-love-redheads":             "I Love Redheads",
	}
	for in, want := range cases {
		if got := slugToTitle(in); got != want {
			t.Errorf("slugToTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	xc := newXXCel()
	hh := New(SiteConfig{SiteID: "heavyonhotties", Domain: "heavyonhotties.com", Host: "https://www.heavyonhotties.com"})
	if !xc.MatchesURL("https://xx-cel.com/movies/page-1/") {
		t.Error("xc should match xx-cel.com")
	}
	if xc.MatchesURL("https://heavyonhotties.com/") {
		t.Error("xc should not match heavyonhotties.com")
	}
	if !hh.MatchesURL("https://www.heavyonhotties.com/movies/i-love-redheads") {
		t.Error("hh should match www.heavyonhotties.com")
	}
}
