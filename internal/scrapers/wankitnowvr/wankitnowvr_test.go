package wankitnowvr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

var _ scraper.StudioScraper = (*Scraper)(nil)

const testHTML = `<div class="row">
<div class="col-12 col-sm-6 col-xl-4 mb-2">
    <div class="card border-0 px-0">
        <a href="https://wankitnowvr.com/videos/tamsin+riley+which+toy/2543">
            <div class="view overlay">
                <img class="card-img-top" src="https://c758f77483.mjedge.net/2543/o/artwork/art.jpg?w=450&h=300" alt="Which Toy?">
            </div>
        </a>
        <div class="card-body p-0">
            <h5 class="card-title pt-2 font-weight-bold h5-responsive">
                <a href="https://wankitnowvr.com/videos/tamsin+riley+which+toy/2543" class="card-link">Which Toy?</a>
            </h5>
            <p class="card-subtitle mb-2 text-muted">May 4, 2026 | Duration: 13:06</p>
            <h6 class="card-subtitle mb-2 text-muted">Starring <a href="https://wankitnowvr.com/models/tamsin+riley/147" title="Tamsin Riley VR videos" class="btn-link">Tamsin Riley</a></h6>
        </div>
    </div>
</div>
</div>`

func TestParseListingPage(t *testing.T) {
	cards := parseListingPage([]byte(testHTML))
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}

	c := cards[0]
	if c.id != "2543" {
		t.Errorf("id = %q, want 2543", c.id)
	}
	if c.title != "Which Toy?" {
		t.Errorf("title = %q, want %q", c.title, "Which Toy?")
	}
	if c.date.Format("2006-01-02") != "2026-05-04" {
		t.Errorf("date = %v, want 2026-05-04", c.date)
	}
	if c.duration != 786 {
		t.Errorf("duration = %d, want 786", c.duration)
	}
	if len(c.performers) != 1 || c.performers[0] != "Tamsin Riley" {
		t.Errorf("performers = %v, want [Tamsin Riley]", c.performers)
	}
	if c.thumbnail != "https://c758f77483.mjedge.net/2543/o/artwork/art.jpg?w=450&h=300" {
		t.Errorf("thumbnail = %q", c.thumbnail)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"13:06", 786},
		{"1:00", 60},
		{"0:30", 30},
	}
	for _, tt := range tests {
		if got := parseDuration(tt.in); got != tt.want {
			t.Errorf("parseDuration(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestFetchPageIntegration(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/videos":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, testHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	cards := parseListingPage([]byte(testHTML))
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
}
