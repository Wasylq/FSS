package jerkoffinstructions

import (
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://jerkoffinstructions.com/", true},
		{"https://jerkoffinstructions.com", true},
		{"https://www.jerkoffinstructions.com/videos/796/", true},
		{"https://jerkoffinstructions.com/tour.php?p=1", true},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

const fixtureSingle = `
<div class="curvy">
<span id="ctl"><b>&bull;</b></span>
<span id="cbl"><b>&bull;</b></span>
<span id="ctr"><b>&bull;</b></span>
<span id="cbr"><b>&bull;</b></span>
<table border="0" cellspacing="0" cellpadding="5" class="form" style="margin:0px;" width="100%">
<tr>
<td colspan="2">
<div class="title">1500 Video(s) Found for: <span style="font-style:italic;">All Videos</span></div>
<hr style="margin:10px 0px 35px 0px;">
</td>
</tr>
<tr>
<td valign="top" width="300">
<a href="/videos/796/"><img src="/cover_images/JOI1_Simone_Sonay_720mp4.gif" class="photo" width="285" height="171"></a>
<div class="keywords">
<a href="/keywords/20">Big Tits</a>, <a href="/keywords/11">Blondes</a>, <a href="/keywords/96">Friend&#039;s Mom</a>, <a href="/keywords/39">Milf</a></div>
</td>
<td valign="top">
<div class="title">Jacking off for horny widow Simone Sonay</div>
<p>You are staying with your friend for a little while and your friend's step-mom is happy to have you.&nbsp;<a href="/videos/796/">more</a></p>
Date Added: 04/23/2026<br>
Starring: <a href="/models/281">Simone Sonay</a><br>
Company: Liberated Eye Inc<br>
Running Time: 11  mins<br><br>
</td>
</tr>
</table>
</div>
`

const fixtureMultiPerf = `
<div class="curvy">
<span id="ctl"><b>&bull;</b></span>
<span id="cbl"><b>&bull;</b></span>
<span id="ctr"><b>&bull;</b></span>
<span id="cbr"><b>&bull;</b></span>
<table border="0" cellspacing="0" cellpadding="5" class="form" style="margin:0px;" width="100%">
<tr>
<td valign="top" width="300">
<a href="/videos/500/"><img src="/cover_images/scene500.gif" class="photo" width="285" height="171"></a>
<div class="keywords">
<a href="/keywords/39">Milf</a></div>
</td>
<td valign="top">
<div class="title">Double trouble</div>
<p>Two hotties team up.&nbsp;<a href="/videos/500/">more</a></p>
Date Added: 06/15/2020<br>
Starring: <a href="/models/255">Ariella Ferrera</a> and <a href="/models/149">Zoey Holloway</a><br>
Company: Liberated Eye Inc<br>
Running Time: 15  mins<br><br>
</td>
</tr>
</table>
</div>
`

func TestParseListingPage(t *testing.T) {
	cards, total := parseListingPage([]byte(fixtureSingle))
	if total != 1500 {
		t.Errorf("total = %d, want 1500", total)
	}
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}

	c := cards[0]
	if c.id != "796" {
		t.Errorf("id = %q", c.id)
	}
	if c.url != siteBase+"/videos/796/" {
		t.Errorf("url = %q", c.url)
	}
	if c.title != "Jacking off for horny widow Simone Sonay" {
		t.Errorf("title = %q", c.title)
	}
	if c.thumbnail != siteBase+"/cover_images/JOI1_Simone_Sonay_720mp4.gif" {
		t.Errorf("thumbnail = %q", c.thumbnail)
	}
	if c.date.Year() != 2026 || c.date.Month() != 4 || c.date.Day() != 23 {
		t.Errorf("date = %v", c.date)
	}
	if len(c.performers) != 1 || c.performers[0] != "Simone Sonay" {
		t.Errorf("performers = %v", c.performers)
	}
	if c.duration != 11*60 {
		t.Errorf("duration = %d, want %d", c.duration, 11*60)
	}
	if len(c.tags) != 4 || c.tags[2] != "Friend's Mom" {
		t.Errorf("tags = %v", c.tags)
	}
	if c.description == "" {
		t.Error("description is empty")
	}
}

func TestParseMultiPerformer(t *testing.T) {
	cards, _ := parseListingPage([]byte(fixtureMultiPerf))
	if len(cards) != 1 {
		t.Fatalf("got %d cards", len(cards))
	}
	c := cards[0]
	if len(c.performers) != 2 || c.performers[0] != "Ariella Ferrera" || c.performers[1] != "Zoey Holloway" {
		t.Errorf("performers = %v", c.performers)
	}
}

func TestParseEmptyPage(t *testing.T) {
	cards, total := parseListingPage([]byte(`<html></html>`))
	if total != 0 {
		t.Errorf("total = %d", total)
	}
	if len(cards) != 0 {
		t.Errorf("cards = %d", len(cards))
	}
}

func TestBuildScene(t *testing.T) {
	c := listingCard{
		id:          "796",
		url:         siteBase + "/videos/796/",
		title:       "Test Scene",
		description: "A test description.",
		thumbnail:   siteBase + "/cover_images/test.gif",
		date:        time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC),
		duration:    660,
		performers:  []string{"Simone Sonay"},
		tags:        []string{"Milf", "Blondes"},
	}
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	scene := buildScene(c, now)

	if scene.ID != "796" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.Studio != "Jerk Off Instructions" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.SiteID != "jerkoffinstructions" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Duration != 660 {
		t.Errorf("Duration = %d", scene.Duration)
	}
	if len(scene.Tags) != 2 {
		t.Errorf("Tags = %v", scene.Tags)
	}
	if scene.Description != "A test description." {
		t.Errorf("Description = %q", scene.Description)
	}
}

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}
