package sexunderwater

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

func cardHTML(trailer, title, modelHref, model, date, thumb string) string {
	return fmt.Sprintf(`<div class="updateItem">
	<a onclick="tload('/trailers/%s.mp4')">
		<img alt="x" src="%s" src0_1x="%s" src0_2x="%s-2x.jpg" />	</a>
	<div class="updateDetails">
		<h4>
			<!-- Link to Trailer Only If a Trailer is Present -->
			<a onclick="tload('/trailers/%s.mp4')">
			%s			</a>
		</h4>
		<p>
	<span class="tour_update_models">
	<a href="%s">%s</a>
	</span>
 <span>%s</span></p>
	</div>
</div>`, trailer, thumb, thumb, thumb, trailer, title, modelHref, model, date)
}

func listingHTML() string {
	return "<html><body>" +
		cardHTML("lunch_break-60_su-tr", "Lunch Break", "https://sexunderwater.com/models/Deliah.html", "Deliah", "01/29/2023", "content/nr147 lunch/1.jpg") +
		cardHTML("olivia_olove_bj-60_su-tr", "Olivia &amp; Friends", "https://sexunderwater.com/models/OliviaOlove.html", "Olivia Olove", "01/08/2023", "content/nr126-oliviabj/1.jpg") +
		"</body></html>"
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://sexunderwater.com/categories/SexUnderwater_1_d.html": true,
		"https://www.sexunderwater.com/":                              true,
		"https://example.com/x":                                       false,
		"":                                                            false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestParseListing(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	siteBase = "https://sexunderwater.com"

	scenes := parseListing([]byte(listingHTML()), "studioURL", time.Now().UTC())
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "lunch_break-60_su-tr" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "Lunch Break" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != "https://sexunderwater.com/trailers/lunch_break-60_su-tr.mp4" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Studio != studioName {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Deliah" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Date.Year() != 2023 || sc.Date.Month() != 1 || sc.Date.Day() != 29 {
		t.Errorf("Date = %v", sc.Date)
	}
	if sc.Thumbnail != "https://sexunderwater.com/content/nr147 lunch/1.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	// HTML entity decoded in title.
	if scenes[1].Title != "Olivia & Friends" {
		t.Errorf("title[1] = %q", scenes[1].Title)
	}
}

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "SexUnderwater_1_d.html") {
			_, _ = fmt.Fprint(w, listingHTML())
			return
		}
		_, _ = fmt.Fprint(w, "<html><body>no items</body></html>")
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), "studioURL", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes error: %v", err)
	}
	got := map[string]string{}
	for r := range ch {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
			continue
		}
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r.Scene.Title
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["lunch_break-60_su-tr"] != "Lunch Break" {
		t.Errorf("scenes = %v", got)
	}
}
