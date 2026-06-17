package allover30

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const fixtureHTML = `<!DOCTYPE html>
<html><body>
<div class="modelInfo">
<div class="modelPic"><img src="https://static.allover30.com/r/rya005/model-thumb.jpg" /></div>
	<h3>Ryan Keely</h3>
	<ul><li><strong>Age :</strong> 35</li></ul>
</div>
<div class="ryt770">
	<div class="modelBox  vid">
		<div class="modelttl">Movie</div>
		<div class="modelP">
			<a href="/Join"><img src="https://static.allover30.com/r/rya005/22098/cover.jpg" /></a>
		</div>
		<ul class="modelPdtls">
			<li><strong>Date added:</strong> Mar 27th, 2020</li>
			<li><strong>Category:</strong> <a href="/Join">Ladies With Toys</a></li>
		</ul>
	</div>

	<div class="modelBox ">
		<div class="modelttl">Photo Set</div>
		<div class="modelP">
			<a href="/Join"><img src="https://static.allover30.com/r/rya005/21622/cover.jpg" /></a>
		</div>
		<ul class="modelPdtls">
			<li><strong>Date added:</strong> Jan 27th, 2020</li>
			<li><strong>Category:</strong> <a href="/Join">Ladies With Toys</a></li>
			<li><strong>Set contains:</strong> 132 images</li>
		</ul>
	</div>

	<div class="modelBox  vid">
		<div class="modelttl">Movie</div>
		<div class="modelP">
			<a href="/Join"><img src="https://static.allover30.com/r/rya005/21889/cover.jpg" /></a>
		</div>
		<ul class="modelPdtls">
			<li><strong>Date added:</strong> Jan 31st, 2020</li>
			<li><strong>Category:</strong> <a href="/Join">Interview</a></li>
		</ul>
	</div>
</div>
</body></html>`

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://new.allover30.com/model-pages/ryan-keely/1549", true},
		{"https://new.allover30.com/model-pages/x/1936", true},
		{"https://allover30.com/model-pages/x/100", true},
		{"https://new.allover30.com/", true},
		{"https://www.pornhub.com/pornstar/foo", false},
		{"https://allover31.com/", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestListScenes_RequiresModelPage(t *testing.T) {
	s := New()
	_, err := s.ListScenes(context.Background(), "https://new.allover30.com/", scraper.ListOpts{})
	if err == nil {
		t.Fatal("expected error for non-model-page URL")
	}
	if got := err.Error(); !strings.Contains(got, "model page URL") {
		t.Errorf("error should mention model page URL, got: %s", got)
	}
}

func TestParsePage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, fixtureHTML)
	}))
	defer ts.Close()

	s := New()
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL+"/model-pages/ryan-keely/1549", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []SceneResult
	for r := range ch {
		scenes = append(scenes, r)
	}

	var movies []SceneResult
	var progress []SceneResult
	for _, r := range scenes {
		switch r.Kind {
		case scraper.KindScene:
			movies = append(movies, r)
		case scraper.KindTotal:
			progress = append(progress, r)
		}
	}

	if len(progress) != 1 {
		t.Fatalf("expected 1 progress result, got %d", len(progress))
	}
	if progress[0].Total != 2 {
		t.Errorf("progress total = %d, want 2", progress[0].Total)
	}

	if len(movies) != 2 {
		t.Fatalf("expected 2 movies (photo set filtered), got %d", len(movies))
	}

	m := movies[0].Scene
	if m.ID != "22098" {
		t.Errorf("ID = %q, want 22098", m.ID)
	}
	if m.Title != "Ryan Keely — Ladies With Toys" {
		t.Errorf("Title = %q, want %q", m.Title, "Ryan Keely — Ladies With Toys")
	}
	if m.SiteID != "allover30" {
		t.Errorf("SiteID = %q, want allover30", m.SiteID)
	}
	if !strings.Contains(m.Thumbnail, "22098/cover.jpg") {
		t.Errorf("Thumbnail = %q, want to contain 22098/cover.jpg", m.Thumbnail)
	}
	wantDate := time.Date(2020, 3, 27, 0, 0, 0, 0, time.UTC)
	if !m.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", m.Date, wantDate)
	}
	if len(m.Performers) != 1 || m.Performers[0] != "Ryan Keely" {
		t.Errorf("Performers = %v, want [Ryan Keely]", m.Performers)
	}
	if len(m.Tags) != 1 || m.Tags[0] != "Ladies With Toys" {
		t.Errorf("Tags = %v, want [Ladies With Toys]", m.Tags)
	}

	m2 := movies[1].Scene
	if m2.ID != "21889" {
		t.Errorf("second movie ID = %q, want 21889", m2.ID)
	}
	if m2.Title != "Ryan Keely — Interview" {
		t.Errorf("second movie Title = %q, want %q", m2.Title, "Ryan Keely — Interview")
	}
}

func TestKnownIDsEarlyStop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, fixtureHTML)
	}))
	defer ts.Close()

	s := New()
	s.client = ts.Client()

	ch, err := s.ListScenes(context.Background(), ts.URL+"/model-pages/ryan-keely/1549", scraper.ListOpts{
		KnownIDs: map[string]bool{"21889": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var movieCount int
	var stoppedEarly bool
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			movieCount++
		case scraper.KindStoppedEarly:
			stoppedEarly = true
		}
	}

	if movieCount != 1 {
		t.Errorf("got %d movies before early stop, want 1", movieCount)
	}
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
}

type SceneResult = scraper.SceneResult
