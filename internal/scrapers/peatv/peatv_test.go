package peatv

import (
	"testing"
	"time"
)

const fixtureListing = `
<html><body>
<div style="text-align:right">93 件中 1 ～ 25 件目を表示中</div>
<div style="text-align:right;margin-bottom:10px">
  <span style="font-weight:normal;">1</span>&nbsp;
  <a href="/search.php?b=1&amp;p=2" title="page 2">2</a>&nbsp;
  <a href="/search.php?b=1&amp;p=4" title="last page">【最後へ】</a>
</div>
<div class="row featurette">
<div class="hori5">
<a href="https://pea-tv.jp/monthly_detail.php?code=WA-582"><img class="featurette-image img-responsive prod_img" src="./pic_base/product/WA-582/wa-582_pickup.jpg" alt="素人妻ナンパ 全員生中出し ５時間 セレブＤＸ１０４"></a>
<img src="pic_base/icon/icon_hd.gif" class="pea_icon">
<h4 style="height:2em;"><a href="https://pea-tv.jp/monthly_detail.php?code=WA-582">素人妻ナンパ 全員生中出し ...</a></h4>
<p>■----</p><p>■WA-582</p><p>■300分</p>
</div>
<div class="hori5">
<a href="https://pea-tv.jp/monthly_detail.php?code=ZEX-337"><img class="featurette-image img-responsive prod_img" src="./pic_base/product/ZEX-337/zex-337_pickup.jpg" alt="某１８禁動画サイトで話題の乳首舐め"></a>
<h4 style="height:2em;"><a href="https://pea-tv.jp/monthly_detail.php?code=ZEX-337">某１８禁動画サイトで...</a></h4>
<p>■----</p><p>■ZEX-337</p><p>■120分</p>
</div>
</div>
</body></html>
`

func TestParseListingPage(t *testing.T) {
	items, total, lastPage := parseListingPage([]byte(fixtureListing))

	if total != 93 {
		t.Errorf("total = %d, want 93", total)
	}
	if lastPage != 4 {
		t.Errorf("lastPage = %d, want 4", lastPage)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	c := items[0]
	if c.code != "WA-582" {
		t.Errorf("code = %q", c.code)
	}
	if c.title != "素人妻ナンパ 全員生中出し ５時間 セレブＤＸ１０４" {
		t.Errorf("title = %q", c.title)
	}
	if c.thumbnail != "https://pea-tv.jp/pic_base/product/WA-582/wa-582_pickup.jpg" {
		t.Errorf("thumbnail = %q", c.thumbnail)
	}
	if c.duration != 300*60 {
		t.Errorf("duration = %d, want %d", c.duration, 300*60)
	}

	c2 := items[1]
	if c2.code != "ZEX-337" {
		t.Errorf("code = %q", c2.code)
	}
	if c2.duration != 120*60 {
		t.Errorf("duration = %d, want %d", c2.duration, 120*60)
	}
}

const fixtureDetail = `
<html><head>
<title>[HD高画質]素人妻ナンパ...(WA-582) -  -  - AV動画 - PEA-TV【ピー・ティーヴィ】</title>
</head><body>
<table class="table">
<tr><td colspan="1">品番</td><td colspan="3">WA-582</td></tr>
<tr><td colspan="1">配信開始日</td><td colspan="3">2026年4月24日</td></tr>
<tr><td colspan="1">通販開始日</td><td colspan="3">2026年4月24日</td></tr>
<tr><td colspan="1">再生時間</td><td colspan="3">300分</td></tr>
<tr><td colspan="1">レーベル</td><td colspan="3">----</td></tr>
<tr><td colspan="1">シリーズ</td><td colspan="3">----</td></tr>
<tr><td colspan="4"><p class="text-justify">素人妻ナンパの人気シリーズ。今回もセレブな奥様たちを街でナンパ。</p></td></tr>
</table>
</body></html>
`

func TestParseDetailDate(t *testing.T) {
	d, ok := parseDetailDate([]byte(fixtureDetail))
	if !ok {
		t.Fatal("parseDetailDate returned false")
	}
	want := time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC)
	if d != want {
		t.Errorf("date = %v, want %v", d, want)
	}
}

func TestParseDetailDateMailOrderFallback(t *testing.T) {
	html := `<table>
<tr><td colspan="1">配信開始日</td><td colspan="3">-</td></tr>
<tr><td colspan="1">通販開始日</td><td colspan="3">2025年12月5日</td></tr>
</table>`
	d, ok := parseDetailDate([]byte(html))
	if !ok {
		t.Fatal("parseDetailDate returned false")
	}
	want := time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC)
	if d != want {
		t.Errorf("date = %v, want %v", d, want)
	}
}

func TestParseJPDate(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"2026年4月24日", time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC), true},
		{"2020年12月1日", time.Date(2020, 12, 1, 0, 0, 0, 0, time.UTC), true},
		{"-", time.Time{}, false},
		{"", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := parseJPDate(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseJPDate(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestBuildListingURL(t *testing.T) {
	cases := []struct {
		studioURL string
		want      string
	}{
		{"https://pea-tv.jp/", "https://pea-tv.jp/search.php"},
		{"https://pea-tv.jp/top_index.php", "https://pea-tv.jp/search.php"},
		{"https://pea-tv.jp/search.php?b=1", "https://pea-tv.jp/search.php?b=1"},
		{"https://pea-tv.jp/search.php?b=7&p=3", "https://pea-tv.jp/search.php?b=7"},
		{"https://pea-tv.jp/search.php", "https://pea-tv.jp/search.php"},
	}
	for _, c := range cases {
		got := buildListingURL(c.studioURL)
		if got != c.want {
			t.Errorf("buildListingURL(%q) = %q, want %q", c.studioURL, got, c.want)
		}
	}
}

func TestPageURL(t *testing.T) {
	cases := []struct {
		base string
		page int
		want string
	}{
		{"https://pea-tv.jp/search.php", 1, "https://pea-tv.jp/search.php"},
		{"https://pea-tv.jp/search.php", 2, "https://pea-tv.jp/search.php?p=2"},
		{"https://pea-tv.jp/search.php?b=1", 3, "https://pea-tv.jp/search.php?b=1&p=3"},
	}
	for _, c := range cases {
		got := pageURL(c.base, c.page)
		if got != c.want {
			t.Errorf("pageURL(%q, %d) = %q, want %q", c.base, c.page, got, c.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://pea-tv.jp/", true},
		{"https://pea-tv.jp/search.php?b=1", true},
		{"https://pea-tv.jp/top_index.php", true},
		{"http://www.pea-tv.jp/", true},
		{"https://example.com/", false},
		{"https://pea-tvfake.jp/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}
