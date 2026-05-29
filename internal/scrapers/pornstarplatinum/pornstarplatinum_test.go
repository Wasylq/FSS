package pornstarplatinum

import (
	"strings"
	"testing"
	"time"
)

const fixturePage = `<html><body>
<div class="panel-block video-list grid row" id="videos-list">

<div class="item no-nth col-12 mx-auto col-sm-6 col-md-4 col-lg-3 mt-5">
  <div class="item-header">
    <a href="/tour/model/4851/Leya_Falcon_in_Lesbian_Asylum_Lust.html" class="thumbnail-link">
      <img src="https://c776ef2f9b.mjedge.net/pspthumbnails/31459.jpg" class="img-fluid rounded-top" alt="" loading="lazy">
    </a>
  </div>
  <div class="item-content">
    <div class="video-meta-title"><a href="/tour/model/4851/Leya_Falcon_in_Lesbian_Asylum_Lust.html">Leya Falcon in Lesbian Asylum Lust</a></div>
    <div class="video-meta-container">
      <div class="marker left font-white">Leya Falcon</div>
      <div class="video-meta right font-white">
        05/28/2026
        <a href="/tour/model/4851/Leya_Falcon_in_Lesbian_Asylum_Lust.html"><i class="fa fa-eye"></i>2041</a>
        <a href="javascript:;" class="like-this" data-contentid="4851"><i class="fa fa-heart"></i><span class="4851-likes">2384</span></a>
      </div>
    </div>
  </div>
</div>

<div class="item no-nth col-12 mx-auto col-sm-6 col-md-4 col-lg-3 mt-5">
  <div class="item-header">
    <a href="/tour/model/3011/Savana_Styles_in_Taking_on_the_Big_Piper.html" class="thumbnail-link">
      <img src="https://c776ef2f9b.mjedge.net/pspthumbnails/22492.jpg" class="img-fluid rounded-top" alt="" loading="lazy">
    </a>
  </div>
  <div class="item-content">
    <div class="video-meta-title"><a href="/tour/model/3011/Savana_Styles_in_Taking_on_the_Big_Piper.html">Savana Styles in Taking on the Big Piper</a></div>
    <div class="video-meta-container">
      <div class="marker left font-white">Savana Styles</div>
      <div class="video-meta right font-white">
        04/14/2026
        <a href="/tour/model/3011/Savana_Styles_in_Taking_on_the_Big_Piper.html"><i class="fa fa-eye"></i>1502</a>
        <a href="javascript:;" class="like-this" data-contentid="3011"><i class="fa fa-heart"></i><span class="3011-likes">980</span></a>
      </div>
    </div>
  </div>
</div>

</div>

<!-- pagination -->
<a href="scenes.php?page=2">2</a>
<a href="scenes.php?page=3">3</a>
<a href="scenes.php?page=295">Last</a>

</body></html>`

func TestParseCards(t *testing.T) {
	cards, maxPage := parseCards([]byte(fixturePage))
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	if maxPage != 295 {
		t.Errorf("maxPage = %d, want 295", maxPage)
	}

	c0 := cards[0]
	if c0.id != "4851" {
		t.Errorf("c0.id = %q", c0.id)
	}
	if c0.url != "/tour/model/4851/Leya_Falcon_in_Lesbian_Asylum_Lust.html" {
		t.Errorf("c0.url = %q", c0.url)
	}
	if c0.thumb != "https://c776ef2f9b.mjedge.net/pspthumbnails/31459.jpg" {
		t.Errorf("c0.thumb = %q", c0.thumb)
	}
	if c0.title != "Leya Falcon in Lesbian Asylum Lust" {
		t.Errorf("c0.title = %q", c0.title)
	}
	if c0.performer != "Leya Falcon" {
		t.Errorf("c0.performer = %q", c0.performer)
	}
	want := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	if !c0.date.Equal(want) {
		t.Errorf("c0.date = %v, want %v", c0.date, want)
	}
	if c0.views != 2041 {
		t.Errorf("c0.views = %d, want 2041", c0.views)
	}
	if c0.likes != 2384 {
		t.Errorf("c0.likes = %d, want 2384", c0.likes)
	}

	c1 := cards[1]
	if c1.id != "3011" || c1.performer != "Savana Styles" {
		t.Errorf("c1 = %+v", c1)
	}
}

func TestToScene(t *testing.T) {
	s := New()
	cards, _ := parseCards([]byte(fixturePage))
	sc := s.toScene(cards[0], "https://pornstarplatinum.com/", time.Now())
	if sc.ID != "4851" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Studio != "Pornstar Platinum" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Series != "Leya Falcon" {
		t.Errorf("Series = %q (should carry performer for downstream per-model filtering)", sc.Series)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Leya Falcon" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if !strings.HasPrefix(sc.URL, "https://pornstarplatinum.com/tour/model/4851/") {
		t.Errorf("URL = %q", sc.URL)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		// Parent
		{"https://pornstarplatinum.com/", true},
		{"https://www.pornstarplatinum.com/tour/scenes.php", true},
		// Stashdb-tracked sister tours
		{"https://tour.alurajensonxxx.com/", true},
		{"https://amybrookexxx.com/", true},
		{"http://angelinavalentine.com/", true},
		{"https://avadevine.com/", true},
		{"https://www.clubveronicaavluv.com/", true},
		{"https://tour.deewilliams.xxx/", true},
		{"https://tour.deewilliams.xxx/index.php", true},
		{"http://nickiblue.com/", true},
		{"https://tour.pornstarjustice.com/", true},
		{"https://www.pornstarjustice.com/", true},
		{"https://sexyvanessa.com/", true},
		{"https://taboostepmom.com/", true},
		// Network-page extras
		{"https://tour.kendralustxxx.com/", true},
		{"https://joslynjames.xxx/", true},
		{"https://gigiriveraxxx.com/", true},
		// Negatives
		{"https://annaclaireclouds.site/", false}, // not actually PSP
		{"https://ariellaferrera.com/", false},    // DNS-dead, dropped
		{"https://pornstarplatinumfake.com/", false},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestResolveFilter(t *testing.T) {
	cases := []struct {
		url       string
		performer string
		ok        bool
	}{
		// Parent — no filter, returns whole catalogue
		{"https://pornstarplatinum.com/", "", true},
		{"https://www.pornstarplatinum.com/tour/scenes.php?page=5", "", true},
		// Single-performer sister tours
		{"https://tour.deewilliams.xxx/index.php", "Dee Williams", true},
		{"https://tour.alurajensonxxx.com/", "Alura Jenson", true},
		{"https://nickiblue.com/", "Nicki Blue", true},
		{"https://sexyvanessa.com/", "Sexy Vanessa", true},
		{"https://www.clubveronicaavluv.com/", "Veronica Avluv", true},
		{"https://tour.kendralustxxx.com/", "Kendra Lust", true},
		// Themed brands — empty filter
		{"https://taboostepmom.com/", "", true},
		{"https://tour.pornstarjustice.com/", "", true},
		// Unknown URLs
		{"https://example.com/", "", false},
	}
	for _, c := range cases {
		got, ok := resolveFilter(c.url)
		if got != c.performer || ok != c.ok {
			t.Errorf("resolveFilter(%q) = (%q, %v), want (%q, %v)", c.url, got, ok, c.performer, c.ok)
		}
	}
}

func TestSiteFilterTable(t *testing.T) {
	// Every entry has a regex; performer can be empty.
	for i, sf := range sites {
		if sf.matchRe == nil {
			t.Errorf("sites[%d]: nil matchRe", i)
		}
	}
	// Spot-check that no two sister-site entries have the same performer
	// (would be a config bug — two distinct slugs shouldn't both filter
	// for the same name).
	seen := map[string]int{}
	for i, sf := range sites {
		if sf.Performer == "" {
			continue
		}
		if prev, ok := seen[sf.Performer]; ok {
			t.Errorf("performer %q duplicated in sites[%d] and sites[%d]", sf.Performer, prev, i)
		}
		seen[sf.Performer] = i
	}
}
