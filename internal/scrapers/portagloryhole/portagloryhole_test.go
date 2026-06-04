package portagloryhole

import (
	"testing"
	"time"
)

const fixtureCard = `
<div class="xlabs_list_item results_container" id="results_1">
<div class="post_item video" data-post-id="3949">
    <a href="/join" class="add_favorite " data-xlabs-bookmark="post, 3949">
        <i class="fas fa-heart"></i>
    </a>
    <div class="post_video">
        <a class="preview" href="/join" title="Kat M in the Porta Gloryhole for the First Time"
           data-media-poster="https://c7421dcd81.mjedge.net/media/americancumdolls/studios/portagloryhole/videos/18112017000000/f73667589c9bd1d677c0515beac20303.jpeg"
           data-media-mp4="">
            <i class="fas fa-play"></i>
            <img class="_image item_cover" src="https://c7421dcd81.mjedge.net/media/americancumdolls/studios/portagloryhole/videos/18112017000000/f73667589c9bd1d677c0515beac20303.jpeg"/>
        </a>
    </div>
    <h3>
        <span class="posted_on">Nov 19, 2025</span>
        <span class="counter_photos"><i class="fas fa-video"></i> 38:07</span>
    </h3>
    <h1>
        <a href="/join" title="Kat M in the Porta Gloryhole for the First Time">Kat M in the Porta Gloryhole for the First Time</a>
    </h1>
    <h2>
        <a href="/join" title="Kat">Kat</a>
    </h2>
</div>
<div class="post_item video" data-post-id="3821">
    <div class="post_video">
        <a class="preview" href="/join" title="Sophia&#039;s Gloryhole Adventure"
           data-media-poster="https://c7421dcd81.mjedge.net/media/americancumdolls/studios/portagloryhole/videos/22082017000000/thumb.jpeg"
           data-media-mp4="">
            <img class="_image item_cover" src="https://c7421dcd81.mjedge.net/media/americancumdolls/studios/portagloryhole/videos/22082017000000/thumb.jpeg"/>
        </a>
    </div>
    <h3>
        <span class="posted_on">Mar 4, 2016</span>
        <span class="counter_photos"><i class="fas fa-video"></i> 22:15</span>
    </h3>
    <h1>
        <a href="/join" title="Sophia&#039;s Gloryhole Adventure">Sophia&#039;s Gloryhole Adventure</a>
    </h1>
    <h2>
        <a href="/join" title="Sophia">Sophia</a>
    </h2>
</div>
<div class="pagination_wrapper ">
<a href="#" data-page="2" title="last">2</a>
</div>
</div>
`

func TestParseListing(t *testing.T) {
	cards, totalPages := parseListing([]byte(fixtureCard))
	if totalPages != 2 {
		t.Errorf("totalPages = %d, want 2", totalPages)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}

	c := cards[0]
	if c.id != "3949" {
		t.Errorf("id = %q", c.id)
	}
	if c.title != "Kat M in the Porta Gloryhole for the First Time" {
		t.Errorf("title = %q", c.title)
	}
	if c.performer != "Kat" {
		t.Errorf("performer = %q", c.performer)
	}
	if c.date != time.Date(2025, 11, 19, 0, 0, 0, 0, time.UTC) {
		t.Errorf("date = %v", c.date)
	}
	if c.duration != 38*60+7 {
		t.Errorf("duration = %d, want %d", c.duration, 38*60+7)
	}
	if c.thumbnail != "https://c7421dcd81.mjedge.net/media/americancumdolls/studios/portagloryhole/videos/18112017000000/f73667589c9bd1d677c0515beac20303.jpeg" {
		t.Errorf("thumbnail = %q", c.thumbnail)
	}

	c2 := cards[1]
	if c2.id != "3821" {
		t.Errorf("card2 id = %q", c2.id)
	}
	if c2.title != "Sophia's Gloryhole Adventure" {
		t.Errorf("card2 title = %q", c2.title)
	}
	if c2.performer != "Sophia" {
		t.Errorf("card2 performer = %q", c2.performer)
	}
	if c2.date != time.Date(2016, 3, 4, 0, 0, 0, 0, time.UTC) {
		t.Errorf("card2 date = %v", c2.date)
	}
}

func TestToScene(t *testing.T) {
	c := listingCard{
		id:        "3949",
		title:     "Test Scene",
		performer: "Model A",
		duration:  600,
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	scene := c.toScene("https://www.portagloryhole.com/", now)

	if scene.ID != "3949" {
		t.Errorf("ID = %q", scene.ID)
	}
	if scene.SiteID != "portagloryhole" {
		t.Errorf("SiteID = %q", scene.SiteID)
	}
	if scene.Studio != "Porta Gloryhole" {
		t.Errorf("Studio = %q", scene.Studio)
	}
	if scene.URL != "https://www.portagloryhole.com/post/details/3949" {
		t.Errorf("URL = %q", scene.URL)
	}
	if len(scene.Performers) != 1 || scene.Performers[0] != "Model A" {
		t.Errorf("Performers = %v", scene.Performers)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.portagloryhole.com/", true},
		{"https://portagloryhole.com/", true},
		{"http://www.portagloryhole.com/anything", true},
		{"https://www.example.com/", false},
		{"https://portagloryholefake.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
