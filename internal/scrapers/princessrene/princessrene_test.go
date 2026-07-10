package princessrene

import (
	"testing"
	"time"
)

// listingFixture mirrors the real /videos/ card markup: real scene links plus
// category and pagination links that must be filtered out, with one slug
// repeated to exercise dedup.
const listingFixture = `
<div class="item item-video">
  <div class="item-image">
    <a href="https://worshiprene.com/videos/beg-to-see-my-tits/"><img src="https://worshiprene.com/media/video/13359/x-640x360.jpg" /></a>
  </div>
  <div class="item-content">
    <div class="duration">09:40</div>
    <h3><a href="https://worshiprene.com/videos/beg-to-see-my-tits/">Beg to See My Tits</a></h3>
    <div class="terms">
      <a href="https://worshiprene.com/videos/category/joi" class="term-link">JOI</a>
    </div>
  </div>
</div>
<div class="item item-video">
  <div class="item-content">
    <h3><a href="https://worshiprene.com/videos/jerk-for-me/">Jerk for Me</a></h3>
  </div>
</div>
<nav class="pagination">
  <a href="https://worshiprene.com/videos/page/2/">2</a>
</nav>
`

func TestParseListing(t *testing.T) {
	slugs := parseListing([]byte(listingFixture))
	want := []string{"beg-to-see-my-tits", "jerk-for-me"}
	if len(slugs) != len(want) {
		t.Fatalf("got %d slugs %v, want %d %v", len(slugs), slugs, len(want), want)
	}
	for i, w := range want {
		if slugs[i] != w {
			t.Errorf("slug[%d] = %q, want %q", i, slugs[i], w)
		}
	}
}

func TestParseListingEmpty(t *testing.T) {
	// A page with only category/pagination links yields no scenes (end of list).
	const noScenes = `
<a href="https://worshiprene.com/videos/category/feet">Feet</a>
<a href="https://worshiprene.com/videos/page/3/">3</a>
`
	if got := parseListing([]byte(noScenes)); len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}

// detailFixture captures the real OpenGraph + Yoast JSON-LD shape of a detail
// page (with HTML-entity-encoded description and "- Princess Rene" title suffix).
const detailFixture = `<!DOCTYPE html><html><head>
<meta property="og:locale" content="en_US" />
<meta property="og:type" content="article" />
<meta property="og:title" content="Beg to See My Tits - Princess Rene" />
<meta property="og:description" content="There&#8217;s one part of me that you&#8217;ve always dreamed about seeing: my perky breasts &hellip;" />
<meta property="og:url" content="https://worshiprene.com/videos/beg-to-see-my-tits/" />
<meta property="og:site_name" content="Princess Rene" />
<meta property="og:image" content="https://worshiprene.com/media/video/13359/princess-rene-beg-to-see-my-tits_frame.0000005.jpg" />
<script type="application/ld+json" class="yoast-schema-graph">{"@context":"https://schema.org","@graph":[{"@type":"WebPage","datePublished":"2026-05-25T20:58:28+00:00","dateModified":"2026-05-25T20:58:28+00:00"}]}</script>
</head><body></body></html>`

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))

	if d.title != "Beg to See My Tits" {
		t.Errorf("title = %q, want %q", d.title, "Beg to See My Tits")
	}
	wantDesc := "There’s one part of me that you’ve always dreamed about seeing: my perky breasts …"
	if d.description != wantDesc {
		t.Errorf("description = %q, want %q", d.description, wantDesc)
	}
	if d.thumbnail != "https://worshiprene.com/media/video/13359/princess-rene-beg-to-see-my-tits_frame.0000005.jpg" {
		t.Errorf("thumbnail = %q", d.thumbnail)
	}
	want := time.Date(2026, 5, 25, 20, 58, 28, 0, time.UTC)
	if !d.date.Equal(want) {
		t.Errorf("date = %v, want %v", d.date, want)
	}
}

func TestSlugToTitle(t *testing.T) {
	if got := slugToTitle("beg-to-see-my-tits"); got != "beg to see my tits" {
		t.Errorf("got %q", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://worshiprene.com/videos/":            true,
		"https://www.worshiprene.com/videos/page/2/": true,
		"http://worshiprene.com":                     true,
		"https://meanawolf.com/videos/":              false,
		"https://worshipreneXcom/videos/":            false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}
