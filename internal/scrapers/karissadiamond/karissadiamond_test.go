package karissadiamond

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://karissa-diamond.com/", true},
		{"https://karissa-diamond.com/videoCollection/", true},
		{"https://www.karissa-diamond.com/videoCollection/1664/Far-Away-II/", true},
		{"http://karissa-diamond.com/photoCollection/", true},
		{"https://www.mplstudios.com/portfolio/290-Karissa_Diamond/", false},
		{"https://karissadiamond.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- test server ----

// batches models the live endpoint: it answers offset a=N with a slice of the
// corpus and reports where the next batch starts, exactly like loadMore.php.
type fakeSite struct {
	corpus      []string // titles, newest first
	batch       int
	detailHits  atomic.Int32
	listingHits atomic.Int32
}

func (f *fakeSite) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/workFiles/loadMore.php" {
			f.listingHits.Add(1)
			off, _ := strconv.Atoi(r.URL.Query().Get("a"))
			end := min(off+f.batch, len(f.corpus))
			if off > len(f.corpus) {
				off = len(f.corpus)
			}

			var sb strings.Builder
			sb.WriteString("[[")
			for i := off; i < end; i++ {
				if i > off {
					sb.WriteString(",")
				}
				id := 1600 + i
				fmt.Fprintf(&sb, `{"id":%d,"title":%q,"relDate":"October %02d, 2019","link":"/videoCollection/%d/Slug/","collected":false}`,
					id, f.corpus[i], (i%28)+1, id)
			}
			fmt.Fprintf(&sb, "],%d,false]", end)
			_, _ = fmt.Fprint(w, sb.String())
			return
		}
		f.detailHits.Add(1)
		_, _ = fmt.Fprint(w, `<div class="col-md-6 twoem">
			<span id="videoDuration"><i class="far fa-clock"></i> 6:20</span>
		</div>`)
	})
}

func newFakeSite(t *testing.T, corpus []string, batch int) (*fakeSite, *Scraper) {
	t.Helper()
	f := &fakeSite{corpus: corpus, batch: batch}
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)

	orig := siteBase
	siteBase = srv.URL
	t.Cleanup(func() { siteBase = orig })

	s := New()
	s.Client = srv.Client()
	return f, s
}

// ---- TestRunWalksAllOffsets ----

func TestRunWalksAllOffsets(t *testing.T) {
	corpus := make([]string, 28)
	for i := range corpus {
		corpus[i] = fmt.Sprintf("Video %d", i)
	}
	f, s := newFakeSite(t, corpus, 10)

	scenes, _ := collect(t, s, siteBase+"/videoCollection/")

	if len(scenes) != 28 {
		t.Fatalf("got %d scenes, want 28", len(scenes))
	}
	// 10 + 10 + 8, then one empty batch that ends the walk.
	if got := f.listingHits.Load(); got != 4 {
		t.Errorf("listing fetches = %d, want 4", got)
	}
	if got := f.detailHits.Load(); got != 28 {
		t.Errorf("detail fetches = %d, want 28", got)
	}

	// Offsets must advance: no scene may repeat.
	seen := make(map[string]bool)
	for _, sc := range scenes {
		if seen[sc.ID] {
			t.Fatalf("duplicate scene ID %q — offset failed to advance", sc.ID)
		}
		seen[sc.ID] = true
	}
}

// ---- TestSceneFields ----

func TestSceneFields(t *testing.T) {
	_, s := newFakeSite(t, []string{"Far & Away II"}, 10)

	scenes, _ := collect(t, s, siteBase+"/videoCollection/")
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != "1600" {
		t.Errorf("ID = %q, want 1600", sc.ID)
	}
	if sc.SiteID != siteID {
		t.Errorf("SiteID = %q, want %q", sc.SiteID, siteID)
	}
	// The API double-encodes nothing, but titles do carry HTML entities.
	if sc.Title != "Far & Away II" {
		t.Errorf("Title = %q, want %q", sc.Title, "Far & Away II")
	}
	if sc.Studio != studioName {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != performerName {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Duration != 380 {
		t.Errorf("Duration = %d, want 380", sc.Duration)
	}
	want := time.Date(2019, time.October, 1, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}
	if !strings.HasSuffix(sc.Thumbnail, "/media/video/1600/cover_720.jpg") {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if !strings.HasSuffix(sc.URL, "/videoCollection/1600/Slug/") {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt is zero")
	}
}

// ---- TestTitleEntitiesUnescaped ----

func TestTitleEntitiesUnescaped(t *testing.T) {
	_, s := newFakeSite(t, []string{"Rise &amp; Shine"}, 10)

	scenes, _ := collect(t, s, siteBase+"/videoCollection/")
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
	if scenes[0].Title != "Rise & Shine" {
		t.Errorf("Title = %q, want %q", scenes[0].Title, "Rise & Shine")
	}
}

// ---- TestStalledOffsetTerminates ----

// If the endpoint ever stops advancing its next-offset, the loop must emit the
// batch it has and stop rather than request the same offset forever.
func TestStalledOffsetTerminates(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/workFiles/loadMore.php" {
			hits.Add(1)
			// Always reports next offset 0 — never advances.
			_, _ = fmt.Fprint(w, `[[{"id":1,"title":"Stuck","relDate":"May 1, 2019","link":"/videoCollection/1/Stuck/"}],0,false]`)
			return
		}
		_, _ = fmt.Fprint(w, `<span id="videoDuration"> 1:00</span>`)
	}))
	defer srv.Close()

	orig := siteBase
	siteBase = srv.URL
	defer func() { siteBase = orig }()

	s := New()
	s.Client = srv.Client()

	done := make(chan struct{})
	var scenes []models.Scene
	go func() {
		defer close(done)
		scenes, _ = collect(t, s, srv.URL+"/videoCollection/")
	}()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("scraper did not terminate on a stalled offset")
	}

	if len(scenes) != 1 {
		t.Errorf("got %d scenes, want 1", len(scenes))
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("listing fetches = %d, want 1", got)
	}
}

// ---- TestMalformedResponse ----

func TestMalformedResponse(t *testing.T) {
	for _, body := range []string{`[]`, `{"nope":1}`, `[[],"notanumber",false]`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, body)
			}))
			defer srv.Close()

			orig := siteBase
			siteBase = srv.URL
			defer func() { siteBase = orig }()

			s := New()
			s.Client = srv.Client()

			ch, err := s.ListScenes(context.Background(), srv.URL+"/videoCollection/", scraper.ListOpts{})
			if err != nil {
				t.Fatal(err)
			}
			sawErr := false
			for res := range ch {
				if res.Kind == scraper.KindError {
					sawErr = true
				}
			}
			if !sawErr {
				t.Errorf("malformed body %q produced no error result", body)
			}
		})
	}
}

// ---- TestContextCancellation ----

func TestContextCancellation(t *testing.T) {
	corpus := make([]string, 40)
	for i := range corpus {
		corpus[i] = "V"
	}
	_, s := newFakeSite(t, corpus, 10)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := s.ListScenes(ctx, siteBase+"/videoCollection/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	cancel()

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
