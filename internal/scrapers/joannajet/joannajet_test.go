package joannajet

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Anastylosis/FSS/internal/scrapers/testutil"
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

// scenePage reproduces a scene, including the "More Videos" rail of other
// scenes that follows it — each rail entry has its own title, date and quality.
func scenePage(title, desc, released, quality string) string {
	return fmt.Sprintf(`<html><head>
<title>Joanna Jet | Scene Preview - %s</title>
</head><body class="updates-index">
<div class="JJVidInfo">
  <span class="left">Released: <strong>%s</strong></span>
  <span class="right">Quality: <strong>%s</strong></span>
</div>
<div class="JJVidDesc"> %s </div>
<img src="https://www.joannajet.com/pics/gallery/preview/e_jjlsbdtt.jpg" alt="%s">
<div class="moreHeader"><div class="moreHeader-in">More Videos</div></div>
<div class="JJminiVidArea">
  <a href="scene_m.php?vid=1092"><img src="/pics/gallery/thumbnails/v_x/t_x.jpg" alt="A Neighbour"></a>
  <span class="JJminiVidArea-info">Quality: <strong>1080p </strong></span>
  <span class="JJminiVidArea-info">Released: <strong>14 August 2026 </strong></span>
</div>
</body></html>`, title, released, quality, desc, title)
}

// emptyShell is what a vid the site does not have returns: the page renders,
// the title carries only the fixed prefix, and nothing else is filled in.
const emptyShell = `<html><head><title>Joanna Jet | Scene Preview - </title></head>
<body class="updates-index"></body></html>`

func indexPage(vids ...int) string {
	var sb strings.Builder
	sb.WriteString(`<html><body>`)
	for _, v := range vids {
		fmt.Fprintf(&sb, `<a href="scene_m.php?vid=%d&this_page=1&vcat=99">x</a>`, v)
	}
	sb.WriteString(`</body></html>`)
	return sb.String()
}

type stubSite struct {
	*httptest.Server
	mu   sync.Mutex
	hits map[string]int
}

func (s *stubSite) hit(p string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits[p]++
}

func (s *stubSite) count(p string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[p]
}

func newSite(t *testing.T, present []int, indexed []int) *stubSite {
	t.Helper()
	have := map[int]bool{}
	for _, v := range present {
		have[v] = true
	}
	site := &stubSite{hits: map[string]int{}}
	site.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		site.hit(r.URL.Path + "?" + r.URL.RawQuery)
		switch r.URL.Path {
		case "/scene_m.php":
			vid, _ := strconv.Atoi(r.URL.Query().Get("vid"))
			if !have[vid] {
				_, _ = fmt.Fprint(w, emptyShell)
				return
			}
			_, _ = fmt.Fprint(w, scenePage(fmt.Sprintf("Me and You %d", vid),
				"A description.", "15 November 2024", "4K"))
		case "/movies_m.php":
			if r.URL.Query().Get("action") == "display" {
				_, _ = fmt.Fprint(w, indexPage(indexed...))
				return
			}
			_, _ = fmt.Fprint(w, `<html><body><a href="movies_m.php?action=display&movieid=16">m</a></body></html>`)
		default:
			_, _ = fmt.Fprint(w, indexPage(indexed...))
		}
	}))
	return site
}

func newTestScraper(srv *httptest.Server) *Scraper {
	s := New()
	s.Client = srv.Client()
	s.baseOverride = srv.URL
	return s
}

func collect(t *testing.T, s *Scraper, studioURL string, opts scraper.ListOpts) ([]models.Scene, []error, int) {
	t.Helper()
	ch, err := s.ListScenes(context.Background(), studioURL, opts)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	var scenes []models.Scene
	var errs []error
	total := 0
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			scenes = append(scenes, res.Scene)
		case scraper.KindError:
			errs = append(errs, res.Err)
		case scraper.KindTotal:
			total = res.Total
		}
	}
	return scenes, errs, total
}

// The index pages reach a fraction of the catalogue, so the sweep runs to the
// highest vid they name plus a margin and finds the ones in between.
func TestSweepFindsScenesTheIndexDoesNot(t *testing.T) {
	site := newSite(t, []int{2, 5, 9}, []int{9})
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, _ := collect(t, s, "http://www.joannajet.com/", scraper.ListOpts{Workers: 3})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 3 {
		t.Fatalf("got %d scenes, want 3 — vids 2 and 5 are not linked anywhere", len(scenes))
	}
}

// A vid the site does not have renders the page shell with only the fixed
// prefix in <title>. Whitespace collapsing leaves that reading exactly
// "Joanna Jet | Scene Preview -", which a literal prefix trim does not empty —
// and 85 such shells were stored as scenes titled that.
func TestEmptyShellIsNotAScene(t *testing.T) {
	d := parseScene(emptyShell)
	if d.title != "" {
		t.Errorf("title = %q, want empty", d.title)
	}

	site := newSite(t, []int{5}, []int{5})
	defer site.Close()
	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "http://www.joannajet.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		if strings.Contains(sc.Title, "Scene Preview") {
			t.Errorf("an empty shell was stored as %q", sc.Title)
		}
	}
	if len(scenes) != 1 {
		t.Errorf("got %d scenes, want 1", len(scenes))
	}
}

// Every scene page ends with a "More Videos" rail whose entries carry their own
// title, date and quality; nothing may take those for the scene's.
func TestMoreVideosRailDoesNotLeakIntoTheScene(t *testing.T) {
	d := parseScene(scenePage("Me and You 640 - Tight and Shiny", "A description.",
		"15 November 2024", "4K"))

	if d.title != "Me and You 640 - Tight and Shiny" {
		t.Errorf("title = %q", d.title)
	}
	if d.date.Format("2006-01-02") != "2024-11-15" {
		t.Errorf("date = %v — the rail's 14 August 2026 may have won", d.date)
	}
	if d.resolution != "4K" {
		t.Errorf("resolution = %q — the rail's 1080p may have won", d.resolution)
	}
	if d.description != "A description." {
		t.Errorf("description = %q", d.description)
	}
}

// The sweep is by id, not by date, so a stored id says nothing about the ids
// after it.
func TestKnownIDsAreSkippedNotStoppedOn(t *testing.T) {
	site := newSite(t, []int{2, 5, 9}, []int{9})
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "http://www.joannajet.com/",
		scraper.ListOpts{Workers: 2, KnownIDs: map[string]bool{"2": true}})

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want the two unknown ones", len(scenes))
	}
	if site.count("/scene_m.php?vid=2") != 0 {
		t.Error("a stored scene was re-fetched")
	}
}

// A single scene URL skips the sweep.
func TestSingleSceneURLSkipsTheSweep(t *testing.T) {
	site := newSite(t, []int{9}, []int{9})
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, errs, total := collect(t, s, "http://www.joannajet.com/scene_m.php?vid=9", scraper.ListOpts{Workers: 1})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || total != 1 {
		t.Fatalf("got %d scenes (total %d), want 1", len(scenes), total)
	}
	if site.count("/home.php?") != 0 {
		t.Error("a single-scene URL read the index pages")
	}
}

// Index pages naming no scenes leave the sweep with no bound — a template
// change, not an empty site.
func TestIndexWithNoScenesIsAParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "<html><body>nothing here</body></html>")
	}))
	defer srv.Close()

	s := newTestScraper(srv)
	scenes, errs, _ := collect(t, s, "http://www.joannajet.com/", scraper.ListOpts{Workers: 1})
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) == 0 {
		t.Fatal("an index with no scenes reported no error")
	}
	if k := scraper.Classify(errs[len(errs)-1]); k != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", k)
	}
}

// The host's HTTPS listener serves an incomplete chain, so the scheme is forced
// rather than taken from the operator's URL — `cmd.normalizeInputURL` upgrades
// a bare host to https and every such run would otherwise fail at the TLS layer.
func TestBaseIsAlwaysHTTP(t *testing.T) {
	if got := New().base(); !strings.HasPrefix(got, "http://") {
		t.Errorf("base() = %q, want an http:// origin", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://joannajet.com",
		"http://www.joannajet.com/",
		"https://www.joannajet.com/scene_m.php?vid=1000",
		// The site's own pages link a four-w typo host.
		"https://wwww.joannajet.com/home.php",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{"https://joannajetfan.com/", "https://example.com/joannajet.com"} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestScenesValidate(t *testing.T) {
	site := newSite(t, []int{2, 5, 9}, []int{9})
	defer site.Close()

	s := newTestScraper(site.Server)
	scenes, _, _ := collect(t, s, "http://www.joannajet.com/", scraper.ListOpts{Workers: 2})
	for _, sc := range scenes {
		testutil.ValidateScene(t, sc)
	}
}

func TestContextCancellationStopsTheSweep(t *testing.T) {
	site := newSite(t, []int{500}, []int{500})
	defer site.Close()

	s := newTestScraper(site.Server)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "http://www.joannajet.com/", scraper.ListOpts{Workers: 3, Delay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	cancel()
	for range ch { //nolint:revive // drain so the goroutines can finish their sends
	}
}
