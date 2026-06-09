package masutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const plumperPassCard = `<!-- start_link -->
                <div class="col-sm-6 col-md-6 col-lg-4 col-xl-3 vidblock">
	<a href="refstat.php?lid=16384&sid=584&uvar=MC4wLjAuMC4wLjAuMC4wLjA" onmouseover="window.status='BBW Facesitting Mayhem'; return true" onmouseout="window.status=' '; return true"><img src="faceimages/GI2497bbwd_Minnie_Mayhem.jpg" class="img-fluid" /></a>
						<div class="itemminfo">
							<p class="vidname"><a href="refstat.php?lid=16384&sid=584&uvar=MC4wLjAuMC4wLjAuMC4wLjA">BBW Facesitting Mayhem</a><br /><a href="refstat.php?lid=2524&sid=584&uvar=MC4wLjAuMC4wLjAuMC4wLjA">Minnie Mayhem</a></p>
							<p class="date">June 8, 2026<br /><a href="https://join.plumperpass.com/signup/signup.php">Watch Full Scene Instantly!</a></p>
						</div>
                </div>
<!-- end_link -->`

const standardCard = `<!-- start_link -->
					<div class="itemm">
						<a href="refstat.php?lid=16382&sid=787&uvar=MC4wLjAuMC4wLjAuMC4wLjA" onmouseover="window.status='Katt Little in &quot;Kat\'s Cabana Pounding&quot;'; return true" onmouseout="window.status=' '; return true"><img src="image_resize.php?i=faceimages/GI4579pp.Katt_Little.jpg&w=500&h=330&stretching=fill&tok=289847d3949203b2" /></a>
						<div class="itemminfo">
							<h3><a href="refstat.php?lid=16092&sid=787&uvar=MC4wLjAuMC4wLjAuMC4wLjA">Kat Little</a></h3>
                            <p>Kat's Cabana Pounding</p>
							<p class="date">June 5, 2026</p>
						</div>
					</div>
                 	<!-- end_link -->`

const bbwlandCard = `<!-- start_link -->
						<a href="refstat.php?lid=16385&sid=809&uvar=MC4wLjY0LjEyNy4wLjAuMC4wLjA" onmouseover="window.status='Sierra Skye Gets Spread'; return true" onmouseout="window.status=' '; return true">
<div class="hs-wrapper">
<img src="image_resize.php?i=faceimages/GI011780577096.jpg&w=291&h=194&stretching=fill&tok=3ddf7898e1eec5f7" alt="image01"/>
					<img src="content/BL/731bl/02.jpg" alt="image02"/>
</div>
</a>
						<div class="itemminfo">
							<!-- <h3></h3> -->
                            <p>Sierra Skye Gets Spread</p>
							<!-- <p class="date">June 7, 2026</p> -->
						</div>
<!-- end_link -->`

func TestParseCardsPlumperPass(t *testing.T) {
	cards := ParseCards(plumperPassCard)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	c := cards[0]
	if c.LID != "16384" {
		t.Errorf("LID = %q, want 16384", c.LID)
	}
	if c.Title != "BBW Facesitting Mayhem" {
		t.Errorf("Title = %q, want %q", c.Title, "BBW Facesitting Mayhem")
	}
	if len(c.Performers) != 1 || c.Performers[0] != "Minnie Mayhem" {
		t.Errorf("Performers = %v, want [Minnie Mayhem]", c.Performers)
	}
	if c.Date != "June 8, 2026" {
		t.Errorf("Date = %q, want %q", c.Date, "June 8, 2026")
	}
	if c.Thumbnail != "faceimages/GI2497bbwd_Minnie_Mayhem.jpg" {
		t.Errorf("Thumbnail = %q", c.Thumbnail)
	}
}

func TestParseCardsStandard(t *testing.T) {
	cards := ParseCards(standardCard)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	c := cards[0]
	if c.LID != "16382" {
		t.Errorf("LID = %q, want 16382", c.LID)
	}
	if c.Title != "Kat's Cabana Pounding" {
		t.Errorf("Title = %q, want %q", c.Title, "Kat's Cabana Pounding")
	}
	if len(c.Performers) != 1 || c.Performers[0] != "Kat Little" {
		t.Errorf("Performers = %v, want [Kat Little]", c.Performers)
	}
	if c.Date != "June 5, 2026" {
		t.Errorf("Date = %q, want %q", c.Date, "June 5, 2026")
	}
	if c.Thumbnail != "image_resize.php?i=faceimages/GI4579pp.Katt_Little.jpg&w=500&h=330&stretching=fill&tok=289847d3949203b2" {
		t.Errorf("Thumbnail = %q", c.Thumbnail)
	}
}

func TestParseCardsBBWLand(t *testing.T) {
	cards := ParseCards(bbwlandCard)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	c := cards[0]
	if c.LID != "16385" {
		t.Errorf("LID = %q, want 16385", c.LID)
	}
	if c.Title != "Sierra Skye Gets Spread" {
		t.Errorf("Title = %q, want %q", c.Title, "Sierra Skye Gets Spread")
	}
	if len(c.Performers) != 0 {
		t.Errorf("Performers = %v, want empty", c.Performers)
	}
	if c.Date != "June 7, 2026" {
		t.Errorf("Date = %q, want %q", c.Date, "June 7, 2026")
	}
}

func TestParseCardsMultiple(t *testing.T) {
	body := plumperPassCard + "\n" + standardCard
	cards := ParseCards(body)
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	if cards[0].LID != "16384" {
		t.Errorf("card[0].LID = %q, want 16384", cards[0].LID)
	}
	if cards[1].LID != "16382" {
		t.Errorf("card[1].LID = %q, want 16382", cards[1].LID)
	}
}

func TestExtractMaxPage(t *testing.T) {
	body := `<a href="javascript:void(0);" class="select pagenumbers_selected current">1</a> <a href="show.php?a=584_2" class="pagenumbers">2</a> <a href="show.php?a=584_3" class="pagenumbers">3</a> <a href="show.php?a=584_162" class="pagenumbers">162</a>`
	got := ExtractMaxPage(body)
	if got != 162 {
		t.Errorf("ExtractMaxPage = %d, want 162", got)
	}
}

func TestExtractMaxPageSelect(t *testing.T) {
	body := `<SELECT name="MAS_pages"><option value=1 selected>1</option><option value=2>2</option><option value=27>27</option></SELECT>`
	got := ExtractMaxPage(body)
	if got != 27 {
		t.Errorf("ExtractMaxPage = %d, want 27", got)
	}
}

func TestExtractMaxPageNone(t *testing.T) {
	got := ExtractMaxPage("<html><body>no pagination</body></html>")
	if got != 0 {
		t.Errorf("ExtractMaxPage = %d, want 0", got)
	}
}

func TestBuildPageURL(t *testing.T) {
	got := buildPageURL("https://www.plumperpass.com", "584", 1)
	want := "https://www.plumperpass.com/index.php?videos&a=584_1"
	if got != want {
		t.Errorf("page 1: got %q, want %q", got, want)
	}
	got = buildPageURL("https://www.plumperpass.com", "584", 5)
	want = "https://www.plumperpass.com/index.php?videos&a=584_5"
	if got != want {
		t.Errorf("page 5: got %q, want %q", got, want)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New(SiteConfig{
		SiteID: "plumperpass",
		Domain: "plumperpass.com",
		PageID: "584",
		Base:   "https://www.plumperpass.com",
	})
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.plumperpass.com", true},
		{"https://plumperpass.com", true},
		{"https://www.plumperpass.com/?videos", true},
		{"https://www.bbwland.com", false},
		{"https://example.com", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestToScene(t *testing.T) {
	cfg := SiteConfig{
		SiteID: "plumperpass",
		Domain: "plumperpass.com",
		PageID: "584",
		Base:   "https://www.plumperpass.com",
	}
	card := CardData{
		LID:        "16384",
		Title:      "BBW Facesitting Mayhem",
		Date:       "June 8, 2026",
		Performers: []string{"Minnie Mayhem"},
		Thumbnail:  "faceimages/GI2497bbwd.jpg",
	}
	sc := toScene(cfg, card, "https://www.plumperpass.com", fixedTime)
	if sc.ID != "16384" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.SiteID != "plumperpass" {
		t.Errorf("SiteID = %q", sc.SiteID)
	}
	if sc.URL != "https://www.plumperpass.com/t1/refstat.php?lid=16384&sid=584" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != "https://www.plumperpass.com/faceimages/GI2497bbwd.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Date.IsZero() || sc.Date.Format("2006-01-02") != "2026-06-08" {
		t.Errorf("Date = %v", sc.Date)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Minnie Mayhem" {
		t.Errorf("Performers = %v", sc.Performers)
	}
}

var fixedTime = func() time.Time { t, _ := time.Parse(time.RFC3339, "2026-06-09T00:00:00Z"); return t }()

func TestFetchPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.php":
			_, _ = fmt.Fprint(w, plumperPassCard)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New(SiteConfig{SiteID: "test", Domain: "example.com", PageID: "584", Base: ts.URL})
	s.Client = ts.Client()

	body, err := s.fetchPage(t.Context(), ts.URL+"/index.php?videos&a=584_1")
	if err != nil {
		t.Fatal(err)
	}
	cards := ParseCards(body)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	if cards[0].LID != "16384" {
		t.Errorf("LID = %q", cards[0].LID)
	}
}

func TestHTMLEntitiesInTitle(t *testing.T) {
	body := `<!-- start_link -->
<div class="itemm">
	<a href="refstat.php?lid=100&sid=787"><img src="faceimages/test.jpg" /></a>
	<div class="itemminfo">
		<h3><a href="refstat.php?lid=50&sid=787">Performer</a></h3>
		<p>Leather &amp; Lust: Behind the Moans</p>
		<p class="date">May 21, 2026</p>
	</div>
</div>
<!-- end_link -->`

	cards := ParseCards(body)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	if cards[0].Title != "Leather & Lust: Behind the Moans" {
		t.Errorf("Title = %q, want %q", cards[0].Title, "Leather & Lust: Behind the Moans")
	}
}
