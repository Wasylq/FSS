package testutil

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// goodScene is the shape sceneProblems must accept without complaint. Each
// test below breaks exactly one field of it.
func goodScene() models.Scene {
	return models.Scene{
		ID:         "123",
		SiteID:     "example",
		Title:      "A Scene",
		URL:        "https://example.com/scene/123",
		Date:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Duration:   600,
		Performers: []string{"Someone"},
		ScrapedAt:  time.Now().UTC(),
	}
}

func TestSceneProblemsAcceptsValidScene(t *testing.T) {
	if got := sceneProblems(goodScene()); len(got) != 0 {
		t.Errorf("sceneProblems(good) = %v, want none", got)
	}
}

// Every rule ValidateScene claims to enforce must actually reject its bad
// shape — otherwise 344 integration tests are asserting nothing.
func TestSceneProblemsRejectsEachBadShape(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*models.Scene)
		want string
	}{
		{"empty ID", func(s *models.Scene) { s.ID = "" }, "empty ID"},
		{"empty SiteID", func(s *models.Scene) { s.SiteID = "" }, "empty SiteID"},
		{"empty Title", func(s *models.Scene) { s.Title = "" }, "empty Title"},
		{"empty URL", func(s *models.Scene) { s.URL = "" }, "empty URL"},
		{"relative URL", func(s *models.Scene) { s.URL = "/scene/123" }, "malformed URL"},
		{"scheme-only URL", func(s *models.Scene) { s.URL = "https://" }, "malformed URL"},
		{"negative Duration", func(s *models.Scene) { s.Duration = -1 }, "implausible Duration"},
		{"Duration over 7 days", func(s *models.Scene) { s.Duration = 7*24*60*60 + 1 }, "implausible Duration"},
		{"no credits", func(s *models.Scene) { s.Performers = nil; s.Studio = "" }, "neither Performers nor Studio"},
		{"zero ScrapedAt", func(s *models.Scene) { s.ScrapedAt = time.Time{} }, "zero ScrapedAt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := goodScene()
			c.mut(&s)
			got := sceneProblems(s)
			if len(got) == 0 {
				t.Fatalf("sceneProblems accepted a scene with %s", c.name)
			}
			if !strings.Contains(strings.Join(got, "; "), c.want) {
				t.Errorf("sceneProblems = %v, want a message containing %q", got, c.want)
			}
		})
	}
}

// Studio alone satisfies the credits rule — many sites have no per-scene
// performer list.
func TestSceneProblemsAcceptsStudioWithoutPerformers(t *testing.T) {
	s := goodScene()
	s.Performers = nil
	s.Studio = "Example Studio"
	if got := sceneProblems(s); len(got) != 0 {
		t.Errorf("sceneProblems = %v, want none (Studio should satisfy the credits rule)", got)
	}
}

// Duration 0 and a zero Date are advisory, not failures: list endpoints often
// omit both. If these ever become hard errors, ~40 scrapers start failing.
func TestSceneProblemsToleratesMissingDurationAndDate(t *testing.T) {
	s := goodScene()
	s.Duration = 0
	s.Date = time.Time{}
	if got := sceneProblems(s); len(got) != 0 {
		t.Errorf("sceneProblems = %v, want none (Duration/Date are advisory)", got)
	}
	notes := sceneNotes(s)
	if len(notes) != 1 || !strings.Contains(notes[0], "zero Date") {
		t.Errorf("sceneNotes = %v, want a zero-Date note", notes)
	}
}

func TestSceneNotesSilentOnGoodScene(t *testing.T) {
	if got := sceneNotes(goodScene()); len(got) != 0 {
		t.Errorf("sceneNotes = %v, want none", got)
	}
}

func TestDrainSeparatesKinds(t *testing.T) {
	ch := make(chan scraper.SceneResult, 8)
	ch <- scraper.Progress(42)
	ch <- scraper.Scene(models.Scene{ID: "1"})
	ch <- scraper.Error(io.ErrUnexpectedEOF)
	ch <- scraper.Scene(models.Scene{ID: "2"})
	ch <- scraper.StoppedEarly()
	close(ch)

	scenes, stoppedEarly, errs := drain(ch)
	if len(scenes) != 2 || scenes[0].ID != "1" || scenes[1].ID != "2" {
		t.Errorf("scenes = %v, want IDs 1,2 in order", scenes)
	}
	if !stoppedEarly {
		t.Error("stoppedEarly = false, want true")
	}
	if len(errs) != 1 || !errors.Is(errs[0], io.ErrUnexpectedEOF) {
		t.Errorf("errs = %v, want the one sent error", errs)
	}
}

func TestDrainEmptyChannel(t *testing.T) {
	ch := make(chan scraper.SceneResult)
	close(ch)
	scenes, stoppedEarly, errs := drain(ch)
	if len(scenes) != 0 || stoppedEarly || len(errs) != 0 {
		t.Errorf("drain(empty) = (%v, %v, %v), want all zero", scenes, stoppedEarly, errs)
	}
}

// A Progress signal alone must not be mistaken for a scene.
func TestDrainIgnoresProgress(t *testing.T) {
	ch := make(chan scraper.SceneResult, 1)
	ch <- scraper.Progress(99)
	close(ch)
	if scenes, _, _ := drain(ch); len(scenes) != 0 {
		t.Errorf("scenes = %v, want none", scenes)
	}
}

func TestIsPlaceholder(t *testing.T) {
	if !isPlaceholder("https://REPLACE-ME.com/") {
		t.Error("isPlaceholder did not detect REPLACE-ME")
	}
	if isPlaceholder("https://example.com/studio") {
		t.Error("isPlaceholder flagged a real URL")
	}
}

const sitemapFixture = `<?xml version="1.0"?>
<urlset><url><loc>https://honeytrans.com/scenes/1/a</loc></url>
<url><loc>https://honeytrans.com/scenes/2/b</loc></url></urlset>`

func TestRewriteSitemapHost(t *testing.T) {
	out, err := rewriteSitemapHost([]byte(sitemapFixture), "https://honeytrans.com", "http://127.0.0.1:1234")
	if err != nil {
		t.Fatalf("rewriteSitemapHost: %v", err)
	}
	if strings.Contains(string(out), "honeytrans.com") {
		t.Errorf("live host survived the rewrite: %s", out)
	}
	if n := strings.Count(string(out), "http://127.0.0.1:1234"); n != 2 {
		t.Errorf("rewrote %d occurrences, want 2", n)
	}
}

// The tripwire is the whole point of the helper: a refreshed fixture on a
// different host would make the rewrite a silent no-op and send detail fetches
// to the live site.
func TestRewriteSitemapHostFailsWhenHostAbsent(t *testing.T) {
	_, err := rewriteSitemapHost([]byte(sitemapFixture), "https://someothersite.com", "http://127.0.0.1:1234")
	if err == nil {
		t.Fatal("rewriteSitemapHost accepted a fixture not containing the live host")
	}
	if !strings.Contains(err.Error(), "no-op") {
		t.Errorf("error = %v, want it to explain the no-op risk", err)
	}
}

func TestSitemapServerServesRewrittenSitemapAndDetail(t *testing.T) {
	srv := SitemapServer(t, "https://honeytrans.com", []byte(sitemapFixture), []byte("DETAIL-BODY"))

	body := httpGet(t, srv.URL+"/sitemap.xml")
	if strings.Contains(body, "honeytrans.com") {
		t.Errorf("served sitemap still points at the live host: %s", body)
	}
	if !strings.Contains(body, srv.URL) {
		t.Errorf("served sitemap does not point at the test server: %s", body)
	}

	// Any non-sitemap path is a detail page.
	if got := httpGet(t, srv.URL+"/scenes/1/a"); got != "DETAIL-BODY" {
		t.Errorf("detail body = %q, want DETAIL-BODY", got)
	}
	// Per-language and video sitemap variants must also serve the sitemap.
	if got := httpGet(t, srv.URL+"/sitemap_video.xml"); !strings.Contains(got, "urlset") {
		t.Errorf("sitemap_video.xml did not serve the sitemap: %q", got)
	}
}

func httpGet(t *testing.T, u string) string {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", u, err)
	}
	return string(b)
}

func TestDuplicateIDs(t *testing.T) {
	sc := func(id string) models.Scene { return models.Scene{ID: id} }

	if got := duplicateIDs([]models.Scene{sc("a"), sc("b"), sc("c")}); len(got) != 0 {
		t.Errorf("duplicateIDs(unique) = %v, want none", got)
	}
	if got := duplicateIDs(nil); len(got) != 0 {
		t.Errorf("duplicateIDs(nil) = %v, want none", got)
	}
	got := duplicateIDs([]models.Scene{sc("a"), sc("b"), sc("a"), sc("b"), sc("a")})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("duplicateIDs = %v, want [a b] (each reported once, in first-repeat order)", got)
	}
}

// leakyScraper ignores ctx entirely: it keeps sending until its buffer drains,
// modelling a scraper whose channel sends are not guarded by a select on
// ctx.Done(). AssertCancellable must catch it.
type leakyScraper struct{ block chan struct{} }

func (l *leakyScraper) ID() string         { return "leaky" }
func (l *leakyScraper) Patterns() []string { return []string{"leaky.test"} }
func (l *leakyScraper) MatchesURL(string) bool {
	return true
}
func (l *leakyScraper) ListScenes(_ context.Context, _ string, _ scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		out <- scraper.Scene(models.Scene{ID: "1"})
		<-l.block // never returns until the test releases it
	}()
	return out, nil
}

// goodScraper exits promptly once its context is cancelled.
type goodScraper struct{}

func (goodScraper) ID() string             { return "good" }
func (goodScraper) Patterns() []string     { return []string{"good.test"} }
func (goodScraper) MatchesURL(string) bool { return true }
func (goodScraper) ListScenes(ctx context.Context, _ string, _ scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go func() {
		defer close(out)
		for i := 0; ; i++ {
			select {
			case out <- scraper.Scene(models.Scene{ID: strconv.Itoa(i)}):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func TestAssertCancellablePassesForACancellableScraper(t *testing.T) {
	AssertCancellable(t, goodScraper{}, "https://good.test/", scraper.ListOpts{})
}

// The helper is only worth having if it fails on a scraper that ignores
// cancellation — verified here by running it against one and asserting it
// reports a failure, rather than trusting that it would.
func TestAssertCancellableCatchesALeak(t *testing.T) {
	l := &leakyScraper{block: make(chan struct{})}
	defer close(l.block)

	fake := &testing.T{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		AssertCancellable(fake, l, "https://leaky.test/", scraper.ListOpts{})
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("AssertCancellable never returned")
	}
	if !fake.Failed() {
		t.Error("AssertCancellable passed a scraper that ignores context cancellation")
	}
}
