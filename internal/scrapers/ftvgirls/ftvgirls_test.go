package ftvgirls

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
		{"https://ftvgirls.com/updates.html", true},
		{"https://www.ftvgirls.com/updates.html", true},
		{"https://ftvgirls.com/", true},
		{"https://ftvgirls.com/update/aubree-2466.html", true},
		{"https://example.com/ftvgirls", false},
		{"https://ftvmilfs.com/updates.html", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseListingPageHrefID(t *testing.T) {
	body := []byte(`
<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2>Aubree</h2></div>
<div class="UpdateDate"><h3>May  7, 2026</h3></div>
</div>
<div id="SecondColumn" class="cf">
<div class="S2C1 cf">
<div class="VideoTime"><img alt="" /><h3>89 mins</h3></div>
</div>
<div class="S2C2 cf">
<div class="Tags cf">
<img src="updatesCategories/1st.png" title="First Time - New to adult." alt="" />
</div>
</div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/update/aubree-2466.html"><img class="ModelPhotoWide" src="https://cdn.ftvgirls.com/aubree.jpg" alt="" /></a>
</div>
<div class="ModelBio">
<div class="Bio">
<p>Aubree arrives on FTV!</p>
</div>
</div>
</div>
</div><!-- ModelContainer -->
<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2>Kenna</h2></div>
<div class="UpdateDate"><h3>Apr 30, 2026</h3></div>
</div>
<div id="SecondColumn" class="cf">
<div class="S2C1 cf">
<div class="VideoTime"><img alt="" /><h3>56 mins</h3></div>
</div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/update/kenna-2465.html"><img class="ModelPhotoWide" src="https://cdn.ftvgirls.com/kenna.jpg" alt="" /></a>
</div>
<div class="ModelBio">
<div class="Bio">
<p>Kenna returns!</p>
</div>
</div>
</div>
</div><!-- ModelContainer -->`)

	entries := ftvutil.ParseListingPage(body)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	e := entries[0]
	if e.ID != "2466" {
		t.Errorf("id = %q, want 2466", e.ID)
	}
	if e.Name != "Aubree" {
		t.Errorf("name = %q", e.Name)
	}
	wantDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	if !e.Date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", e.Date, wantDate)
	}
	if e.Duration != 5340 {
		t.Errorf("duration = %d, want 5340", e.Duration)
	}
	if e.Desc != "Aubree arrives on FTV!" {
		t.Errorf("desc = %q", e.Desc)
	}

	e2 := entries[1]
	if e2.ID != "2465" {
		t.Errorf("id = %q, want 2465", e2.ID)
	}
}

func TestParseDetailPageFTVGirls(t *testing.T) {
	body := []byte(`
<title>Aubree on FTVGirls.com Released May  7, 2026! - Real Girls, Real Adventure!</title>
<a class="jackbox" data-title="<b>Name: </b><span>Aubree</span> <b>Age: </b><span>19</span> <b>Figure: </b><span>32B-24-34</span> <b>Release date: </b><span>May  7, 2026</span>" href="trailer.mp4">
<img id="VideoSample" src="vid.jpg" alt="" /></a>
<div id="BioHeader">
<h1>Aubree's Statistics</h1>
<h2><b>Age:</b> 19<span class="separator"> | </span> <b>Height:</b> 5'5" <span class="separator"> | </span><b>Figure:</b> 32B-24-34</h2>
</div>
<div class="OneHeader" id="Bio">
<p>Meet Aubree on FTV Girls.</p>
</div>
<div id="MagazineContainer"><img id="Magazine" src="https://cdn.ftvgirls.com/aubree-mag.jpg" alt="" /></div>`)

	d := ftvutil.ParseDetailPage(body)
	if d.Name != "Aubree" {
		t.Errorf("name = %q", d.Name)
	}
	if d.Age != 19 {
		t.Errorf("age = %d", d.Age)
	}
	if d.Figure != "32B-24-34" {
		t.Errorf("figure = %q", d.Figure)
	}
	if d.Height != `5'5"` {
		t.Errorf("height = %q", d.Height)
	}
	wantDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	if !d.Date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", d.Date, wantDate)
	}
	if d.Thumb != "https://cdn.ftvgirls.com/aubree-mag.jpg" {
		t.Errorf("thumb = %q", d.Thumb)
	}
}

const detailTpl = `<title>Model %d on FTVGirls.com Released Jan 15, 2026! - Real Girls, Real Adventure!</title>
<a class="jackbox" data-title="<b>Name: </b><span>Model %d</span> <b>Age: </b><span>21</span> <b>Figure: </b><span>34B-25-35</span> <b>Release date: </b><span>Jan 15, 2026</span>" href="t.mp4"></a>
<div id="BioHeader"><h1>Model %d's Statistics</h1>
<h2><b>Age:</b> 21<span class="separator"> | </span> <b>Height:</b> 5'6"</h2></div>
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
<a href="/update/model-%d.html"><img class="ModelPhotoWide" src="https://cdn.test/m-%d.jpg" /></a>
</div>
<div class="ModelBio"><div class="Bio"><p>Bio %d.</p></div></div>
</div>
</div><!-- ModelContainer -->`, id, id, id, id)
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

	s := &ftvutil.Scraper{
		Cfg:    ftvutil.SiteConfig{SiteID: "ftvgirls", Domain: "ftvgirls.com", Studio: "FTV Girls", TitleSite: "FTVGirls.com"},
		Client: ts.Client(),
		Base:   ts.URL,
	}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/updates.html", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 3 {
		t.Fatalf("got %d scenes, want 3", len(results))
	}
	if results[0].Studio != "FTV Girls" {
		t.Errorf("studio = %q, want FTV Girls", results[0].Studio)
	}
	if results[0].SiteID != "ftvgirls" {
		t.Errorf("siteID = %q, want ftvgirls", results[0].SiteID)
	}
}

func TestListScenesKnownIDs(t *testing.T) {
	ts := newTestServer([]int{5, 4, 3, 2, 1})
	defer ts.Close()

	s := &ftvutil.Scraper{
		Cfg:    ftvutil.SiteConfig{SiteID: "ftvgirls", Domain: "ftvgirls.com", Studio: "FTV Girls", TitleSite: "FTVGirls.com"},
		Client: ts.Client(),
		Base:   ts.URL,
	}
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

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}
