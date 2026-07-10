package meanworld

import "testing"

const listingFixture = `
<div class="latestUpdateB" data-setid="2981">
	<div class="videoPic">
		<a href="https://megasite.meanworld.com/scenes/Taylor-Rae-Slave-Orders_vids.html">
			<video poster_1x="/content//contentthumbs/95/17/49517-1x.jpg" src="x.mp4"></video>
		</a>
	</div>
	<div class="latestUpdateBinfo">
		<a href="https://megasite.meanworld.com/scenes/Taylor-Rae-Slave-Orders_vids.html"> Taylor Rae Slave Orders </a>
		<a href="https://megasite.meanworld.com/models/Taylor-Rae.html">Taylor Rae</a>
		<div class="buttons_light buyProduct">Buy ($10.99)</div>
		<script>var p={"buy":[{"Id":13,"InternalLabel":"Slave Orders Section: $10.99","FullPrice":"10.99"}]}</script>
	</div>
</div>
<div class="latestUpdateB" data-setid="2980">
	<a href="/scenes/Another-Scene_vids.html"> Another Scene </a>
	<a href="/models/Jane-Doe.html">Jane Doe</a>
	<div>Buy ($8.99)</div>
	<script>"InternalLabel":"Mean Bitches Section: $8.99"</script>
</div>
<div class="pagination">Page 1 of 225</div>
`

const detailFixture = `
<title>MeanBitches Megasite - Taylor Rae Slave Orders - Movies</title>
<meta name="description" content="Beautiful blonde Goddess Taylor Rae welcomes you." />
<ul class="videoInfo">
	<li><span><i class="fa-calendar"></i></span><!-- Date --> 06/26/2026</li>
	<li><span><i class="fa-camera"></i></span> 98</li>
	<li><span><i class="fa-video"></i></span> 8 min</li>
</ul>
`

func TestParseListing(t *testing.T) {
	scenes := parseListing([]byte(listingFixture))
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}
	s0 := scenes[0]
	if s0.slug != "Taylor-Rae-Slave-Orders" {
		t.Errorf("slug = %q", s0.slug)
	}
	if s0.url != "https://megasite.meanworld.com/scenes/Taylor-Rae-Slave-Orders_vids.html" {
		t.Errorf("url = %q", s0.url)
	}
	if s0.title != "Taylor Rae Slave Orders" {
		t.Errorf("title = %q", s0.title)
	}
	if len(s0.performers) != 1 || s0.performers[0] != "Taylor Rae" {
		t.Errorf("performers = %v", s0.performers)
	}
	if s0.section != "Slave Orders" {
		t.Errorf("section = %q, want Slave Orders", s0.section)
	}
	if s0.price != 10.99 {
		t.Errorf("price = %v, want 10.99", s0.price)
	}
	if s0.thumb != "https://megasite.meanworld.com/content//contentthumbs/95/17/49517-1x.jpg" {
		t.Errorf("thumb = %q", s0.thumb)
	}

	s1 := scenes[1]
	if s1.url != "https://megasite.meanworld.com/scenes/Another-Scene_vids.html" {
		t.Errorf("relative url = %q", s1.url)
	}
	if s1.section != "Mean Bitches" {
		t.Errorf("section = %q", s1.section)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))
	if d.date.Format("2006-01-02") != "2026-06-26" {
		t.Errorf("date = %v", d.date)
	}
	if d.duration != 8*60 {
		t.Errorf("duration = %d, want %d", d.duration, 8*60)
	}
	if d.description == "" {
		t.Errorf("description should be set")
	}
}

func TestEstimateTotal(t *testing.T) {
	if got := estimateTotal([]byte("Page 1 of 225")); got != 225*perPage {
		t.Errorf("estimateTotal = %d", got)
	}
}

func TestListBase(t *testing.T) {
	cases := map[string]string{
		"https://megasite.meanworld.com/categories/movies_1_d.html":             "https://megasite.meanworld.com",
		"https://megasite.meanworld.com/slaveorders/categories/movies_1_d.html": "https://megasite.meanworld.com/slaveorders",
		"https://megasite.meanworld.com/":                                       "https://megasite.meanworld.com",
	}
	for in, want := range cases {
		if got := listBase(in); got != want {
			t.Errorf("listBase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://megasite.meanworld.com/":                           true,
		"https://megasite.meanworld.com/categories/movies_1_d.html": true,
		"https://example.com/meanworld":                             false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}
