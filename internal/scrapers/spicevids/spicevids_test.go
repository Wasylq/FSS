package spicevids

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/ayloutil"
	"github.com/Anastylosis/FSS/scraper"
)

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = &spicevidsScraper{}
}

func TestMatchesURL(t *testing.T) {
	s := &spicevidsScraper{}

	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"root", "https://www.spicevids.com", true},
		{"scenes page", "https://www.spicevids.com/scenes", true},
		{"model URL", "https://www.spicevids.com/model/123/name", true},
		{"collection URL", "https://www.spicevids.com/collection/62061/adamandevevod", true},
		{"category URL", "https://www.spicevids.com/category/5/anal", true},
		{"no www", "https://spicevids.com/scenes", true},
		{"other domain", "https://www.example.com", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := s.MatchesURL(c.url); got != c.want {
				t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}

// spicevids is a thin delegate: every scrape is handled by ayloutil, which
// carries the parsing tests. What was untested here is its own interface
// surface — ID, Patterns and the ListScenes wiring — which left the package at
// 28.6%. There is deliberately no end-to-end test: driving ayloutil through this
// wrapper would duplicate ayloutil's own tests without adding signal.

func TestID(t *testing.T) {
	s := &spicevidsScraper{aylo: ayloutil.New(ayloConfig)}
	if s.ID() != "spicevids" {
		t.Errorf("ID = %q, want spicevids", s.ID())
	}
	if s.ID() != ayloConfig.SiteID {
		t.Errorf("ID %q disagrees with ayloConfig.SiteID %q", s.ID(), ayloConfig.SiteID)
	}
}

// Every advertised pattern must be a URL this scraper actually accepts,
// otherwise `fss list-scrapers` promises URLs it would refuse.
func TestPatternsAreAccepted(t *testing.T) {
	s := &spicevidsScraper{aylo: ayloutil.New(ayloConfig)}
	pats := s.Patterns()
	if len(pats) == 0 {
		t.Fatal("Patterns is empty")
	}
	for _, p := range pats {
		host := p
		if i := strings.IndexByte(host, '/'); i >= 0 {
			host = host[:i]
		}
		if !s.MatchesURL("https://" + host) {
			t.Errorf("pattern %q names host %q, which MatchesURL rejects", p, host)
		}
	}
}

// ListScenes must hand back a channel that closes rather than leaking a
// goroutine. Driven with an already-cancelled context so no request is made.
func TestListScenesReturnsAClosedChannelOnCancel(t *testing.T) {
	s := &spicevidsScraper{aylo: ayloutil.New(ayloConfig)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch, err := s.ListScenes(ctx, "https://www.spicevids.com/", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("channel still open 10s after a cancelled context — the scrape leaks a goroutine")
	}
}
