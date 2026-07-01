package digitaljmediautil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

var now = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// ---- fixtures (trimmed from the live markup of each site) ----

const fellatioBlock = `
<div class="tour-data">
    <h2><a href="girl/Rio">Rio</a></h2>
    <div class="data silver">12:22 / 121 photos / <a href="tag/mouth-cum">mouth cum</a> / <a href="tag/POV">POV</a></div>
</div>
<div class="player" style="background:url(https://cdn.fellatiojapan.com/preview/644_Rio_3F4G/scene-lg.jpg) 0 0 no-repeat;">
    <video><source type="video/mp4" src="https://cdn.fellatiojapan.com/preview/644_Rio_3F4G/sample.mp4"></video>
</div>`

const cospuriBlock = `
<div class="scene cosplay">
    <div class="scene-thumb" style="background:url(https://cdn.cospuri.com/preview/0548cpar/scene-med.jpg) 0 0 no-repeat;">
        <a href="/sample?id=0548cpar"></a>
    </div>
    <div class="info">
        <div class="model"><a href="/model/Emiri-Momota">Emiri Momota</a><a class="channel" href="/samples?channel=cosplay">cosplay</a></div>
        <div class="length"><strong>27</strong>min</div>
        <div class="photos"><strong>100</strong>pics</div>
        <div class="tags"><a class="tag" href="/samples?tag=Latex">Latex</a><a class="tag" href="/samples?tag=Sex">Sex</a></div>
    </div>
</div>`

const cuteButtsBlock = `
<div class="scene">
    <div class="scene-thumb" style="background:url(https://cdn.cutebutts.com/preview/0127hsrh/scene-med.jpg) 0 0 no-repeat;">
        <div class="date tag">2026・01・23</div>
        <a class="id" href="/sample/0127hsrh/Teachers-Pet"></a>
        <div class="tag-box"><a class="tag" href="/samples?tag=schoolgirl">schoolgirl</a><a class="tag" href="/samples?tag=sex">sex</a></div>
    </div>
    <h3 class="title"><a href="/sample/0127hsrh/Teachers-Pet">Teacher&#039;s Pet</a></h3>
    <h4 class="model"><a href="/model/Mari-Hirose">Mari Hirose</a> & <a href="/model/Ria-Kurumi">Ria Kurumi</a></h4>
</div>`

const cuteButtsDetailHTML = `<div class="scene-info"><div class="details">
<span><strong>Date:</strong> 2026・01・23</span>
<span><strong>Runtime:</strong> 31 min</span>
<span><strong>Photos:</strong> 142</span>
</div></div>`

const cumBuffetBlock = `
<div class="video">
    <a href="/sample/0150zrt7/Dinner-Time-For-Dalila-Dark"><img src="https://cdn.cumbuffet.com/preview/0150zrt7/scene-sm.jpg" class="thumb"></a>
    <div class="meta">
        <a href="/sample/0150zrt7/Dinner-Time-For-Dalila-Dark" class="video-link">Dinner Time For Dalila Dark</a>
        <div class="model-name"><a href="/girl/Dalila-Dark">Dalila Dark</a></div>
        <div class="date">Jan 12, 2024</div>
    </div>
</div>`

const cumBuffetDetailHTML = `<aside><div class="tags"><strong class="title">Tags:</strong>
<ul class="tag-list">
<li><a href="/samples?tag=Petite" class="tag">Petite</a></li>
<li><a href="/samples?tag=POV" class="tag">POV</a></li>
</ul></div></aside>`

const legsJapanBlock = `
<div class="player" style="background:url(https://cdn.legsjapan.com/samples/1170mmom/scene-lg.jpg) 0 0 no-repeat;">
    <video><source type="video/mp4" src="https://cdn.legsjapan.com/samples/1170mmom/sample.mp4"></video>
</div>
<div class="tContent left">
    <a href="girl/Ria-Kurumi"><h1>Ria Kurumi</h1></a>
    <h3><strong>Footjob in Skirt</strong></h3>
    <h3>length:<strong>11:06</strong> </h3><h3>photos:<strong>61</strong> </h3>
    <h4>tags: <strong><a href="/en/tag/Cumshot">Cumshot</a>, <a href="/en/tag/Footjob">Footjob</a></strong></h4>
</div>`

const tokyoBlock = `
<div class="girl box">
    <div class="player" style="background:url(https://cdn.tokyofacefuck.com/preview/3382bb60/sample.jpg) 0 0 no-repeat;">
        <video><source type="video/mp4" src="https://cdn.tokyofacefuck.com/preview/3382bb60/sample.mp4"></video>
    </div>
    <div class="info"><h1>Mai Miori</h1>
        <div class="infotxt thinbox textured"><p>Sexy Mai Miori gets her face fucked.</p></div>
    </div>
</div>`

const handjobBlock = `
<div class="item-title">
    <div class="item-ltitle"><h1>Rei Hoshino, Misa<span class="h2">handjob preview</span></h1></div>
    <div class="item-rtitle"><h3>Scene Length <strong>13:36</strong></h3><h3>Scene Photos <strong>68</strong></h3></div>
</div>
<div class="player" style="background:url(https://cdn.handjobjapan.com/preview/2666b4q9/sample.jpg) 0 0 no-repeat;">
    <video><source type="video/mp4" src="https://cdn.handjobjapan.com/preview/2666b4q9/sample.mp4"></video>
</div>`

const spermBlock = `
<div class="sample-title central"><a href="actress/Aya-Komatsu">Aya Komatsu</a> & <a href="actress/Nagi-Tsukino">Nagi Tsukino</a> Masturbates Her Sloppy Cum Filled Pussy</div>
<div class="player" style="background:url(https://cdn.spermmania.com/preview/319_KomatsuAya_TsukinoNagi_SH35/scene-lg.jpg) 0 0 no-repeat;">
    <video><source type="video/mp4" src="https://cdn.spermmania.com/preview/319_KomatsuAya_TsukinoNagi_SH35/sample.mp4"></video>
</div>
<div class="sample-info central">
    <div class="info-item">Runtime <strong>21:17</strong></div>
    <div class="info-item">Photos <strong>93</strong></div>
    <div class="info-item">Type <strong><a href="type/Massive-Creampie">Massive Creampie</a></strong></div>
</div>`

const transexBlock = `
<div class="sample-info">
    <h1>Shemale Domination</h1>
    featuring <a href="/model/Mari-Ayanami,-Sora-Kamiki"><strong>Mari Ayanami, Sora Kamiki</strong></a> in <strong>123</strong> photos
</div>
<div class="player" style="background:url(https://cdn.transexjapan.com/tour/0435mjka/wide-3.jpg) 0 0 no-repeat;">
    <video><source type="video/mp4" src="https://cdn.transexjapan.com/tour/0435mjka/sample.mp4"></video>
</div>`

const uraBlock = `
<div class="player" style="background:url(https://cdn.uralesbian.com/tour/28/tour-lg.jpg) 0 0 no-repeat;">
    <video><source type="video/mp4" src="https://cdn.uralesbian.com/tour/28/sample.mp4"></video>
</div>
<h1><a href="model/Nao-Takashima">Nao Takashima</a> & <a href="model/Mari-Hirose">Mari Hirose</a></h1>
<div class="tour-datum"><strong>4</strong> HD Scenes<br />Runtime <strong>1H16</strong></div>
<div class="tour-datum"><strong>913</strong> Photos</div>`

func cfgFor(id string) SiteConfig {
	for _, c := range Configs() {
		if c.SiteID == id {
			return c
		}
	}
	panic("no config " + id)
}

// ---- direct parser tests ----

func TestParseFellatio(t *testing.T) {
	scenes := parseFellatio(cfgFor("fellatiojapan"), fellatioBlock, "u", now)
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes", len(scenes))
	}
	s := scenes[0]
	if s.ID != "644_Rio_3F4G" {
		t.Errorf("ID=%q", s.ID)
	}
	if s.Title != "Rio" || len(s.Performers) != 1 || s.Performers[0] != "Rio" {
		t.Errorf("title/perf = %q %v", s.Title, s.Performers)
	}
	if s.Duration != 742 {
		t.Errorf("Duration=%d want 742", s.Duration)
	}
	if s.Thumbnail != "https://cdn.fellatiojapan.com/preview/644_Rio_3F4G/scene-lg.jpg" {
		t.Errorf("Thumbnail=%q", s.Thumbnail)
	}
	if len(s.Tags) != 2 || s.Tags[0] != "mouth cum" {
		t.Errorf("Tags=%v", s.Tags)
	}
}

func TestParseCospuri(t *testing.T) {
	s := parseCospuri(cfgFor("cospuri"), cospuriBlock, "u", now)[0]
	if s.ID != "0548cpar" {
		t.Errorf("ID=%q", s.ID)
	}
	if s.Title != "Emiri Momota - cosplay" {
		t.Errorf("Title=%q", s.Title)
	}
	if s.Performers[0] != "Emiri Momota" {
		t.Errorf("Perf=%v", s.Performers)
	}
	if s.Duration != 1620 {
		t.Errorf("Duration=%d want 1620", s.Duration)
	}
	if s.URL != "https://cospuri.com/sample?id=0548cpar" {
		t.Errorf("URL=%q", s.URL)
	}
	if s.Series != "cosplay" {
		t.Errorf("Series=%q", s.Series)
	}
}

func TestParseCuteButtsWithDetail(t *testing.T) {
	s := parseCuteButts(cfgFor("cutebutts"), cuteButtsBlock, "u", now)[0]
	if s.ID != "0127hsrh" {
		t.Errorf("ID=%q", s.ID)
	}
	if s.Title != "Teacher's Pet" {
		t.Errorf("Title=%q", s.Title)
	}
	if len(s.Performers) != 2 || s.Performers[1] != "Ria Kurumi" {
		t.Errorf("Perf=%v", s.Performers)
	}
	want := time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)
	if !s.Date.Equal(want) {
		t.Errorf("Date=%v want %v", s.Date, want)
	}
	cuteButtsDetail(&s, cuteButtsDetailHTML)
	if s.Duration != 31*60 {
		t.Errorf("Duration=%d want 1860", s.Duration)
	}
}

func TestParseCumBuffetWithDetail(t *testing.T) {
	s := parseCumBuffet(cfgFor("cumbuffet"), cumBuffetBlock, "u", now)[0]
	if s.ID != "0150zrt7" {
		t.Errorf("ID=%q", s.ID)
	}
	if s.Title != "Dinner Time For Dalila Dark" {
		t.Errorf("Title=%q", s.Title)
	}
	if s.Performers[0] != "Dalila Dark" {
		t.Errorf("Perf=%v", s.Performers)
	}
	want := time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC)
	if !s.Date.Equal(want) {
		t.Errorf("Date=%v", s.Date)
	}
	cumBuffetDetail(&s, cumBuffetDetailHTML)
	if len(s.Tags) != 2 || s.Tags[0] != "Petite" {
		t.Errorf("Tags=%v", s.Tags)
	}
}

func TestParseRemainingSites(t *testing.T) {
	cases := []struct {
		id       string
		block    string
		wantID   string
		wantPerf string
		wantDur  int
	}{
		{"legsjapan", legsJapanBlock, "1170mmom", "Ria Kurumi", 666},
		{"tokyofacefuck", tokyoBlock, "3382bb60", "Mai Miori", 0},
		{"handjobjapan", handjobBlock, "2666b4q9", "Rei Hoshino", 816},
		{"spermmania", spermBlock, "319_KomatsuAya_TsukinoNagi_SH35", "Aya Komatsu", 1277},
		{"transexjapan", transexBlock, "0435mjka", "Mari Ayanami", 0},
		{"uralesbian", uraBlock, "28", "Nao Takashima", 4560},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			cfg := cfgFor(c.id)
			scenes := cfg.parse(cfg, c.block, "u", now)
			if len(scenes) != 1 {
				t.Fatalf("got %d scenes", len(scenes))
			}
			s := scenes[0]
			if s.ID != c.wantID {
				t.Errorf("ID=%q want %q", s.ID, c.wantID)
			}
			if len(s.Performers) == 0 || s.Performers[0] != c.wantPerf {
				t.Errorf("Perf=%v want %q", s.Performers, c.wantPerf)
			}
			if s.Duration != c.wantDur {
				t.Errorf("Duration=%d want %d", s.Duration, c.wantDur)
			}
			if s.Thumbnail == "" {
				t.Errorf("empty thumbnail")
			}
		})
	}
}

// ---- end-to-end via httptest (pagination + detail enrichment) ----

func TestListScenesPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			_, _ = fmt.Fprint(w, "<html>"+fellatioBlock+strings.ReplaceAll(fellatioBlock, "644_Rio_3F4G", "645_Kana_9XQ2")+"</html>")
			return
		}
		_, _ = fmt.Fprint(w, "<html>empty</html>")
	}))
	defer ts.Close()

	cfg := cfgFor("fellatiojapan")
	cfg.Base = ts.URL
	s := New(cfg)
	s.Client = ts.Client()

	ch, _ := s.ListScenes(context.Background(), ts.URL+cfg.ListPath, scraper.ListOpts{})
	got := map[string]bool{}
	for r := range ch {
		if r.Err != nil {
			t.Fatalf("err: %v", r.Err)
		}
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = true
		}
	}
	if len(got) != 2 || !got["644_Rio_3F4G"] || !got["645_Kana_9XQ2"] {
		t.Fatalf("got %v", got)
	}
}

func TestListScenesDetailEnrichment(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/sample/"):
			_, _ = fmt.Fprint(w, cumBuffetDetailHTML)
		case r.URL.Query().Get("page") == "1":
			_, _ = fmt.Fprint(w, "<html>"+cumBuffetBlock+"</html>")
		default:
			_, _ = fmt.Fprint(w, "<html>empty</html>")
		}
	}))
	defer ts.Close()

	cfg := cfgFor("cumbuffet")
	cfg.Base = ts.URL
	s := New(cfg)
	s.Client = ts.Client()

	ch, _ := s.ListScenes(context.Background(), ts.URL+cfg.ListPath, scraper.ListOpts{})
	var tags []string
	count := 0
	for r := range ch {
		if r.Kind == scraper.KindScene {
			count++
			tags = r.Scene.Tags
		}
	}
	if count != 1 {
		t.Fatalf("count=%d", count)
	}
	if len(tags) != 2 || tags[0] != "Petite" {
		t.Errorf("enriched tags=%v", tags)
	}
}

// ---- config + helpers ----

func TestConfigs(t *testing.T) {
	cfgs := Configs()
	if len(cfgs) != 10 {
		t.Fatalf("got %d configs want 10", len(cfgs))
	}
	seen := map[string]bool{}
	for _, c := range cfgs {
		if seen[c.SiteID] {
			t.Errorf("dup id %s", c.SiteID)
		}
		seen[c.SiteID] = true
		if c.parse == nil {
			t.Errorf("%s: nil parse", c.SiteID)
		}
		s := New(c)
		if !s.MatchesURL(c.Base) {
			t.Errorf("%s: MatchesURL(%q)=false", c.SiteID, c.Base)
		}
		if s.MatchesURL("https://example.com/x") {
			t.Errorf("%s: matched foreign URL", c.SiteID)
		}
		if s.ID() != c.SiteID || len(s.Patterns()) == 0 {
			t.Errorf("%s: bad ID/Patterns", c.SiteID)
		}
	}
}

func TestHelpers(t *testing.T) {
	if got := cdnBase("https://cospuri.com"); got != "https://cdn.cospuri.com" {
		t.Errorf("cdnBase=%q", got)
	}
	if got := parseHMRuntime("1H16"); got != 4560 {
		t.Errorf("parseHMRuntime(1H16)=%d", got)
	}
	if got := parseHMRuntime("21:17"); got != 1277 {
		t.Errorf("parseHMRuntime(21:17)=%d", got)
	}
	if got := splitModels("A, B & C"); len(got) != 3 || got[2] != "C" {
		t.Errorf("splitModels=%v", got)
	}
	if got := dedupTrim([]string{"x", "x", " y ", ""}); len(got) != 2 {
		t.Errorf("dedupTrim=%v", got)
	}
	if !parseJPDate("2026・01・23").Equal(time.Date(2026, 1, 23, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("parseJPDate failed")
	}
	if !parseJPDate("garbage").IsZero() {
		t.Errorf("parseJPDate(garbage) not zero")
	}
	if cleanText("<b>hi</b>  there") != "hi there" {
		t.Errorf("cleanText failed")
	}
}
