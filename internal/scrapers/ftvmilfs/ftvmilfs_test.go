package ftvmilfs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/scrapers/ftvutil"
	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://ftvmilfs.com/modelssfw.html", true},
		{"https://www.ftvmilfs.com/updates.html", true},
		{"https://ftvmilfs.com/", true},
		{"https://ftvmilfs.com/update/serene-610.html", true},
		{"https://example.com/ftvmilfs", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
	}{
		{"Apr 28, 2026", time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)},
		{"Apr  7, 2026", time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)},
		{"Jan 24, 2015", time.Date(2015, 1, 24, 0, 0, 0, 0, time.UTC)},
		{"Feb  7, 2017", time.Date(2017, 2, 7, 0, 0, 0, 0, time.UTC)},
		{"", time.Time{}},
	}
	for _, c := range cases {
		if got := ftvutil.ParseDate(c.input); !got.Equal(c.want) {
			t.Errorf("ParseDate(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestParseListingPage(t *testing.T) {
	body := []byte(`
<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2>Serene</h2></div>
<div class="UpdateDate"><h3>Apr 28, 2026</h3></div>
</div>
<div id="SecondColumn" class="cf">
<div class="S2C1 cf">
<div class="VideoTime"><img alt="" /><h3>64 mins</h3></div>
<div class="Pictures"><img alt="" /><h3>250 pics</h3></div>
</div>
<div class="S2C2 cf">
<div class="Tags cf">
<img src="updatesCategories/1st.png" title="First Time Experience - Never been in adult before." alt="" />
<img src="updatesCategories/bb.png" title="Busty Girl - Big, natural breasts." alt="" />
</div>
</div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/join.html"><img class="ModelPhotoWide" src="https://cdn.test/serene-tour-610.jpg" alt="" /></a>
</div>
<div class="ModelBio">
<div class="Bio">
<p>Serene is back on FTV!</p>
</div>
</div>
</div>
</div><!-- ModelContainer -->
<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2>Melanie</h2></div>
<div class="UpdateDate"><h3>Apr 21, 2026</h3></div>
</div>
<div id="SecondColumn" class="cf">
<div class="S2C1 cf">
<div class="VideoTime"><img alt="" /><h3>74 mins</h3></div>
</div>
<div class="S2C2 cf">
<div class="Tags cf">
<img src="updatesCategories/dil.png" title="Dildo Play - Adult toys used here." alt="" />
</div>
</div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/join.html"><img class="ModelPhotoWide" src="https://cdn.test/melanie-tour-609.jpg" alt="" /></a>
</div>
<div class="ModelBio">
<div class="Bio">
<p>Melanie returns to FTV!</p>
</div>
</div>
</div>
</div><!-- ModelContainer -->`)

	entries := ftvutil.ParseListingPage(body)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.ID != "610" {
		t.Errorf("id = %q, want 610", e.ID)
	}
	if e.Name != "Serene" {
		t.Errorf("name = %q", e.Name)
	}
	wantDate := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	if !e.Date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", e.Date, wantDate)
	}
	if e.Duration != 3840 {
		t.Errorf("duration = %d, want 3840", e.Duration)
	}
	if len(e.Tags) != 2 || e.Tags[0] != "First Time Experience" || e.Tags[1] != "Busty Girl" {
		t.Errorf("tags = %v", e.Tags)
	}
	if e.Thumb != "https://cdn.test/serene-tour-610.jpg" {
		t.Errorf("thumb = %q", e.Thumb)
	}
	if e.Desc != "Serene is back on FTV!" {
		t.Errorf("desc = %q", e.Desc)
	}

	e2 := entries[1]
	if e2.ID != "609" {
		t.Errorf("id = %q, want 609", e2.ID)
	}
	if e2.Name != "Melanie" {
		t.Errorf("name = %q", e2.Name)
	}
	if e2.Duration != 4440 {
		t.Errorf("duration = %d, want 4440", e2.Duration)
	}
}

func TestParseDetailPage(t *testing.T) {
	body := []byte(`
<title>Serene on FTVMilfs.com Released Apr 28, 2026!</title>
<a class="jackbox" data-title="<b>Name: </b><span>Serene</span> <b>Age: </b><span>28</span> <b>Figure: </b><span>34D-26-34</span> <b>Release date: </b><span>Apr 28, 2026</span>" href="trailer.mp4">
<img id="VideoSample" src="vid.jpg" alt="" /></a>
<div id="BioHeader">
<h1>Serene's Statistics</h1>
<h2><b>Age:</b> 28<span class="separator"> | </span> <b>Height:</b> 5'4" <span class="separator"> | </span><b>Figure:</b> 34D-26-34</h2>
</div>
<div class="OneHeader" id="Bio">
<p>The day starts with gorgeous Serene.</p>
</div>
<div id="MagazineContainer"><img id="Magazine" src="https://cdn.test/serene-touru-610.jpg" alt="" /></div>`)

	d := ftvutil.ParseDetailPage(body)
	if d.Name != "Serene" {
		t.Errorf("name = %q", d.Name)
	}
	if d.Age != 28 {
		t.Errorf("age = %d", d.Age)
	}
	if d.Figure != "34D-26-34" {
		t.Errorf("figure = %q", d.Figure)
	}
	if d.Height != `5'4"` {
		t.Errorf("height = %q", d.Height)
	}
	wantDate := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	if !d.Date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", d.Date, wantDate)
	}
	if d.Desc != "The day starts with gorgeous Serene." {
		t.Errorf("desc = %q", d.Desc)
	}
	if d.Thumb != "https://cdn.test/serene-touru-610.jpg" {
		t.Errorf("thumb = %q", d.Thumb)
	}
}

func TestParseDetailPageTitleFallback(t *testing.T) {
	body := []byte(`<title>Luna on FTVMilfs.com Released Apr 14, 2026!</title>
<div class="OneHeader" id="Bio"><p>Luna description.</p></div>`)

	d := ftvutil.ParseDetailPage(body)
	if d.Name != "Luna" {
		t.Errorf("name = %q", d.Name)
	}
	wantDate := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	if !d.Date.Equal(wantDate) {
		t.Errorf("date = %v", d.Date)
	}
}

func TestParseDetailPageEmpty(t *testing.T) {
	d := ftvutil.ParseDetailPage([]byte(`<html><body></body></html>`))
	if d.Name != "" || d.Desc != "" {
		t.Errorf("expected empty detail, got %+v", d)
	}
}

const detailTpl = `<title>Model %d on FTVMilfs.com Released Jan 15, 2026!</title>
<a class="jackbox" data-title="<b>Name: </b><span>Model %d</span> <b>Age: </b><span>25</span> <b>Figure: </b><span>34C-25-35</span> <b>Release date: </b><span>Jan 15, 2026</span>" href="t.mp4"></a>
<div id="BioHeader"><h1>Model %d's Statistics</h1>
<h2><b>Age:</b> 25<span class="separator"> | </span> <b>Height:</b> 5'6"</h2></div>
<div class="OneHeader" id="Bio"><p>Description for model %d.</p></div>
<div id="MagazineContainer"><img id="Magazine" src="https://cdn.test/model-%d.jpg" /></div>`

func buildUpdatesPage(ids []int) []byte {
	var containers string
	for _, id := range ids {
		containers += fmt.Sprintf(`<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2>Model %d</h2></div>
<div class="UpdateDate"><h3>Jan 15, 2026</h3></div>
</div>
<div id="SecondColumn" class="cf">
<div class="S2C1 cf">
<div class="VideoTime"><img /><h3>30 mins</h3></div>
</div>
<div class="S2C2 cf">
<div class="Tags cf">
<img src="updatesCategories/0.png" title="Real Orgasms - desc" alt="" />
</div>
</div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/join.html"><img class="ModelPhotoWide" src="https://cdn.test/m-tour-%d.jpg" /></a>
</div>
<div class="ModelBio"><div class="Bio"><p>Bio %d.</p></div></div>
</div>
</div><!-- ModelContainer -->`, id, id, id)
	}
	return []byte(containers)
}

func newTestServer(sceneIDs []int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch r.URL.Path {
		case "/updates.html":
			_, _ = w.Write(buildUpdatesPage(sceneIDs))
		default:
			var id int
			_, _ = fmt.Sscanf(r.URL.Path, "/update/x-%d.html", &id)
			if id >= 1 && id <= sceneIDs[0] {
				_, _ = fmt.Fprintf(w, detailTpl, id, id, id, id, id)
			} else {
				_, _ = fmt.Fprint(w, `<html><body></body></html>`)
			}
		}
	}))
}

func TestListScenes(t *testing.T) {
	ts := newTestServer([]int{3, 2, 1})
	defer ts.Close()

	s := ftvutil.New(ftvutil.SiteConfig{SiteID: "ftvmilfs", Domain: "ftvmilfs.com", Studio: "FTV MILFs", TitleSite: "FTVMilfs.com"})
	s.Client = ts.Client()
	s.Base = ts.URL
	ch, err := s.ListScenes(context.Background(), ts.URL+"/updates.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 3 {
		t.Fatalf("got %d scenes, want 3", len(results))
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := newTestServer([]int{5, 4, 3, 2, 1})
	defer ts.Close()

	s := ftvutil.New(ftvutil.SiteConfig{SiteID: "ftvmilfs", Domain: "ftvmilfs.com", Studio: "FTV MILFs", TitleSite: "FTVMilfs.com"})
	s.Client = ts.Client()
	s.Base = ts.URL
	ch, err := s.ListScenes(context.Background(), ts.URL+"/updates.html", scraper.ListOpts{
		KnownIDs: map[string]bool{"3": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}
}

func TestListScenesEnrichment(t *testing.T) {
	ts := newTestServer([]int{3, 2, 1})
	defer ts.Close()

	s := ftvutil.New(ftvutil.SiteConfig{SiteID: "ftvmilfs", Domain: "ftvmilfs.com", Studio: "FTV MILFs", TitleSite: "FTVMilfs.com"})
	s.Client = ts.Client()
	s.Base = ts.URL
	ch, err := s.ListScenes(context.Background(), ts.URL+"/updates.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)

	enriched := 0
	for _, sc := range results {
		if sc.Duration > 0 {
			enriched++
		}
	}
	if enriched != 3 {
		t.Errorf("got %d enriched scenes, want 3 (all IDs are on the listing)", enriched)
	}
}
