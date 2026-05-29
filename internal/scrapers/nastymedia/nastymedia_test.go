package nastymedia

import (
	"testing"
	"time"
)

const fixtureHOME = `<!doctype html>
<html><head><meta name="generator" content="WYSIWYG Web Builder 18"></head>
<body>
   <div id="container">
      <div id="wb_Card8" style="position:absolute;left:0px;top:1008px;width:328px;height:276px;z-index:10;" class="card">
         <div id="Card8-card-body">
            <a href="https://www.coozhound.com/GET_ROAST.html"><img id="Card8-card-item0" src="images/Snapshot_2187.PNG" width="1920" height="1080" alt="" title=""></a>
            <div id="Card8-card-item1">NASTY MILF CREAMPIED</div>
            <hr id="Card8-card-item2">
            <div id="Card8-card-item3">Ms. Cum Get Some gets smoked</div>
         </div>
      </div>
      <div id="wb_Card5" style="position:absolute;left:669px;top:1118px;width:328px;height:276px;z-index:13;" class="card">
         <div id="Card5-card-body">
            <a href="https://www.coozhound.com/DIAMOND_STARR.html"><img id="Card5-card-item0" src="https://www.coozhound.com/images/Snapshot_2872.PNG" width="1920" height="1080" alt="" title=""></a>
            <div id="Card5-card-item1">UPDATE FEBRUARY 15TH, 2025</div>
            <hr id="Card5-card-item2">
            <div id="Card5-card-item3">21 y/o 1st bbc pack out</div>
         </div>
      </div>
      <div id="wb_Card9" style="position:absolute;left:0px;top:2000px;width:328px;height:276px;z-index:14;" class="card">
         <div id="Card9-card-body">
            <a href="https://www.coozhound.com/DIAMOND_STARR.html"><img id="Card9-card-item0" src="images/dup.PNG" width="1920" height="1080" alt="" title=""></a>
            <div id="Card9-card-item1">already seen — should be deduped</div>
            <hr id="Card9-card-item2">
            <div id="Card9-card-item3">duplicate description</div>
         </div>
      </div>
      <div id="wb_Card11" style="position:absolute;left:0px;top:3000px;width:328px;height:276px;z-index:15;" class="card">
         <div id="Card11-card-body">
            <a href="https://www.coozhound.com/RELLIE.html"><img id="Card11-card-item0" src="images/rellie.JPG" width="1920" height="1080" alt="" title=""></a>
            <div id="Card11-card-item1">UPDATE :&nbsp; MAY 15TH , 2026</div>
            <hr id="Card11-card-item2">
            <div id="Card11-card-item3">crazy double creampie 3 some</div>
         </div>
      </div>
</div>
</body></html>`

func TestParseCards(t *testing.T) {
	got := parseCards([]byte(fixtureHOME), "https://www.coozhound.com")
	if len(got) != 3 {
		t.Fatalf("got %d cards, want 3 (the dup should be dropped)", len(got))
	}

	// First card: no parseable date in header — title falls back to
	// item3 description.
	c1 := got[0]
	if c1.slug != "GET_ROAST" {
		t.Errorf("c1.slug = %q", c1.slug)
	}
	if c1.url != "https://www.coozhound.com/GET_ROAST.html" {
		t.Errorf("c1.url = %q", c1.url)
	}
	if c1.thumb != "https://www.coozhound.com/images/Snapshot_2187.PNG" {
		t.Errorf("c1.thumb = %q", c1.thumb)
	}
	if c1.header != "NASTY MILF CREAMPIED" {
		t.Errorf("c1.header = %q", c1.header)
	}
	if c1.desc != "Ms. Cum Get Some gets smoked" {
		t.Errorf("c1.desc = %q", c1.desc)
	}
	if c1.dateOK {
		t.Errorf("c1 should have no date, got %v", c1.date)
	}

	// Second card: header is a date, item3 is the description.
	c2 := got[1]
	if c2.slug != "DIAMOND_STARR" {
		t.Errorf("c2.slug = %q", c2.slug)
	}
	if c2.thumb != "https://www.coozhound.com/images/Snapshot_2872.PNG" {
		t.Errorf("c2.thumb = %q (absolute URL should be left alone)", c2.thumb)
	}
	if !c2.dateOK {
		t.Errorf("c2 should have parsed date")
	} else if c2.date != time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC) {
		t.Errorf("c2.date = %v", c2.date)
	}

	// Third unique card: the spaced-out NBSP date variant.
	c3 := got[2]
	if c3.slug != "RELLIE" {
		t.Errorf("c3.slug = %q", c3.slug)
	}
	if !c3.dateOK || c3.date != time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) {
		t.Errorf("c3 date = %v ok=%v", c3.date, c3.dateOK)
	}
}

func TestToScene(t *testing.T) {
	s := New(SiteConfig{ID: "coozhound", BaseURL: "https://www.coozhound.com", SiteName: "CoozHound"})
	cards := parseCards([]byte(fixtureHOME), "https://www.coozhound.com")
	scene := s.toScene(cards[0], "https://www.coozhound.com/HOME.html", time.Now())
	if scene.ID != "GET_ROAST" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Studio != "Nasty Media Group" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.Series != "CoozHound" {
		t.Errorf("Series = %q", scene.Series)
	}
	// Title prefers item3 (descriptive) over item1 (which here looks
	// like a short title without a date marker).
	if scene.Title != "Ms. Cum Get Some gets smoked" {
		t.Errorf("Title = %q", scene.Title)
	}
	// When item3 doubles as the title, Description should NOT also
	// carry the same string (avoid duplication).
	if scene.Description == scene.Title {
		t.Errorf("Description duplicated Title: %q", scene.Description)
	}
}

func TestParseMonthDay(t *testing.T) {
	cases := []struct {
		month, day, year string
		want             time.Time
		ok               bool
	}{
		{"FEBRUARY", "15", "2025", time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC), true},
		{"may", "1", "2026", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), true},
		{"Jan", "30", "2024", time.Date(2024, 1, 30, 0, 0, 0, 0, time.UTC), true},
		{"BOGUS", "5", "2024", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := parseMonthDay(c.month, c.day, c.year)
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("parseMonthDay(%q,%q,%q) = %v ok=%v; want %v ok=%v", c.month, c.day, c.year, got, ok, c.want, c.ok)
		}
	}
}

func TestAbsoluteURL(t *testing.T) {
	base := "https://www.coozhound.com"
	cases := []struct{ in, want string }{
		{"images/x.png", base + "/images/x.png"},
		{"/images/x.png", base + "/images/x.png"},
		{"https://other/x.png", "https://other/x.png"},
		{"//cdn.example/x.png", "https://cdn.example/x.png"},
		{"", ""},
	}
	for _, c := range cases {
		if got := absoluteURL(c.in, base); got != c.want {
			t.Errorf("absoluteURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHumanizeSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"RELLIE", "Rellie"},
		{"DIAMOND_STARR", "Diamond Starr"},
		{"JEWELZ_2025", "Jewelz 2025"},
		{"", ""},
	}
	for _, c := range cases {
		if got := humanizeSlug(c.in); got != c.want {
			t.Errorf("humanizeSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSitesTable(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if seen[cfg.ID] {
			t.Errorf("duplicate ID: %q", cfg.ID)
		}
		seen[cfg.ID] = true
		if cfg.BaseURL == "" {
			t.Errorf("site %q missing BaseURL", cfg.ID)
		}
		if cfg.MatchRe == nil {
			t.Errorf("site %q missing MatchRe", cfg.ID)
		}
	}
	if len(sites) != 4 {
		t.Errorf("expected 4 sites, got %d", len(sites))
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
		{"coozhound", "https://www.coozhound.com/", true},
		{"coozhound", "http://coozhound.com/HOME.html", true},
		{"coozhound", "https://www.urbanamateurs.net/", false},
		{"urbanamateurs", "https://www.urbanamateurs.net/HOME.html", true},
		{"urbanamateurs", "https://urbanamateurs.net/", true},
		{"nastynyamateurs", "https://nastynyamateurs.com/HOME.html", true},
		{"msnympho", "http://www.msnympho.com/HOME.html", true},
		// Hostname-substring trap.
		{"coozhound", "https://coozhoundfake.com/", false},
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
