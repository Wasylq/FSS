package mplstudios

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return b
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.mplstudios.com/", true},
		{"https://www.mplstudios.com/videos/", true},
		{"https://mplstudios.com/videos/3/", true},
		{"https://www.mplstudios.com/portfolio/290-Karissa_Diamond/", true},
		{"http://mplstudios.com/update/6928v-Unplugged/", true},
		{"https://karissa-diamond.com/", false},
		{"https://example.com/mplstudios.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestParseCardsListing ----

func TestParseCardsListing(t *testing.T) {
	cards := parseCards(readFixture(t, "listing.html"), false)
	if len(cards) != 4 {
		t.Fatalf("got %d cards, want 4", len(cards))
	}

	got := cards[0]
	want := card{
		id:           "7104v2",
		url:          "https://www.mplstudios.com/update/7104v2-Edging_On_Ecstasy_2/",
		title:        "Edging On Ecstasy 2",
		cover:        "https://cdn.mplstudios.com/v3Assets/videoCovers/290/99e17be4/001.jpg",
		date:         time.Date(2025, time.January, 19, 0, 0, 0, 0, time.UTC),
		model:        "Karissa Diamond",
		photographer: "Bobby",
	}
	if got != want {
		t.Errorf("card[0] =\n %+v\nwant\n %+v", got, want)
	}

	// Cards must stay in listing order (newest first) so KnownIDs early-stop
	// in Paginate sees the freshest scenes before the known ones.
	wantIDs := []string{"7104v2", "7104v1", "6928v", "6956v2"}
	for i, id := range wantIDs {
		if cards[i].id != id {
			t.Errorf("card[%d].id = %q, want %q", i, cards[i].id, id)
		}
	}
}

// ---- TestParseCardsPortfolioFiltersPhotoSets ----

func TestParseCardsPortfolioFiltersPhotoSets(t *testing.T) {
	body := readFixture(t, "portfolio.html")

	// The portfolio fixture holds 4 cards: 2 photo sets and 2 videos.
	if all := parseCards(body, false); len(all) != 4 {
		t.Fatalf("unfiltered: got %d cards, want 4", len(all))
	}

	videos := parseCards(body, true)
	if len(videos) != 2 {
		t.Fatalf("videosOnly: got %d cards, want 2", len(videos))
	}
	for _, c := range videos {
		if !strings.Contains(c.id, "v") {
			t.Errorf("card %q does not look like a video update", c.id)
		}
	}
	if videos[0].id != "7104v2" || videos[1].id != "7104v1" {
		t.Errorf("got ids %q,%q; want 7104v2,7104v1", videos[0].id, videos[1].id)
	}
}

// ---- TestParseCardsIgnoresScriptTemplate ----

// The live pages embed a JS infinite-scroll template that reproduces the card
// markup verbatim. It must not parse as a card.
func TestParseCardsIgnoresScriptTemplate(t *testing.T) {
	body := []byte(`<div id="updateGrid">
	<div class="text-center mb-3 box1" data-id="1001v">
		<a href="/update/1001v-Real_One/"><img class="stdCover img-fluid" src="https://cdn/x.jpg"></a>
		<div><span class="ellipsis">Mar 3, 2020</span></div>
		<div><span class="ellipsis"><a href="/portfolio/12-Ann/">Ann</a></span></div>
		<div><span class="ellipsis" title="Real One">'Real One'</span></div>
	</div>
</div>
<script>
	$( '#grid' ).append( '<div class="text-center mb-3 box1" data-date="' + this[6] + '"><a href="/update/' + this[0] + '/"></a></div>' );
</script>`)

	cards := parseCards(body, false)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1 (script template must be stripped)", len(cards))
	}
	if cards[0].id != "1001v" {
		t.Errorf("id = %q, want 1001v", cards[0].id)
	}
}

// ---- TestParseCardDedupAndFallbackTitle ----

func TestParseCardDedupAndFallbackTitle(t *testing.T) {
	body := []byte(`<div id="updateGrid">
	<div class="text-center mb-3 box1"><a href="/update/2200v-Sun_Kissed_Days/"></a></div>
	<div class="text-center mb-3 box1"><a href="/update/2200v-Sun_Kissed_Days/"></a></div>
	<div class="text-center mb-3 box1"><a href="/nothing/here/"></a></div>
</div>`)

	cards := parseCards(body, false)
	if len(cards) != 1 {
		t.Fatalf("got %d cards, want 1", len(cards))
	}
	// With no title attribute the slug tail is the fallback title.
	if cards[0].title != "Sun Kissed Days" {
		t.Errorf("title = %q, want %q", cards[0].title, "Sun Kissed Days")
	}
	if cards[0].id != "2200v" {
		t.Errorf("id = %q, want 2200v", cards[0].id)
	}
}

// ---- TestRunListing (end-to-end over httptest) ----

func TestRunListing(t *testing.T) {
	listing := readFixture(t, "listing.html")

	// Detail pages are fetched from a worker pool, so the counter must be atomic.
	var detailHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos/1/":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/update/"):
			detailHits.Add(1)
			_, _ = fmt.Fprint(w, `<div>Aug 16, 2024</div><div>Movie Length: 10:09</div>`)
		default:
			// Page 2 is empty, which ends pagination.
			_, _ = fmt.Fprint(w, `<div id="updateGrid"></div>`)
		}
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	scenes, total := collect(t, s, srv.URL+"/videos/")

	if total != 0 {
		t.Errorf("total = %d, want 0 (listing exposes no count)", total)
	}
	if len(scenes) != 4 {
		t.Fatalf("got %d scenes, want 4", len(scenes))
	}
	if got := detailHits.Load(); got != 4 {
		t.Errorf("detail fetches = %d, want 4", got)
	}

	sc := scenes[0]
	if sc.ID != "7104v2" {
		t.Errorf("ID = %q, want 7104v2", sc.ID)
	}
	if sc.SiteID != siteID {
		t.Errorf("SiteID = %q, want %q", sc.SiteID, siteID)
	}
	if sc.Title != "Edging On Ecstasy 2" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Studio != studioName {
		t.Errorf("Studio = %q, want %q", sc.Studio, studioName)
	}
	if sc.Director != "Bobby" {
		t.Errorf("Director = %q, want Bobby", sc.Director)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Karissa Diamond" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Duration != 609 {
		t.Errorf("Duration = %d, want 609", sc.Duration)
	}
	if !sc.Date.Equal(time.Date(2025, time.January, 19, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Date = %v", sc.Date)
	}
	if sc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt is zero")
	}
	if !strings.HasSuffix(sc.URL, "/update/7104v2-Edging_On_Ecstasy_2/") {
		t.Errorf("URL = %q", sc.URL)
	}
}

// ---- TestRunPortfolio ----

func TestRunPortfolio(t *testing.T) {
	portfolio := readFixture(t, "portfolio.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/portfolio/") {
			_, _ = w.Write(portfolio)
			return
		}
		_, _ = fmt.Fprint(w, `<div>Movie Length: 1:02:03</div>`)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	scenes, total := collect(t, s, srv.URL+"/portfolio/290-Karissa_Diamond/")

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (photo sets must be excluded)", len(scenes))
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if scenes[0].Duration != 3723 {
		t.Errorf("Duration = %d, want 3723", scenes[0].Duration)
	}
}

// ---- TestDetailFailureIsNotFatal ----

// A detail page that errors must still yield the scene built from the listing,
// minus duration — the listing already carries every other field.
func TestDetailFailureIsNotFatal(t *testing.T) {
	listing := readFixture(t, "listing.html")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos/1/":
			_, _ = w.Write(listing)
		case strings.HasPrefix(r.URL.Path, "/update/"):
			w.WriteHeader(http.StatusNotFound)
		default:
			_, _ = fmt.Fprint(w, `<div id="updateGrid"></div>`)
		}
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	scenes, _ := collect(t, s, srv.URL+"/videos/")
	if len(scenes) != 4 {
		t.Fatalf("got %d scenes, want 4", len(scenes))
	}
	for _, sc := range scenes {
		if sc.Duration != 0 {
			t.Errorf("scene %s: Duration = %d, want 0", sc.ID, sc.Duration)
		}
		if sc.Title == "" {
			t.Errorf("scene %s: Title is empty", sc.ID)
		}
	}
}

// ---- TestContextCancellation ----

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(readFixture(t, "listing.html"))
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, srv.URL+"/videos/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	cancel()

	// The channel must close rather than block forever.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("channel did not close after context cancellation")
	}
}

// ---- helpers ----

func collect(t *testing.T, s *Scraper, studioURL string) ([]models.Scene, int) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	total := 0
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindTotal:
			total = res.Total
		case scraper.KindError:
			t.Fatalf("scraper error: %v", res.Err)
		}
	}
	return scenes, total
}
