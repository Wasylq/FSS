package indiesav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.indies-av.co.jp/lables/ymdd/", true},
		{"https://www.indies-av.co.jp/lables/ymds/", true},
		{"https://www.indies-av.co.jp/genre/soap/", true},
		{"https://www.indies-av.co.jp/actress/asami_mizuhata/", true},
		{"https://indies-av.co.jp/", true},
		{"https://example.com/indies-av", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseListingPage(t *testing.T) {
	body := []byte(`
<ul>
<li class="col-md-3 package">
  <a class="d-flex flex-wrap" href="https://www.indies-av.co.jp/title/ymdd497/">
    <div class="col-4 px-0 mb-2 col-md-12 packagewrap-obj">
      <meta itemprop="name" content="Test Title One" />
      <meta itemprop="releaseDate" content="2026-06-02" />
      <meta itemprop="description" content="Test description" />
      <meta itemprop="url" content="https://www.indies-av.co.jp/title/ymdd497/" />
      <img itemprop="image" src="https://www.indies-av.co.jp/wp-content/uploads/Package/YMDD-497/YMDD-497_250.jpg" alt="Test" class="w-100">
    </div>
    <div class="col-8 px-0 col-md-12 pl-md-0 pl-3" itemscope itemtype="http://schema.org/Offer" itemprop="offers">
      <meta itemprop="priceCurrency" content="JPY" />
      <meta itemprop="price" content="3180" />
      <meta itemprop="gtin13" content="4573653441453" />
      <meta itemprop="sku" content="YMDD-497" />
      <span class="badge badge-pill badge-primary mb-1">YMDD-497</span>
      <p class="h6">Test Title One</p>
    </div>
  </a>
</li>
<li class="col-md-3 package">
  <a class="d-flex flex-wrap" href="https://www.indies-av.co.jp/title/ymds282/">
    <div class="col-4 px-0 mb-2 col-md-12 packagewrap-obj">
      <meta itemprop="name" content="Test Title Two" />
      <meta itemprop="releaseDate" content="2026-05-01" />
      <meta itemprop="description" content="Second description" />
      <meta itemprop="url" content="https://www.indies-av.co.jp/title/ymds282/" />
      <img itemprop="image" src="https://www.indies-av.co.jp/wp-content/uploads/Package/YMDS-282/YMDS-282_250.jpg" alt="Test" class="w-100">
    </div>
    <div class="col-8 px-0 col-md-12 pl-md-0 pl-3" itemscope itemtype="http://schema.org/Offer" itemprop="offers">
      <meta itemprop="priceCurrency" content="JPY" />
      <meta itemprop="price" content="2480" />
      <meta itemprop="sku" content="YMDS-282" />
      <span class="badge badge-pill badge-primary mb-1">YMDS-282</span>
    </div>
  </a>
</li>
</ul>
`)
	items := parseListingPage(body)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	i := items[0]
	if i.sku != "YMDD-497" {
		t.Errorf("sku = %q", i.sku)
	}
	if i.title != "Test Title One" {
		t.Errorf("title = %q", i.title)
	}
	if i.date != "2026-06-02" {
		t.Errorf("date = %q", i.date)
	}
	if i.description != "Test description" {
		t.Errorf("description = %q", i.description)
	}
	if i.url != "https://www.indies-av.co.jp/title/ymdd497/" {
		t.Errorf("url = %q", i.url)
	}
	if i.image != "https://www.indies-av.co.jp/wp-content/uploads/Package/YMDD-497/YMDD-497_250.jpg" {
		t.Errorf("image = %q", i.image)
	}
	if i.price != 3180 {
		t.Errorf("price = %f, want 3180", i.price)
	}

	i2 := items[1]
	if i2.sku != "YMDS-282" {
		t.Errorf("sku = %q", i2.sku)
	}
	if i2.price != 2480 {
		t.Errorf("price = %f, want 2480", i2.price)
	}
}

func TestParseDetailPage(t *testing.T) {
	body := []byte(`
<li class="d-flex px-0 px-md-2 ">
  <div class="col-md-2 col-4 p-2 border-bottom border-bottom-1 text-center bg-light">
    <span class="h6">女優名</span>
  </div>
  <div class="col-md-10 col-8 p-2 pl-3" itemscope itemtype="http://schema.org/Person">
    <span class="h6">糸井瑠花/桜ゆの</span>
  </div>
</li>
<li class="d-flex px-0 px-md-2 ">
  <div class="col-md-2 col-4 p-2 border-bottom border-bottom-1 text-center bg-light">
    <span class="h6">収録時間</span>
  </div>
  <div class="col-md-10 col-8 p-2 pl-3">
    <span class="h6" itemprop="offers">200分</span>
  </div>
</li>
<li class="d-flex px-0 px-md-2">
  <div class="col-md-10 col-8 p-2 pl-3">
    <span class="h6" itemprop="offers">
      <span style=display:none>ジャンル</span><a href="/genre/soap/">ソープ</a>、<a href="/genre/internal_ejaculation/">中出し</a>、<a href="/genre/uniform/">制服</a>
    </span>
  </div>
</li>
<li class="d-flex px-0 px-md-2">
  <div class="col-md-10 col-8 p-2 pl-3">
    <span class="h6" itemprop="offers" itemscope itemtype="http://schema.org/Offer">
      <span style=display:none>レーベル</span><a href="/lables/ymdd/">若桃</a>
    </span>
  </div>
</li>
`)
	d := parseDetailPage(body)
	if len(d.performers) != 2 || d.performers[0] != "糸井瑠花" || d.performers[1] != "桜ゆの" {
		t.Errorf("performers = %v", d.performers)
	}
	if d.duration != 12000 {
		t.Errorf("duration = %d, want 12000", d.duration)
	}
	if len(d.genres) != 3 || d.genres[0] != "ソープ" {
		t.Errorf("genres = %v", d.genres)
	}
	if d.label != "若桃" {
		t.Errorf("label = %q", d.label)
	}
}

func TestParseDetailPageSinglePerformer(t *testing.T) {
	body := []byte(`
<li class="d-flex px-0 px-md-2 ">
  <div class="col-md-2 col-4 p-2 border-bottom border-bottom-1 text-center bg-light">
    <span class="h6">女優名</span>
  </div>
  <div class="col-md-10 col-8 p-2 pl-3" itemscope itemtype="http://schema.org/Person">
    <span class="h6">真野祈</span>
  </div>
</li>
<li class="d-flex px-0 px-md-2 ">
  <div class="col-md-2 col-4 p-2 border-bottom border-bottom-1 text-center bg-light">
    <span class="h6">収録時間</span>
  </div>
  <div class="col-md-10 col-8 p-2 pl-3">
    <span class="h6" itemprop="offers">130分</span>
  </div>
</li>
`)
	d := parseDetailPage(body)
	if len(d.performers) != 1 || d.performers[0] != "真野祈" {
		t.Errorf("performers = %v", d.performers)
	}
	if d.duration != 7800 {
		t.Errorf("duration = %d, want 7800", d.duration)
	}
}

func TestExtractLastPage(t *testing.T) {
	body := []byte(`<a href="/lables/ymdd/page/2/">2</a><a href="/lables/ymdd/page/3/">3</a><a href="/lables/ymdd/page/12/">12</a>`)
	if got := extractLastPage(body); got != 12 {
		t.Errorf("extractLastPage = %d, want 12", got)
	}
}

func TestHasNextPage(t *testing.T) {
	body := []byte(`<a href="/page/2/">2</a><a href="/page/3/">3</a>`)
	if !hasNextPage(body, 1) {
		t.Error("expected next from page 1")
	}
	if hasNextPage(body, 3) {
		t.Error("expected no next from page 3")
	}
}

func TestResolveListingURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://www.indies-av.co.jp/lables/ymdd/", "https://www.indies-av.co.jp/lables/ymdd/"},
		{"https://www.indies-av.co.jp/lables/ymds/", "https://www.indies-av.co.jp/lables/ymds/"},
		{"https://www.indies-av.co.jp/genre/soap/", "https://www.indies-av.co.jp/genre/soap/"},
		{"https://www.indies-av.co.jp/actress/test/", "https://www.indies-av.co.jp/actress/test/"},
		{"https://www.indies-av.co.jp/", "https://www.indies-av.co.jp/lables/ymdd/"},
	}
	for _, c := range cases {
		base := resolveBase(c.url)
		if got := resolveListingURL(c.url, base); got != c.want {
			t.Errorf("resolveListingURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

const listingItemTpl = `<li class="col-md-3 package">
<a href="/title/%s/">
<div class="packagewrap-obj">
<meta itemprop="name" content="Title %s" />
<meta itemprop="releaseDate" content="2026-01-01" />
<meta itemprop="description" content="Desc" />
<meta itemprop="url" content="%s/title/%s/" />
<img itemprop="image" src="%s/img/%s.jpg" class="w-100">
</div>
<div itemscope itemprop="offers">
<meta itemprop="price" content="1000" />
<meta itemprop="sku" content="%s" />
</div></a></li>`

const detailTpl = `
<li class="d-flex"><div><span class="h6">女優名</span></div>
<div itemscope itemtype="http://schema.org/Person"><span class="h6">Test Model</span></div></li>
<li class="d-flex"><div><span class="h6">収録時間</span></div>
<div><span class="h6" itemprop="offers">120分</span></div></li>
<li class="d-flex"><div><span class="h6" itemprop="offers">
<span style=display:none>ジャンル</span><a href="/genre/a/">TagA</a>、<a href="/genre/b/">TagB</a>
</span></div></li>
<li class="d-flex"><div><span class="h6" itemprop="offers" itemscope>
<span style=display:none>レーベル</span><a href="/lables/test/">TestLabel</a></span></div></li>
`

func newTestServer(skus [][]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		switch {
		case r.URL.Path == "/lables/test/" || (len(r.URL.Path) > 13 && r.URL.Path[:13] == "/lables/test/"):
			pg := 0
			if m := pageNumRe.FindStringSubmatch(r.URL.Path); m != nil {
				pg, _ = fmt.Sscanf(m[1], "%d", &pg)
				pg, _ = fmt.Sscan(m[1], &pg)
			}
			if pg == 0 {
				pg = 1
			}
			idx := pg - 1
			if idx >= len(skus) {
				_, _ = fmt.Fprint(w, "<div>empty</div>")
				return
			}
			base := "http://" + r.Host
			var items string
			for _, sku := range skus[idx] {
				items += fmt.Sprintf(listingItemTpl, sku, sku, base, sku, base, sku, sku)
			}
			nav := ""
			if len(skus) > 1 {
				for p := 2; p <= len(skus); p++ {
					nav += fmt.Sprintf(`<a href="/lables/test/page/%d/">%d</a>`, p, p)
				}
			}
			_, _ = fmt.Fprintf(w, "<ul>%s</ul><nav>%s</nav>", items, nav)

		case len(r.URL.Path) > 7 && r.URL.Path[:7] == "/title/":
			_, _ = fmt.Fprint(w, detailTpl)

		default:
			_, _ = fmt.Fprint(w, "<div>empty</div>")
		}
	}))
}

func TestRun(t *testing.T) {
	ts := newTestServer([][]string{{"YMDD-100", "YMDD-200"}})
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/lables/test/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 2 {
		t.Fatalf("got %d scenes, want 2", len(results))
	}

	found := map[string]bool{}
	for _, r := range results {
		found[r.ID] = true
		if r.Duration != 7200 {
			t.Errorf("scene %s duration = %d, want 7200", r.ID, r.Duration)
		}
		if r.Studio != "Momotaro Eizo" {
			t.Errorf("studio = %q", r.Studio)
		}
		if len(r.Tags) != 2 {
			t.Errorf("tags = %v, want 2", r.Tags)
		}
	}
	if !found["YMDD-100"] || !found["YMDD-200"] {
		t.Errorf("missing scenes: %v", found)
	}
}

func TestRunKnownIDs(t *testing.T) {
	ts := newTestServer([][]string{{"YMDD-100", "YMDD-200", "YMDD-300"}})
	defer ts.Close()

	s := &Scraper{client: ts.Client()}
	ch, err := s.ListScenes(context.Background(), ts.URL+"/lables/test/", scraper.ListOpts{
		KnownIDs: map[string]bool{"YMDD-200": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(results) != 1 {
		t.Fatalf("got %d scenes, want 1", len(results))
	}
	if results[0].ID != "YMDD-100" {
		t.Errorf("expected YMDD-100, got %s", results[0].ID)
	}
}
