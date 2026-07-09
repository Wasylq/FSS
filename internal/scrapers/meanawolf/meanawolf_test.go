package meanawolf

import "testing"

const listingFixture = `
<div class="item" data-setid="357" data-videoposter="https://cdn.example/06/53/653-2x.jpg?token=abc">
	<a href="https://meanawolf.com/scenes/Convince-Him_vids.html"><img/></a>
</div>
<div class="item" data-setid="397" data-videoposter="https://cdn.example/06/93/693-2x.jpg?token=def">
	<a href="/scenes/Nonutchallenge2_vids.html"><img/></a>
</div>
`

const detailFixture = `
<h1>Convince Him</h1>
<ul class="videoInfo">
	<li><span>RUNTIME:</span> 48:34</li>
	<li><span>PHOTOS:</span> <a href="https://meanawolf.com/scenes/Convince-Him_highres.html">58</a></li>
	<li><span>FEATURED:</span> June 26, 2026</li>
	<li><span>FEATURING:</span> 		<a href="https://meanawolf.com/models/MeanaWolf.html">Meana Wolf</a> </li>
	<li><span>CATEGORIES:</span> <a href="https://meanawolf.com/categories/bad-girl.html">Bad Girl</a> <a href="https://meanawolf.com/categories/pov.html">POV</a></li>
</ul>
`

func TestParseListing(t *testing.T) {
	scenes := parseListing([]byte(listingFixture))
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}
	if scenes[0].slug != "Convince-Him" {
		t.Errorf("slug = %q, want Convince-Him", scenes[0].slug)
	}
	if scenes[0].url != "https://meanawolf.com/scenes/Convince-Him_vids.html" {
		t.Errorf("url = %q", scenes[0].url)
	}
	if scenes[0].thumb == "" {
		t.Errorf("thumb should be set")
	}
	// relative scene href resolves to absolute
	if scenes[1].url != "https://meanawolf.com/scenes/Nonutchallenge2_vids.html" {
		t.Errorf("relative url = %q", scenes[1].url)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))
	if d.title != "Convince Him" {
		t.Errorf("title = %q, want Convince Him", d.title)
	}
	if d.duration != 48*60+34 {
		t.Errorf("duration = %d, want %d", d.duration, 48*60+34)
	}
	if d.date.Format("2006-01-02") != "2026-06-26" {
		t.Errorf("date = %v, want 2026-06-26", d.date)
	}
	if len(d.performers) != 1 || d.performers[0] != "Meana Wolf" {
		t.Errorf("performers = %v, want [Meana Wolf]", d.performers)
	}
	if len(d.tags) != 2 || d.tags[0] != "Bad Girl" || d.tags[1] != "POV" {
		t.Errorf("tags = %v, want [Bad Girl POV]", d.tags)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://meanawolf.com/":                        true,
		"https://www.meanawolf.com/updates/page_1.html": true,
		"https://meanawolf.com/models/MeanaWolf.html":   true,
		"https://example.com/meanawolf":                 false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestSlugToTitle(t *testing.T) {
	if got := slugToTitle("No-Nut-Challenge"); got != "No Nut Challenge" {
		t.Errorf("slugToTitle = %q", got)
	}
}
