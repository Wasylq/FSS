package stasyqvr

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

const listingFixture = `
<div class="main-part__item">
  <div class="img-loader-wrap">
    <img src="https://stasyqvr.com/storage/vr/vr-covers/cover475.webp" alt="Fit temptress President Mermaid poses naked" style="width:100%;">
    <video src="https://vcdn.example/previews/475/preview.mp4"></video>
    <a class="main-part__item__link" href="https://stasyqvr.com/virtualreality/scene/id/475"></a>
  </div>
</div>
<h2>Fit temptress President Mermaid poses naked <a class="main-part__item__link" href="https://stasyqvr.com/virtualreality/scene/id/475"></a></h2>
<div class="main-part__item">
  <div class="img-loader-wrap">
    <img src="https://stasyqvr.com/storage/vr/vr-covers/cover474.webp" alt="Another VR Scene">
    <a class="main-part__item__link" href="https://stasyqvr.com/virtualreality/scene/id/474"></a>
  </div>
</div>
`

const detailFixture = `
<div class="main-desc">
  <div class="main-desc__date"> Jun 25, 2026 </div>
  <div class="main-desc__detail">
    <div class="detail-right">
      <h1>Fit temptress President Mermaid poses naked</h1>
      <a class="downloads-signup" href="/user/join">Sign Up</a>
      <p>She is a goddess, and she knows it. Her name is President Mermaid.</p>
    </div>
  </div>
</div>
`

func TestParseListing(t *testing.T) {
	scenes := parseListing(siteBase, []byte(listingFixture))
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes (deduped), got %d", len(scenes))
	}
	if scenes[0].id != "475" {
		t.Errorf("id = %q, want 475", scenes[0].id)
	}
	if scenes[0].url != "https://stasyqvr.com/virtualreality/scene/id/475" {
		t.Errorf("url = %q", scenes[0].url)
	}
	if scenes[0].title != "Fit temptress President Mermaid poses naked" {
		t.Errorf("title = %q", scenes[0].title)
	}
	if scenes[0].thumb != "https://stasyqvr.com/storage/vr/vr-covers/cover475.webp" {
		t.Errorf("thumb = %q", scenes[0].thumb)
	}
	if scenes[1].id != "474" {
		t.Errorf("second id = %q, want 474", scenes[1].id)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))
	if d.title != "Fit temptress President Mermaid poses naked" {
		t.Errorf("title = %q", d.title)
	}
	if d.date.Format("2006-01-02") != "2026-06-25" {
		t.Errorf("date = %v, want 2026-06-25", d.date)
	}
	if d.description == "" || d.description[:10] != "She is a g" {
		t.Errorf("description = %q", d.description)
	}
}

func TestTokenRegex(t *testing.T) {
	page := []byte(`<form><input type="hidden" name="_token" value="5AbER2LOuxUrvdbl2wMSAXa9RSHADTMjXnnHHBhR"></form>`)
	m := tokenRe.FindSubmatch(page)
	if m == nil || string(m[1]) != "5AbER2LOuxUrvdbl2wMSAXa9RSHADTMjXnnHHBhR" {
		t.Errorf("token extraction failed: %v", m)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://stasyqvr.com/":                           true,
		"https://stasyqvr.com/virtualreality/list?page=2": true,
		"https://stasyq.com/":                             false,
		"https://example.com/stasyqvr":                    false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

// --- end to end ---------------------------------------------------------------
//
// The tests above call the parsers directly, which left ListScenes, run,
// confirmAge, enqueueListing, fetchDetail and fetch at 0% and the package at
// 23.3% — the lowest in the repo. The base is now a field, so the full sequence
// runs offline: age-gate handshake, then the listing walk, then detail fetches.
func TestListScenesEndToEnd(t *testing.T) {
	// Handlers run on separate goroutines (Workers > 1), so these counters
	// must be atomic — a plain int++ here is a real data race.
	var gotToken atomic.Int32
	var gotConfirm atomic.Int32
	var listingHits atomic.Int32
	var detailHits atomic.Int32
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/age-confirmation":
			gotToken.Add(1)
			_, _ = fmt.Fprint(w, `<form><input name="_token" value="tok123" /></form>`)
		case "/age-confirm":
			gotConfirm.Add(1)
			if err := r.ParseForm(); err == nil && r.PostFormValue("_token") != "tok123" {
				t.Errorf("age-confirm posted _token = %q, want tok123", r.PostFormValue("_token"))
			}
			w.WriteHeader(http.StatusOK)
		case "/virtualreality/list":
			listingHits.Add(1)
			if r.URL.Query().Get("page") != "1" {
				_, _ = fmt.Fprint(w, `<div class="empty"></div>`)
				return
			}
			// The fixture holds absolute live hrefs and the scraper fetches them
			// verbatim, so the host must be rewritten or the detail fetches walk
			// straight out of this test and onto stasyqvr.com.
			_, _ = fmt.Fprint(w, strings.ReplaceAll(listingFixture, "https://stasyqvr.com", ts.URL))
		default:
			detailHits.Add(1)
			_, _ = fmt.Fprint(w, detailFixture)
		}
	}))
	defer ts.Close()

	s := New()
	s.base = ts.URL

	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) == 0 {
		t.Fatal("no scenes returned")
	}
	// The age gate must be negotiated before any listing request.
	if gotToken.Load() == 0 || gotConfirm.Load() == 0 {
		t.Errorf("age handshake: token=%d confirm=%d, want both non-zero", gotToken.Load(), gotConfirm.Load())
	}
	if listingHits.Load() == 0 || detailHits.Load() == 0 {
		t.Errorf("listing/detail fetches = %d/%d, want both non-zero", listingHits.Load(), detailHits.Load())
	}
	for _, sc := range scenes {
		if sc.Title == "" {
			t.Errorf("scene %q has empty Title", sc.ID)
		}
	}
}

// A missing _token must fail loudly rather than silently scraping an age-gated
// site and returning nothing.
func TestConfirmAgeMissingToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<form>no token here</form>`)
	}))
	defer ts.Close()

	s := New()
	s.base = ts.URL
	err := s.confirmAge(context.Background(), ts.Client())
	if err == nil {
		t.Fatal("confirmAge succeeded with no _token on the page")
	}
	if !strings.Contains(err.Error(), "_token") {
		t.Errorf("error = %v, want it to name the missing _token", err)
	}
}
