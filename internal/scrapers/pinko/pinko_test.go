package pinko

import (
	"testing"
)

func tgScraper() *Scraper {
	return New(SiteConfig{SiteID: "pinkotgirls", Domain: "pinkotgirls.com", Base: "https://www.pinkotgirls.com", StudioName: "Pinko TGirls", DetailPrefix: "/videotrans/"})
}

func pcScraper() *Scraper {
	return New(SiteConfig{SiteID: "pinkoclub", Domain: "pinkoclub.com", Base: "https://www.pinkoclub.com", StudioName: "Pinko Club", DetailPrefix: "/video-porno-italiani/"})
}

// Real markup captured from https://www.pinkotgirls.com/new-video.php?next=1
const tgListingFixture = `
<div class="box-home">
<a class="link-photo-home" href="/videotrans/7818-amanda-si-fa-filmare-mentre-viene-trapanata-e-gode.php" title="Amanda Gets Filmed While Getting Pounded and Loving It"><img width="100%" src="https://img.pinkocdn.com/image/MHM001/16_9.jpg" alt="Amanda Gets Filmed While Getting Pounded and Loving It" class="photo-home"></a>
<h4><a href="/videotrans/7818-amanda-si-fa-filmare-mentre-viene-trapanata-e-gode.php" title="Amanda Gets Filmed While Getting Pounded and Loving It">Amanda Gets Filmed While Getting Pounded and Loving It</a></h4>
</div>
<div class="box-home">
<a class="link-photo-home" href="/videotrans/7812-nicole-e-thaysa.php" title="Nicole &amp; Thaysa: mutual anal"><img width="100%" src="https://img.pinkocdn.com/image/JAZZ050/899-16_9.jpg" alt="x" class="photo-home"></a>
<h4><a href="/videotrans/7812-nicole-e-thaysa.php" title="Nicole &amp; Thaysa: mutual anal">Nicole &amp; Thaysa</a></h4>
</div>
<div class="lazy"><img src="https://img.pinkocdn.com/image//" alt=""></div>
`

// Real markup captured from https://www.pinkoclub.com/new-video.php?next=1
const pcListingFixture = `
<a class="link-photo-home" href="/video-porno-italiani/7515-deep-inside-katy-caro--.php" title="Deep Inside Katy Caro !!! "><img width="100%" src="https://img.pinkocdn.com/image/000721-2014-P-SC3/image_34.jpg" alt="Deep Inside Katy Caro !!! " class="photo-home"></a>
`

// Real markup captured from a /videotrans/ detail page.
const tgDetailFixture = `
<meta property="og:title" content="Amanda Gets Filmed While Getting Pounded and Loving It" />
<meta property="og:description" content="Stunning Amanda drives her partner wild. Intense finale." />
<meta property="og:image" content="https://img.pinkocdn.com/image/MHM001/16_9.jpg" />
<div class="titolo-video">
	<h2>Amanda Gets Filmed While Getting Pounded and Loving It</h2>
	<h4><a href="/trans-star/amanda-araujo.php">Amanda Araujo</a>, <a href="/trans-star/paulo-machy.php">Paulo Machy</a></h4>
	<div class="caption captionvideo">Some caption text.</div>
</div>
<script type="application/ld+json">{"@type":"VideoObject","name":"Summer Brook","author":"primalfetishnetwork"}</script>
`

// Real markup captured from a /video-porno-italiani/ detail page (cast uses /pornostar/).
const pcDetailFixture = `
<meta property="og:title" content="Deep Inside Katy Caro !!! " />
<meta property="og:description" content="Andrea Nobili directs Katy Caro." />
<meta property="og:image" content="https://img.pinkocdn.com/image/000721-2014-P-SC3/image_34.jpg" />
<div class="titolo-video">
	<h2>Deep Inside Katy Caro !!! </h2>
	<h4><a href="/pornostar/katy-caro.php">Katy Caro</a>, <a href="/pornostar/omar-will.php">Omar Will</a></h4>
	<div class="caption captionvideo">desc</div>
</div>
`

func TestListURL(t *testing.T) {
	s := tgScraper()
	if got, want := s.listURL(1), "https://www.pinkotgirls.com/new-video.php?next=1"; got != want {
		t.Errorf("listURL(1) = %q, want %q", got, want)
	}
	if got, want := s.listURL(7), "https://www.pinkotgirls.com/new-video.php?next=7"; got != want {
		t.Errorf("listURL(7) = %q, want %q", got, want)
	}
	if got, want := pcScraper().listURL(2), "https://www.pinkoclub.com/new-video.php?next=2"; got != want {
		t.Errorf("pc listURL(2) = %q, want %q", got, want)
	}
}

func TestParseListingTGirls(t *testing.T) {
	cards := tgScraper().parseListing([]byte(tgListingFixture))
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2: %+v", len(cards), cards)
	}
	c0 := cards[0]
	if c0.id != "7818" {
		t.Errorf("id = %q, want 7818", c0.id)
	}
	if c0.url != "https://www.pinkotgirls.com/videotrans/7818-amanda-si-fa-filmare-mentre-viene-trapanata-e-gode.php" {
		t.Errorf("url = %q", c0.url)
	}
	if c0.title != "Amanda Gets Filmed While Getting Pounded and Loving It" {
		t.Errorf("title = %q", c0.title)
	}
	if c0.thumb != "https://img.pinkocdn.com/image/MHM001/16_9.jpg" {
		t.Errorf("thumb = %q", c0.thumb)
	}
	// HTML entity in title attribute must be unescaped.
	if cards[1].id != "7812" || cards[1].title != "Nicole & Thaysa: mutual anal" {
		t.Errorf("card[1] = %+v", cards[1])
	}
}

func TestParseListingClub(t *testing.T) {
	cards := pcScraper().parseListing([]byte(pcListingFixture))
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	if cards[0].id != "7515" {
		t.Errorf("id = %q, want 7515", cards[0].id)
	}
	if cards[0].url != "https://www.pinkoclub.com/video-porno-italiani/7515-deep-inside-katy-caro--.php" {
		t.Errorf("url = %q", cards[0].url)
	}
	if cards[0].thumb != "https://img.pinkocdn.com/image/000721-2014-P-SC3/image_34.jpg" {
		t.Errorf("thumb = %q", cards[0].thumb)
	}
}

func TestParseDetailTGirls(t *testing.T) {
	d := parseDetail([]byte(tgDetailFixture))
	if d.title != "Amanda Gets Filmed While Getting Pounded and Loving It" {
		t.Errorf("title = %q", d.title)
	}
	if d.description != "Stunning Amanda drives her partner wild. Intense finale." {
		t.Errorf("description = %q", d.description)
	}
	if d.image != "https://img.pinkocdn.com/image/MHM001/16_9.jpg" {
		t.Errorf("image = %q", d.image)
	}
	if len(d.performers) != 2 || d.performers[0] != "Amanda Araujo" || d.performers[1] != "Paulo Machy" {
		t.Errorf("performers = %v", d.performers)
	}
}

func TestParseDetailClub(t *testing.T) {
	d := parseDetail([]byte(pcDetailFixture))
	if d.title != "Deep Inside Katy Caro !!!" {
		t.Errorf("title = %q", d.title)
	}
	if len(d.performers) != 2 || d.performers[0] != "Katy Caro" || d.performers[1] != "Omar Will" {
		t.Errorf("performers = %v", d.performers)
	}
}

func TestMatchesURL(t *testing.T) {
	tg := tgScraper()
	pc := pcScraper()
	cases := []struct {
		s    *Scraper
		url  string
		want bool
	}{
		{tg, "https://www.pinkotgirls.com/new-video.php", true},
		{tg, "https://pinkotgirls.com/", true},
		{tg, "https://www.pinkoclub.com/new-video.php", false},
		{pc, "https://www.pinkoclub.com/new-video.php?next=1", true},
		{pc, "https://www.pinkotgirls.com/", false},
		{tg, "https://example.com/", false},
	}
	for _, c := range cases {
		if got := c.s.MatchesURL(c.url); got != c.want {
			t.Errorf("%s MatchesURL(%q) = %v, want %v", c.s.ID(), c.url, got, c.want)
		}
	}
}
