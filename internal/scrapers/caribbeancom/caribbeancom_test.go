package caribbeancom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// ---- fixtures ----

const (
	titleJP = "ハイパー美脚の痴女教師"
	descJP  = "成績が悪いので呼び出されました。"
	actorJP = "百多えみり"
)

func listingHTML() string {
	return `<html><body>
<div class="grid">
  <div class="media">
    <a href="/moviepages/052226-001/index.html">
      <img class="media-image" itemprop="thumbnail" src="https://tarimages.caribbeancom.com/images/flash256x144/222038.jpg" alt="x">
    </a>
    <a itemprop="url" href="/moviepages/052226-001/index.html">` + titleJP + `</a>
  </div>
  <div class="media">
    <a href="/moviepages/052326-001/index.html">
      <img class="media-image" itemprop="thumbnail" src="https://tarimages.caribbeancom.com/images/flash256x144/222039.jpg" alt="y">
    </a>
  </div>
</div>
</body></html>`
}

func detailHTML() string {
	return `<html><head><meta http-equiv="Content-Type" content="text/html; charset=euc-jp"></head><body>
<div class="movie-info section">
  <div class="heading"><h1 itemprop="name">` + titleJP + `</h1></div>
  <p itemprop="description">` + descJP + `</p>
  <ul itemscope itemtype="http://schema.org/VideoObject">
    <li class="movie-spec">
      <span class="spec-title">出演</span>
      <span class="spec-content">
        <a class="spec__tag" itemprop="actor" itemscope itemtype="http://schema.org/Person" href="/search_act/7539/1.html"><span itemprop="name">` + actorJP + `</span></a>
      </span>
    </li>
    <li class="movie-spec">
      <span class="spec-title">配信日</span>
      <span itemprop="uploadDate" itemprop="datePublished" class="spec-content">2026/05/22</span>
    </li>
    <li class="movie-spec">
      <span class="spec-title">再生時間</span>
      <span class="spec-content"><span itemprop="duration" content="T00H52M37S">00:52:37</span></span>
    </li>
  </ul>
</div><!-- /.movie-info -->
<div class="related">
  <a itemprop="actor" itemscope itemtype="http://schema.org/Person"><span itemprop="name">別の女優</span></a>
</div>
</body></html>`
}

// eucjp encodes a UTF-8 string to EUC-JP bytes (what the live site serves).
func eucjp(t *testing.T, s string) []byte {
	t.Helper()
	b, _, err := transform.Bytes(japanese.EUCJP.NewEncoder(), []byte(s))
	if err != nil {
		t.Fatalf("eucjp encode: %v", err)
	}
	return b
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.caribbeancom.com/listpages/all1.htm", true},
		{"https://caribbeancom.com/moviepages/052226-001/index.html", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestDecodeEUCJP ----

func TestDecodeEUCJP(t *testing.T) {
	got := string(decodeEUCJP(eucjp(t, titleJP)))
	if got != titleJP {
		t.Errorf("decodeEUCJP = %q, want %q", got, titleJP)
	}
}

// ---- TestFetchListing ----

func TestFetchListing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(eucjp(t, listingHTML()))
	}))
	defer ts.Close()

	s := &Scraper{Client: ts.Client()}
	items, err := s.fetchListing(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %+v", len(items), items)
	}
	if items[0].id != "052226-001" {
		t.Errorf("item0.id = %q", items[0].id)
	}
	if !strings.HasSuffix(items[0].thumb, "222038.jpg") {
		t.Errorf("item0.thumb = %q", items[0].thumb)
	}
}

// ---- TestToScene ----

func TestToScene(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(eucjp(t, detailHTML()))
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	it := listItem{id: "052226-001", thumb: "https://img/222038.jpg"}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	sc := s.toScene(context.Background(), "studioURL", it, now)

	if sc.ID != "052226-001" || sc.SiteID != siteID {
		t.Errorf("identity wrong: %+v", sc)
	}
	if sc.Title != titleJP {
		t.Errorf("Title = %q, want %q", sc.Title, titleJP)
	}
	if sc.Description != descJP {
		t.Errorf("Description = %q, want %q", sc.Description, descJP)
	}
	if sc.Thumbnail != it.thumb {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	// Only the in-section actor, not the related-movies actor.
	if len(sc.Performers) != 1 || sc.Performers[0] != actorJP {
		t.Errorf("Performers = %v, want [%s]", sc.Performers, actorJP)
	}
	wantDate := time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if sc.Duration != 52*60+37 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 52*60+37)
	}
}

// ---- TestListScenes (end-to-end) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/listpages/all1.htm"):
			_, _ = w.Write(eucjp(t, listingHTML()))
		case strings.HasPrefix(r.URL.Path, "/moviepages/"):
			_, _ = w.Write(eucjp(t, detailHTML()))
		default:
			// all2.htm etc. -> empty listing -> Done.
			_, _ = w.Write(eucjp(t, "<html><body>nothing</body></html>"))
		}
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
	if got["052226-001"] != titleJP {
		t.Errorf("scene title = %q", got["052226-001"])
	}
}
