package glamosetour

import (
	"testing"
)

const fixtureCard = `<div class="row"><div class="itemmain col-md-3 col-sm-6 col-xs-12">
    <div class="itemv">
        <div class="newmodel2-"></div>
        <a href="refstat.php?lid=10827&sid=84" onmouseover="window.status='Roxy Mendez'; return true" class="bloc-link">
            <div class="images">
                <img src="image_resize.php?i=faceimages/GI080626_PA_Roxy_1080.jpg&w=550&h=300&stretching=fill&tok=abc123" alt="" />
            </div>
        </a>
        <div class="nm-name">
            <p>Roxy Mendez</p>
            <span class="truncate"> <br />Tags: <a href="javascript:search('4K', '');">4K</a>, <a href="javascript:search('Bare__Legs', '');">Bare Legs</a> </span>
            Added: June 8, 2026
            <span>4.09mins</span>
        </div>
    </div>
<div class="clearfix"></div></div>
<div class="itemmain col-md-3 col-sm-6 col-xs-12">
    <div class="itemv">
        <div class="newmodel2-"></div>
        <a href="refstat.php?lid=10826&sid=84" onmouseover="window.status='O&#039;Hara'; return true" class="bloc-link">
            <div class="images">
                <img src="faceimages/GI060626_PA_OHara.jpg" alt="" />
            </div>
        </a>
        <div class="nm-name">
            <p>O'Hara</p>
            <span class="truncate"> <br />Tags: <a href="javascript:search('Brunettes', '');">Brunettes</a> </span>
            Added: June 6, 2026
            <span>5.12mins</span>
        </div>
    </div>
<div class="clearfix"></div></div></div>`

func TestParseListingPage(t *testing.T) {
	s := New(SiteConfig{SiteID: "pantyamateur", Domain: "pantyamateur.com", StudioName: "Panty Amateur"})
	scenes := s.parseListingPage([]byte(fixtureCard), "https://www.pantyamateur.com/")

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != "10827" {
		t.Errorf("ID = %q, want 10827", sc.ID)
	}
	if sc.Title != "Roxy Mendez" {
		t.Errorf("Title = %q", sc.Title)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Roxy Mendez" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Date.Format("2006-01-02") != "2026-06-08" {
		t.Errorf("Date = %v", sc.Date)
	}
	if sc.Duration != 4*60+9 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 4*60+9)
	}
	if sc.Thumbnail != "https://www.pantyamateur.com/faceimages/GI080626_PA_Roxy_1080.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if len(sc.Tags) < 2 || sc.Tags[0] != "4K" || sc.Tags[1] != "Bare Legs" {
		t.Errorf("Tags = %v", sc.Tags)
	}

	sc2 := scenes[1]
	if sc2.ID != "10826" {
		t.Errorf("ID = %q", sc2.ID)
	}
	if sc2.Title != "O'Hara" {
		t.Errorf("Title = %q (should unescape HTML entities)", sc2.Title)
	}
	if sc2.Duration != 5*60+12 {
		t.Errorf("Duration = %d, want %d", sc2.Duration, 5*60+12)
	}
	if sc2.Thumbnail != "https://www.pantyamateur.com/faceimages/GI060626_PA_OHara.jpg" {
		t.Errorf("Thumbnail = %q", sc2.Thumbnail)
	}
}

func TestSitesTable(t *testing.T) {
	seen := map[string]bool{}
	for _, cfg := range sites {
		if cfg.SiteID == "" {
			t.Error("empty SiteID")
		}
		if seen[cfg.SiteID] {
			t.Errorf("duplicate SiteID: %q", cfg.SiteID)
		}
		seen[cfg.SiteID] = true
	}
	if len(sites) != 12 {
		t.Errorf("expected 12 sites, got %d", len(sites))
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(SiteConfig{SiteID: "pantyamateur", Domain: "pantyamateur.com", StudioName: "Panty Amateur"})
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.pantyamateur.com/", true},
		{"https://pantyamateur.com/?videos", true},
		{"https://example.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
