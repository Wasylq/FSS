package littlecapricedreams

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/scraper"
)

func TestScraperInterface(t *testing.T) {
	var _ scraper.StudioScraper = New()
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.littlecaprice-dreams.com/", true},
		{"https://littlecaprice-dreams.com/videos/", true},
		{"http://www.littlecaprice-dreams.com/collection/buttmuse/", true},
		{"https://www.littlecaprice-dreams.com/model/marcello-bravo/", true},
		{"https://www.littlecaprice-dreams.com", true},
		{"https://www.example.com/", false},
		{"https://littlecaprice-dreams.com.evil.test/", false},
		{"https://notlittlecaprice-dreams.com/", false},
	}
	for _, tt := range tests {
		if got := s.MatchesURL(tt.url); got != tt.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestStudioForPrefersTheSubBrand(t *testing.T) {
	c := categories{byID: map[int]term{
		20:  {ID: 20, Slug: "videos", Name: "Videos"},
		28:  {ID: 28, Slug: "gallery", Name: "Gallery"},
		207: {ID: 207, Slug: "buttmuse", Name: "Buttmuse"},
		990: {ID: 990, Slug: "unlisted", Name: "Unlisted"},
	}}
	if got := c.studioFor([]int{20, 207}); got != "Buttmuse" {
		t.Errorf("studioFor = %q, want the sub-brand Buttmuse", got)
	}
	// Content-type terms are not brands; a project carrying only those falls
	// back to the site name rather than being filed under "Videos".
	if got := c.studioFor([]int{20, 28, 990}); got != studioName {
		t.Errorf("studioFor = %q, want %q", got, studioName)
	}
	if got := c.studioFor(nil); got != studioName {
		t.Errorf("studioFor(nil) = %q, want %q", got, studioName)
	}
}

func TestParseDetail(t *testing.T) {
	// The theme writes og tags with name=, not the standard property=.
	body := `<html><head>
	<meta name="og:image" content="https://www.littlecaprice-dreams.com/wp-content/uploads/2026/07/wpp_Jack.jpg"/>
	<meta name="og:video:tag" content="anal"/>
	<meta name="og:video:tag" content="deep throat"/>
	<meta name="og:video:tag" content="anal"/>
	<meta name="og:video:actor" content="https://www.littlecaprice-dreams.com/model/clemence-audiard/"/>
	</head><body>
	<div class="cast">
	<a href='https://www.littlecaprice-dreams.com/model/marcello-bravo/'>Marcello Bravo</a>
	<a href="https://www.littlecaprice-dreams.com/model/clemence-audiard/">Clemence Audiard</a>
	<a href="https://www.littlecaprice-dreams.com/model/marcello-bravo/">Marcello Bravo</a>
	</div></body></html>`

	d := parseDetail(body)
	if d.thumbnail != "https://www.littlecaprice-dreams.com/wp-content/uploads/2026/07/wpp_Jack.jpg" {
		t.Errorf("thumbnail = %q", d.thumbnail)
	}
	want := []string{"Marcello Bravo", "Clemence Audiard"}
	if len(d.performers) != len(want) {
		t.Fatalf("performers = %v, want %v (deduped)", d.performers, want)
	}
	for i := range want {
		if d.performers[i] != want[i] {
			t.Errorf("performers[%d] = %q, want %q", i, d.performers[i], want[i])
		}
	}
	if len(d.tags) != 2 || d.tags[0] != "anal" || d.tags[1] != "deep throat" {
		t.Errorf("tags = %v, want [anal, deep throat] deduped", d.tags)
	}
}

func TestCleanText(t *testing.T) {
	got := cleanText("<p>Enjoy the first part &amp; more&#8230;</p>\n")
	if got != "Enjoy the first part & more…" {
		t.Errorf("cleanText = %q", got)
	}
}

// ---- end-to-end against a fake WordPress ----

type wpServer struct {
	*httptest.Server
	listingHits atomic.Int32
	detailHits  atomic.Int32
}

type fakeProject struct {
	id     int
	slug   string
	title  string
	date   string
	cats   []int
	models []string
}

const (
	videosID        = 20
	galleryID       = 28
	buttmuseID      = 207
	pornLifestyleID = 208
)

func newWPServer(t *testing.T, projects []fakeProject) *wpServer {
	t.Helper()
	ts := &wpServer{}
	mux := http.NewServeMux()

	mux.HandleFunc("/wp-json/wp/v2/project_category", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `[{"id":%d,"slug":"videos","name":"Videos"},
			{"id":%d,"slug":"gallery","name":"Gallery"},
			{"id":%d,"slug":"buttmuse","name":"Buttmuse"},
			{"id":%d,"slug":"porn-lifestyle","name":"PornLifestyle"}]`, videosID, galleryID, buttmuseID, pornLifestyleID)
	})

	mux.HandleFunc("/wp-json/wp/v2/project", func(w http.ResponseWriter, r *http.Request) {
		ts.listingHits.Add(1)
		cat, _ := strconv.Atoi(r.URL.Query().Get("project_category"))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page > 1 {
			_, _ = fmt.Fprint(w, `[]`)
			return
		}
		var out []string
		for _, p := range projects {
			if !hasInt(p.cats, cat) {
				continue
			}
			cats := make([]string, 0, len(p.cats))
			for _, c := range p.cats {
				cats = append(cats, strconv.Itoa(c))
			}
			out = append(out, fmt.Sprintf(`{"id":%d,"date_gmt":"%s","link":"https://www.littlecaprice-dreams.com/project/%s/",
				"title":{"rendered":"%s"},"excerpt":{"rendered":"<p>About %s</p>"},"project_category":[%s]}`,
				p.id, p.date, p.slug, p.title, p.slug, strings.Join(cats, ",")))
		}
		_, _ = fmt.Fprint(w, "["+strings.Join(out, ",")+"]")
	})

	mux.HandleFunc("/project/", func(w http.ResponseWriter, r *http.Request) {
		ts.detailHits.Add(1)
		slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/project/"), "/")
		var p *fakeProject
		for i := range projects {
			if projects[i].slug == slug {
				p = &projects[i]
			}
		}
		if p == nil {
			http.NotFound(w, r)
			return
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, `<meta name="og:image" content="https://cdn.test/%s.jpg"/>`, p.slug)
		fmt.Fprint(&sb, `<meta name="og:video:tag" content="hardcore"/>`)
		for _, m := range p.models {
			fmt.Fprintf(&sb, `<a href="https://www.littlecaprice-dreams.com/model/%s/">%s</a>`,
				strings.ToLower(strings.ReplaceAll(m, " ", "-")), m)
		}
		_, _ = fmt.Fprint(w, sb.String())
	})

	mux.HandleFunc("/model/", func(w http.ResponseWriter, r *http.Request) {
		slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/model/"), "/")
		var sb strings.Builder
		for _, p := range projects {
			for _, m := range p.models {
				if strings.ToLower(strings.ReplaceAll(m, " ", "-")) == slug {
					fmt.Fprintf(&sb, `<a href="/project/%s/">x</a>`, p.slug)
				}
			}
		}
		_, _ = fmt.Fprint(w, sb.String())
	})

	ts.Server = httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func hasInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func newTestScraper(ts *wpServer) *Scraper {
	s := New()
	s.Client = ts.Client()
	s.baseOverride = ts.URL
	return s
}

func collect(t *testing.T, ch <-chan scraper.SceneResult) ([]models.Scene, []error, int, bool) {
	t.Helper()
	var scenes []models.Scene
	var errs []error
	total, stopped := 0, false
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene)
		case scraper.KindError:
			errs = append(errs, r.Err)
		case scraper.KindTotal:
			total = r.Total
		case scraper.KindStoppedEarly:
			stopped = true
		}
	}
	return scenes, errs, total, stopped
}

func sampleProjects() []fakeProject {
	return []fakeProject{
		{id: 1, slug: "vid-one", title: "Vid One", date: "2026-08-14T10:00:00",
			cats: []int{videosID, buttmuseID}, models: []string{"Marcello Bravo", "Clemence Audiard"}},
		{id: 2, slug: "vid-two", title: "Vid Two", date: "2026-08-10T10:00:00",
			cats: []int{videosID}, models: []string{"Marcello Bravo"}},
		// A photo set: same post type, same sub-brand, no videos term.
		{id: 3, slug: "gal-one", title: "Gal One", date: "2026-08-12T10:00:00",
			cats: []int{galleryID, buttmuseID}, models: []string{"Clemence Audiard"}},
	}
}

func TestListScenesSkipsGalleries(t *testing.T) {
	ts := newWPServer(t, sampleProjects())
	s := newTestScraper(ts)

	ch, err := s.ListScenes(context.Background(), "https://www.littlecaprice-dreams.com/videos/", scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, total, _ := collect(t, ch)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 — the gallery shares the post type and must be skipped", len(scenes))
	}
	if total != 2 {
		t.Errorf("progress total = %d, want 2", total)
	}
	if got := ts.detailHits.Load(); got != 2 {
		t.Errorf("detail fetches = %d, want 2 — a skipped gallery must cost no request", got)
	}

	byID := map[string]models.Scene{}
	for _, sc := range scenes {
		byID[sc.ID] = sc
		if sc.SiteID != siteID {
			t.Errorf("siteID = %q", sc.SiteID)
		}
		if sc.StudioURL != "https://www.littlecaprice-dreams.com/videos/" {
			t.Errorf("studioURL = %q, want the operator's URL verbatim", sc.StudioURL)
		}
		if !strings.HasPrefix(sc.URL, ts.URL) {
			t.Errorf("scene %s URL %q left the test server", sc.ID, sc.URL)
		}
		if sc.ScrapedAt.IsZero() {
			t.Errorf("scene %s has no ScrapedAt", sc.ID)
		}
	}
	one := byID["1"]
	if one.Title != "Vid One" {
		t.Errorf("title = %q", one.Title)
	}
	if one.Studio != "Buttmuse" {
		t.Errorf("studio = %q, want the sub-brand", one.Studio)
	}
	if one.Description != "About vid-one" {
		t.Errorf("description = %q", one.Description)
	}
	if one.Date.Format("2006-01-02") != "2026-08-14" {
		t.Errorf("date = %v", one.Date)
	}
	if one.Date.Location() != nil && one.Date.Location().String() != "UTC" {
		t.Errorf("date location = %v, want UTC", one.Date.Location())
	}
	if len(one.Performers) != 2 {
		t.Errorf("performers = %v", one.Performers)
	}
	if len(one.Tags) != 1 || one.Tags[0] != "hardcore" {
		t.Errorf("tags = %v", one.Tags)
	}
	// Falls back to the site name when the project carries no sub-brand.
	if byID["2"].Studio != studioName {
		t.Errorf("studio = %q, want %q", byID["2"].Studio, studioName)
	}
}

// A single path segment naming a real term is that collection — the form the
// sub-brands were published under, which the site redirects to /collection/.
func TestCollectionURLForms(t *testing.T) {
	for _, u := range []string{
		"https://www.littlecaprice-dreams.com/collection/buttmuse/",
		"https://www.littlecaprice-dreams.com/buttmuse/",
	} {
		ts := newWPServer(t, sampleProjects())
		s := newTestScraper(ts)
		ch, err := s.ListScenes(context.Background(), u, scraper.ListOpts{Workers: 2})
		if err != nil {
			t.Fatal(err)
		}
		scenes, errs, _, _ := collect(t, ch)
		if len(errs) != 0 {
			t.Fatalf("%s: unexpected errors: %v", u, errs)
		}
		// vid-one is buttmuse+videos; gal-one is buttmuse+gallery and must not
		// come through a collection URL either.
		if len(scenes) != 1 || scenes[0].ID != "1" {
			t.Fatalf("%s: got %d scenes %v, want just vid-one", u, len(scenes), scenes)
		}
	}
}

func TestUnknownCollectionIsReported(t *testing.T) {
	ts := newWPServer(t, sampleProjects())
	s := newTestScraper(ts)
	ch, err := s.ListScenes(context.Background(),
		"https://www.littlecaprice-dreams.com/collection/nope/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _, _ := collect(t, ch)
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes for an unknown collection", len(scenes))
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "unknown collection") {
		t.Fatalf("errors = %v, want one naming the unknown collection", errs)
	}
}

func TestModelURLFiltersTheWalk(t *testing.T) {
	ts := newWPServer(t, sampleProjects())
	s := newTestScraper(ts)
	ch, err := s.ListScenes(context.Background(),
		"https://www.littlecaprice-dreams.com/model/clemence-audiard/", scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _, _ := collect(t, ch)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// Clemence is on vid-one and on the gallery; only the video counts.
	if len(scenes) != 1 || scenes[0].ID != "1" {
		t.Fatalf("got %v, want just vid-one", scenes)
	}
}

// The REST listing is explicitly date-ordered, so a stored id means the rest of
// the walk is already held.
func TestKnownIDStopsEarly(t *testing.T) {
	ts := newWPServer(t, sampleProjects())
	s := newTestScraper(ts)
	ch, err := s.ListScenes(context.Background(), "https://www.littlecaprice-dreams.com/videos/",
		scraper.ListOpts{Workers: 2, KnownIDs: map[string]bool{"2": true}})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _, stopped := collect(t, ch)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !stopped {
		t.Error("no StoppedEarly emitted after hitting a known ID")
	}
	if len(scenes) != 1 || scenes[0].ID != "1" {
		t.Fatalf("got %v, want only the newer vid-one", scenes)
	}
	if got := ts.detailHits.Load(); got != 1 {
		t.Errorf("detail fetches = %d, want 1 — the known scene must cost no request", got)
	}
}

// Everything already stored: no scenes, but the run still reports the early
// stop so the coverage check knows why the result is small.
func TestAllKnownStillReportsStoppedEarly(t *testing.T) {
	ts := newWPServer(t, sampleProjects())
	s := newTestScraper(ts)
	ch, err := s.ListScenes(context.Background(), "https://www.littlecaprice-dreams.com/videos/",
		scraper.ListOpts{Workers: 1, KnownIDs: map[string]bool{"1": true, "2": true}})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _, stopped := collect(t, ch)
	if len(scenes) != 0 || len(errs) != 0 {
		t.Fatalf("scenes=%v errs=%v, want neither", scenes, errs)
	}
	if !stopped {
		t.Error("no StoppedEarly emitted when everything was already stored")
	}
}

// A taxonomy without the videos term means the content-type marker moved, and
// every gallery would otherwise be ingested as a scene.
func TestMissingVideosTermIsAParseFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "project_category") {
			_, _ = fmt.Fprint(w, `[{"id":28,"slug":"gallery","name":"Gallery"}]`)
			return
		}
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	s := New()
	s.Client = srv.Client()
	s.baseOverride = srv.URL

	ch, err := s.ListScenes(context.Background(), "https://www.littlecaprice-dreams.com/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _, _ := collect(t, ch)
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want 1", errs)
	}
	if got := scraper.Classify(errs[0]); got != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", got)
	}
}

func TestContextCancellationStopsTheScrape(t *testing.T) {
	ts := newWPServer(t, sampleProjects())
	s := newTestScraper(ts)
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, "https://www.littlecaprice-dreams.com/videos/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for range ch { //nolint:revive // draining is the point
	}
}

// porn-lifestyle.com is the Pornlifestyle sub-brand's own domain and the URL
// StashDB lists for it, but it 301s every path — REST routes included — to the
// collection page, so it must be addressable while never being fetched from.
func TestAliasHostResolvesToItsCollection(t *testing.T) {
	s := New()
	for _, u := range []string{"http://porn-lifestyle.com/", "https://www.porn-lifestyle.com"} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
		if got := s.base(u); got != defaultBase {
			t.Errorf("base(%q) = %q, want %q — the alias host cannot serve requests", u, got, defaultBase)
		}
	}
	if got := s.base("https://www.littlecaprice-dreams.com/videos/"); got != "https://www.littlecaprice-dreams.com" {
		t.Errorf("base = %q, want the operator's own host preserved", got)
	}
}

func TestAliasHostScrapesThePornlifestyleCollection(t *testing.T) {
	projects := []fakeProject{
		{id: 1, slug: "pl-one", title: "PL One", date: "2026-08-14T10:00:00",
			cats: []int{videosID, pornLifestyleID}, models: []string{"Marcello Bravo"}},
		{id: 2, slug: "other", title: "Other", date: "2026-08-13T10:00:00",
			cats: []int{videosID, buttmuseID}, models: []string{"Marcello Bravo"}},
	}
	ts := newWPServer(t, projects)
	s := newTestScraper(ts)

	ch, err := s.ListScenes(context.Background(), "http://porn-lifestyle.com/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _, _ := collect(t, ch)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(scenes) != 1 || scenes[0].ID != "1" {
		t.Fatalf("got %v, want just the Pornlifestyle project", scenes)
	}
	if scenes[0].Studio != "PornLifestyle" {
		t.Errorf("studio = %q, want PornLifestyle", scenes[0].Studio)
	}
}

// An empty result with no early stop is a template change, not an empty site,
// and must not be reported as a clean zero-scene success.
func TestEmptyListingIsReported(t *testing.T) {
	ts := newWPServer(t, []fakeProject{
		// Galleries only: the walk succeeds and yields no videos.
		{id: 3, slug: "gal-one", title: "Gal One", date: "2026-08-12T10:00:00",
			cats: []int{galleryID, buttmuseID}, models: []string{"X"}},
	})
	s := newTestScraper(ts)

	ch, err := s.ListScenes(context.Background(), "https://www.littlecaprice-dreams.com/videos/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _, _ := collect(t, ch)
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want 1", errs)
	}
	if got := scraper.Classify(errs[0]); got != scraper.FailureParse {
		t.Errorf("classified as %v, want FailureParse", got)
	}
}

// A listing fetch that fails must not also produce the "no videos" error —
// one cause, one message.
func TestListingFetchFailureIsNotDoubleReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "project_category") {
			_, _ = fmt.Fprintf(w, `[{"id":%d,"slug":"videos","name":"Videos"}]`, videosID)
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := New()
	s.Client = srv.Client()
	s.baseOverride = srv.URL

	ch, err := s.ListScenes(context.Background(), "https://www.littlecaprice-dreams.com/", scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	scenes, errs, _, _ := collect(t, ch)
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want exactly 1", errs)
	}
	if !strings.Contains(errs[0].Error(), "listing page 1") {
		t.Errorf("error = %q, want the transport failure", errs[0])
	}
}
