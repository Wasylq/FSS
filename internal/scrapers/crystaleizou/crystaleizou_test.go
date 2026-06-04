package crystaleizou

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
		{"https://www.crystal-eizou.jp/", true},
		{"http://www.crystal-eizou.jp/info/", true},
		{"https://crystal-eizou.jp/info/archive/2025_06.html", true},
		{"https://example.com/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseArchiveMonths(t *testing.T) {
	html := `<p><a href="archive/2023_01.html">2023年1月</a></p>
<p><a href="archive/2023_02.html">2023年2月</a></p>
<p><a href="archive/2023_01.html">2023年1月</a></p>`

	months := parseArchiveMonths([]byte(html))
	if len(months) != 2 {
		t.Fatalf("got %d months, want 2 (deduped)", len(months))
	}
	if months[0] != "2023_01" || months[1] != "2023_02" {
		t.Errorf("months = %v", months)
	}
}

func TestParseProducts(t *testing.T) {
	html := `<div class="itemSection clearfix">
<div class="left2">
<a href="img/dvd/202506/zoom/EKDV-781.jpg" class="zoomImg"><img src="img/dvd/202506/EKDV-781.jpg" width="120" height="170"/></a>
</div>
<div class="right2">
<p class="strong fsize14 mar10">テスト作品タイトル</p>
<p class="green mar10">発売日：2025/6/10　レーベル：e-kiss　品番：EKDV-781　<br />
時間：190分　価格：2,980円（税抜） 3,278円（税込）</p>
<p class="mar10">作品の説明文です。</p>
<p class="strong">【出演女優】テスト女優（87cm・g-cup）</p>
</div>
</div>`

	items := parseProducts([]byte(html))
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	p := items[0]

	if p.code != "EKDV-781" {
		t.Errorf("code = %q", p.code)
	}
	if p.title != "テスト作品タイトル" {
		t.Errorf("title = %q", p.title)
	}
	if p.label != "e-kiss" {
		t.Errorf("label = %q", p.label)
	}
	if p.date.Format("2006-01-02") != "2025-06-10" {
		t.Errorf("date = %v", p.date)
	}
	if p.duration != 190*60 {
		t.Errorf("duration = %d, want %d", p.duration, 190*60)
	}
	if p.price != 3278 {
		t.Errorf("price = %f", p.price)
	}
	if p.performer != "テスト女優" {
		t.Errorf("performer = %q", p.performer)
	}
	if p.description != "作品の説明文です。" {
		t.Errorf("description = %q", p.description)
	}
	if p.thumbnail != "img/dvd/202506/EKDV-781.jpg" {
		t.Errorf("thumbnail = %q", p.thumbnail)
	}
}

func TestCleanPerformer(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"テスト女優（87cm・g-cup）", "テスト女優"},
		{"女優A（82cm・E-cup）、女優B（84cm・D-cup）", "女優A、女優B"},
		{"ソロ", "ソロ"},
	}
	for _, c := range cases {
		got := cleanPerformer(c.input)
		if got != c.want {
			t.Errorf("cleanPerformer(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestSplitPerformers(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"女優A、女優B", 2},
		{"女優A／女優B", 2},
		{"ソロ", 1},
	}
	for _, c := range cases {
		got := splitPerformers(c.input)
		if len(got) != c.want {
			t.Errorf("splitPerformers(%q) len = %d, want %d", c.input, len(got), c.want)
		}
	}
}

func mainPageHTML() string {
	return `<html><body>
<div id="side">
<p><a href="archive/2025_04.html">2025年4月</a></p>
<p><a href="archive/2025_05.html">2025年5月</a></p>
</div>
<div id="cotents">
<div class="itemSection clearfix">
<div class="left2"><img src="img/dvd/202506/EKDV-820.jpg"/></div>
<div class="right2">
<p class="strong fsize14 mar10">June Scene</p>
<p class="green mar10">発売日：2025/6/9　レーベル：e-kiss　品番：EKDV-820　時間：140分　価格：2,980円（税抜） 3,278円（税込）</p>
<p class="mar10">Description</p>
<p class="strong">【出演女優】June Actress</p>
</div>
</div>
</div></body></html>`
}

func archivePageHTML(code, title, date string) string {
	return fmt.Sprintf(`<html><body>
<div class="itemSection clearfix">
<div class="left2"><img src="img/dvd/202505/%s.jpg"/></div>
<div class="right2">
<p class="strong fsize14 mar10">%s</p>
<p class="green mar10">発売日：%s　レーベル：NITRO　品番：%s　時間：120分　価格：2,980円（税抜） 3,278円（税込）</p>
<p class="mar10">Old description</p>
<p class="strong">【出演女優】Old Actress</p>
</div>
</div>
</body></html>`, code, title, date, code)
}

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, mainPageHTML())
		case "/archive/2025_05.html":
			_, _ = fmt.Fprint(w, archivePageHTML("NITR-550", "May Scene", "2025/5/10"))
		case "/archive/2025_04.html":
			_, _ = fmt.Fprint(w, archivePageHTML("MADV-580", "April Scene", "2025/4/10"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), "https://crystal-eizou.jp/", scraper.ListOpts{})
	if err != nil {
		t.Fatal(err)
	}

	results := testutil.CollectScenes(t, ch)
	if len(results) != 3 {
		t.Fatalf("got %d scenes, want 3", len(results))
	}
	if results[0].ID != "EKDV-820" {
		t.Errorf("first ID = %q, want EKDV-820", results[0].ID)
	}
	if results[0].Duration != 140*60 {
		t.Errorf("duration = %d", results[0].Duration)
	}
	if results[2].ID != "MADV-580" {
		t.Errorf("last ID = %q, want MADV-580", results[2].ID)
	}
}

func TestKnownIDsStopsEarly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, mainPageHTML())
		case "/archive/2025_05.html":
			_, _ = fmt.Fprint(w, archivePageHTML("NITR-550", "May", "2025/5/10"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), base: ts.URL}
	ch, err := s.ListScenes(context.Background(), "https://crystal-eizou.jp/", scraper.ListOpts{
		KnownIDs: map[string]bool{"NITR-550": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	results, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly")
	}
	if len(results) != 1 || results[0].ID != "EKDV-820" {
		t.Errorf("got %v, want [EKDV-820]", results)
	}
}
