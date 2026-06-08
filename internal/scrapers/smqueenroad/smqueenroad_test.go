package smqueenroad

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
		{"https://www.smqr.com/", true},
		{"https://smqr.com/Front/ItemList", true},
		{"https://www.smqr.com/Front/ItemDetail/123", true},
		{"https://example.com", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseListingXML(t *testing.T) {
	body := []byte(`myMainXml = $.createXMLDocument('<NewDataSet><Items ID=\"111\" Name=\"Title One\" /><Items ID=\"222\" Name=\"Title &amp;amp; Two\" /><Items ID=\"333\" Name=\"Title Three\" /></NewDataSet>');`)

	items := parseListingXML(body)
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}

	if items[0].id != "111" || items[0].name != "Title One" {
		t.Errorf("item[0] = %+v", items[0])
	}
	if items[1].id != "222" || items[1].name != "Title & Two" {
		t.Errorf("item[1] = %+v", items[1])
	}
}

func TestParseListingXMLAlphanumericIDs(t *testing.T) {
	body := []byte(`<Items ID=\"SQR2209DLV2792\" Name=\"Special Title\" />`)
	items := parseListingXML(body)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].id != "SQR2209DLV2792" {
		t.Errorf("id = %q", items[0].id)
	}
}

const detailHTML = `<div class="ItemDetail">
<h1>Test Scene Title</h1>
<video poster="/userdata/Items/eyecatch-111.jpg" controls preload="none">
<source src="/userdata/Items/sample-111.mp4" type="video/mp4" />
</video>
<dl>
<dt>発売日</dt>
<dd>2026年06月09日</dd>
<dt>品番</dt>
<dd>QRLE-005</dd>
  <dt>カテゴリー</dt>
  <dd>
  <ul class="FloatTouchList">
  <li><a href="/Front/ItemList/123">全身タイツ</a></li>
  <li><a href="/Front/ItemList/456">レズSM</a></li>
  </ul>
  </dd>
  <dt>出演者</dt>
  <dd>
  <ul class="List_Casts">
  <li><a href="/Front/CastDetail/789">
  <img src="/userdata/Casts/icon-789.jpg" alt="">霧里純
  </a></li>
  <li><a href="/Front/CastDetail/790">
  <img src="/userdata/Casts/icon-790.jpg" alt="">Test Performer
  </a></li>
  </ul>
  </dd>
</dl>
</div>`

func TestParseDetailPage(t *testing.T) {
	d := parseDetailPage([]byte(detailHTML))

	if d.date.Format("2006-01-02") != "2026-06-09" {
		t.Errorf("date = %v", d.date)
	}
	if d.code != "QRLE-005" {
		t.Errorf("code = %q", d.code)
	}
	if len(d.categories) != 2 || d.categories[0] != "全身タイツ" || d.categories[1] != "レズSM" {
		t.Errorf("categories = %v", d.categories)
	}
	if len(d.performers) != 2 || d.performers[0] != "霧里純" || d.performers[1] != "Test Performer" {
		t.Errorf("performers = %v", d.performers)
	}
	if d.thumbnail != "/userdata/Items/eyecatch-111.jpg" {
		t.Errorf("thumbnail = %q", d.thumbnail)
	}
	if d.preview != "/userdata/Items/sample-111.mp4" {
		t.Errorf("preview = %q", d.preview)
	}
}

func TestParseDetailPageMissingFields(t *testing.T) {
	body := []byte(`<div class="ItemDetail"><h1>Title</h1><dl></dl></div>`)
	d := parseDetailPage(body)
	if !d.date.IsZero() {
		t.Errorf("expected zero date, got %v", d.date)
	}
	if d.code != "" {
		t.Errorf("expected empty code, got %q", d.code)
	}
	if len(d.categories) != 0 {
		t.Errorf("expected no categories, got %v", d.categories)
	}
	if len(d.performers) != 0 {
		t.Errorf("expected no performers, got %v", d.performers)
	}
}

func newTestServer(items []listingItem) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		switch r.URL.Path {
		case "/Front/ItemList":
			var xml string
			for _, item := range items {
				xml += fmt.Sprintf(`<Items ID=\"%s\" Name=\"%s\" />`, item.id, item.name)
			}
			_, _ = fmt.Fprintf(w, `<script>myMainXml = $.createXMLDocument('<NewDataSet>%s</NewDataSet>');</script>`, xml)
		default:
			_, _ = fmt.Fprint(w, detailHTML)
		}
	}))
}

func TestRun(t *testing.T) {
	items := []listingItem{
		{id: "111", name: "Scene One"},
		{id: "222", name: "Scene Two"},
	}
	ts := newTestServer(items)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{Workers: 1})
	if err != nil {
		t.Fatal(err)
	}

	scenes := testutil.CollectScenes(t, ch)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	for _, sc := range scenes {
		if sc.SiteID != siteID {
			t.Errorf("SiteID = %q", sc.SiteID)
		}
		if sc.Studio != studioName {
			t.Errorf("Studio = %q", sc.Studio)
		}
		if sc.ID != "111" && sc.ID != "222" {
			t.Errorf("unexpected ID = %q", sc.ID)
		}
	}
}

func TestRunKnownIDs(t *testing.T) {
	items := []listingItem{
		{id: "111", name: "Scene One"},
		{id: "222", name: "Scene Two"},
		{id: "333", name: "Scene Three"},
	}
	ts := newTestServer(items)
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL}
	ch, err := s.ListScenes(context.Background(), ts.URL, scraper.ListOpts{
		Workers:  1,
		KnownIDs: map[string]bool{"222": true},
	})
	if err != nil {
		t.Fatal(err)
	}

	scenes, stoppedEarly := testutil.CollectScenesWithStop(t, ch)
	if !stoppedEarly {
		t.Error("expected StoppedEarly signal")
	}
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1", len(scenes))
	}
}
