package feetondemand

import (
	"testing"
	"time"
)

const fixtureOuter = `<html><head></head><body>
<script type="text/javascript">
$(document).ready(function(){$("#mainbody").load("content/pages/abc123def456.list.htm");});
</script>
<a href="/index.php?mb=VmlkZW9zfHw=&amp;p=0">1</a>
<a href="/index.php?mb=VmlkZW9zfHw=&amp;p=20">2</a>
<a href="/index.php?mb=VmlkZW9zfHw=&amp;p=40">3</a>
<a href="/index.php?mb=VmlkZW9zfHw=&amp;p=1260">Last</a>
</body></html>`

const fixtureInner = `<div class='row'>
<div class='col-lg-3 col-md-6 col-sm-12 img-portfolio'>
    <a href='#' data-toggle='modal' data-target='#pop_h3k3b9s9l9v2g2'>
        <img class='img-responsive thumbvideo' src='https://www.goddessfootdomination.com/content/art/videos/h3k3b9s9l9v2g2.jpg' alt='Stalker Ex&#039;s Feet Cum Fest'>
    </a>
    <h4 style='margin-top:6px;'>
        <a href='#' data-toggle='modal' data-target='#pop_h3k3b9s9l9v2g2'>Stalker Ex&#039;s Feet Cum Fest</a>
    </h4>
    <p style='margin-top:-6px;'><strong>Model: </strong><a href="https://www.goddessfootdomination.com/?page=Models&id=mod1230EL4634539">Adara Jordin</a><br></p>
</div>
<div class='col-lg-3 col-md-6 col-sm-12 img-portfolio'>
    <a href='#' data-toggle='modal' data-target='#pop_k2j6e7l1s5n5p6'>
        <img class='img-responsive thumbvideo' src='https://www.goddessfootdomination.com/content/art/videos/k2j6e7l1s5n5p6.jpg' alt='Second Scene'>
    </a>
    <h4>
        <a href='#' data-toggle='modal' data-target='#pop_k2j6e7l1s5n5p6'>Second Scene</a>
    </h4>
    <p><strong>Model: </strong><a href="https://www.goddessfootdomination.com/?page=Models&amp;id=mod1184EL4700750">Brianna Beach</a></p>
</div>
</div>`

func TestParseAjaxURL(t *testing.T) {
	got, ok := parseAjaxURL([]byte(fixtureOuter))
	if !ok {
		t.Fatal("parseAjaxURL: ok=false on valid input")
	}
	if got != "content/pages/abc123def456.list.htm" {
		t.Errorf("ajax URL = %q", got)
	}

	// Outer without a .load() call → not supported template.
	_, ok = parseAjaxURL([]byte(`<html>nothing</html>`))
	if ok {
		t.Error("parseAjaxURL: ok=true on input without .load() call")
	}
}

func TestParseMaxOffset(t *testing.T) {
	if got := parseMaxOffset([]byte(fixtureOuter)); got != 1260 {
		t.Errorf("max offset = %d, want 1260", got)
	}
	if got := parseMaxOffset([]byte(`<html>no pagination</html>`)); got != 0 {
		t.Errorf("empty page max offset = %d, want 0", got)
	}
}

func TestParseInnerCards(t *testing.T) {
	cards := parseInnerCards([]byte(fixtureInner))
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}

	c0 := cards[0]
	if c0.id != "h3k3b9s9l9v2g2" {
		t.Errorf("c0.id = %q", c0.id)
	}
	if c0.title != "Stalker Ex's Feet Cum Fest" {
		t.Errorf("c0.title = %q (entity unescape failed?)", c0.title)
	}
	if c0.thumb != "https://www.goddessfootdomination.com/content/art/videos/h3k3b9s9l9v2g2.jpg" {
		t.Errorf("c0.thumb = %q", c0.thumb)
	}
	if c0.performer != "Adara Jordin" {
		t.Errorf("c0.performer = %q", c0.performer)
	}

	c1 := cards[1]
	if c1.id != "k2j6e7l1s5n5p6" || c1.performer != "Brianna Beach" {
		t.Errorf("c1 = %+v", c1)
	}
}

func TestParseInnerCards_dedupes(t *testing.T) {
	doubled := fixtureInner + fixtureInner
	cards := parseInnerCards([]byte(doubled))
	if len(cards) != 2 {
		t.Errorf("got %d cards after dedup, want 2", len(cards))
	}
}

func TestToScene(t *testing.T) {
	s := New(SiteConfig{ID: "goddessfootdomination", BaseURL: "https://www.goddessfootdomination.com", SiteName: "Goddess Foot Domination"})
	cards := parseInnerCards([]byte(fixtureInner))
	sc := s.toScene(cards[0], "https://www.goddessfootdomination.com/", time.Now())
	if sc.ID != "h3k3b9s9l9v2g2" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Studio != "Feet on Demand" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Series != "Goddess Foot Domination" {
		t.Errorf("Series = %q", sc.Series)
	}
	if sc.URL != "https://www.goddessfootdomination.com/?mb=VmlkZW9zfHw=#pop_h3k3b9s9l9v2g2" {
		t.Errorf("URL = %q", sc.URL)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Adara Jordin" {
		t.Errorf("Performers = %v", sc.Performers)
	}
}

func TestSitesTable(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if cfg.ID == "" {
			t.Errorf("empty ID")
		}
		if seen[cfg.ID] {
			t.Errorf("duplicate ID: %q", cfg.ID)
		}
		seen[cfg.ID] = true
		if cfg.BaseURL == "" || cfg.SiteName == "" || cfg.MatchRe == nil {
			t.Errorf("site %q has empty config: %+v", cfg.ID, cfg)
		}
	}
	if len(sites) != 5 {
		t.Errorf("expected 5 sites, got %d", len(sites))
	}
}

func TestMatchesURL(t *testing.T) {
	get := func(id string) *Scraper {
		for _, cfg := range sites {
			if cfg.ID == id {
				return New(cfg)
			}
		}
		return nil
	}
	cases := []struct {
		id, url string
		want    bool
	}{
		{"goddessfootdomination", "https://www.goddessfootdomination.com/", true},
		{"goddessfootdomination", "https://goddessfootdomination.com/index.php?mb=VmlkZW9zfHw=&p=1260", true},
		{"goddessfootdomination", "https://www.jerktomyfeet.com/", false},
		{"jerktomyfeet", "https://www.jerktomyfeet.com/?mb=VmlkZW9zfHw=", true},
		{"footfetishcardates", "https://www.footfetishcardates.com/", true},
		{"footfetishaffiliates", "https://footfetishaffiliates.com/", true},
		{"goddessbrianna", "https://www.goddessbrianna.net/index.php?mb=VmlkZW9zfHw=&p=435", true},
		// Substring traps
		{"goddessfootdomination", "https://goddessfootdominationfake.com/", false},
	}
	for _, c := range cases {
		s := get(c.id)
		if s == nil {
			t.Fatalf("unknown ID %q", c.id)
		}
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL[%s](%q) = %v, want %v", c.id, c.url, got, c.want)
		}
	}
}
