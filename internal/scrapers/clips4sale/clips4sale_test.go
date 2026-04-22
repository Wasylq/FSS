package clips4sale

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

const testStudioURL = "https://www.clips4sale.com/studio/27897/bettie-bondage"

// pageHTML wraps clips in a minimal window.__remixContext page so the scraper
// can parse it the same way it parses a real C4S page.
func pageHTML(clips []c4sClip) []byte {
	ld := loaderData{Clips: clips, ClipsCount: len(clips), Page: 1}
	ldJSON, _ := json.Marshal(ld)
	ctx := map[string]interface{}{
		"state": map[string]interface{}{
			"loaderData": map[string]json.RawMessage{
				routeKey: ldJSON,
			},
		},
	}
	ctxJSON, _ := json.Marshal(ctx)
	return fmt.Appendf(nil, "<html><body><script>window.__remixContext = %s</script></body></html>", ctxJSON)
}

// ---- MatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.clips4sale.com/studio/27897/bettie-bondage", true},
		{"http://clips4sale.com/studio/123/some-studio", true},
		{"https://www.clips4sale.com/studio/27897/bettie-bondage/", true},
		// clip URLs must not match (second segment is numeric)
		{"https://www.clips4sale.com/studio/27897/31405019/rumor-has-it", false},
		// other sites
		{"https://www.manyvids.com/Profile/590705/bettie-bondage/Store/Videos", false},
		{"", false},
	}
	for _, c := range cases {
		got := s.MatchesURL(c.url)
		if got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- studioParams ----

func TestStudioParams(t *testing.T) {
	cases := []struct {
		url      string
		wantID   string
		wantSlug string
		wantErr  bool
	}{
		{"https://www.clips4sale.com/studio/27897/bettie-bondage", "27897", "bettie-bondage", false},
		{"https://clips4sale.com/studio/999/my-studio", "999", "my-studio", false},
		{"https://www.manyvids.com/Profile/123", "", "", true},
	}
	for _, c := range cases {
		id, slug, err := studioParams(c.url)
		if (err != nil) != c.wantErr {
			t.Errorf("studioParams(%q) error = %v, wantErr %v", c.url, err, c.wantErr)
			continue
		}
		if id != c.wantID {
			t.Errorf("studioParams(%q) id = %q, want %q", c.url, id, c.wantID)
		}
		if slug != c.wantSlug {
			t.Errorf("studioParams(%q) slug = %q, want %q", c.url, slug, c.wantSlug)
		}
	}
}

// ---- parseDate ----

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
	}{
		{"1/24/25 10:22 PM", time.Date(2025, 1, 24, 22, 22, 0, 0, time.UTC)},
		{"6/29/25 2:05 PM", time.Date(2025, 6, 29, 14, 5, 0, 0, time.UTC)},
		{"12/1/24 12:00 AM", time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)},
		{"", time.Time{}},
		{"bad", time.Time{}},
	}
	for _, c := range cases {
		got := parseDate(c.input)
		if !got.Equal(c.want) {
			t.Errorf("parseDate(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// ---- parseScreenSize ----

func TestParseScreenSize(t *testing.T) {
	cases := []struct {
		input  string
		width  int
		height int
	}{
		{"3840x2160", 3840, 2160},
		{"1920x1080", 1920, 1080},
		{"1280x720", 1280, 720},
		{"", 0, 0},
		{"bad", 0, 0},
	}
	for _, c := range cases {
		w, h := parseScreenSize(c.input)
		if w != c.width || h != c.height {
			t.Errorf("parseScreenSize(%q) = %d×%d, want %d×%d", c.input, w, h, c.width, c.height)
		}
	}
}

// ---- stripHTML ----

func TestStripHTML(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"<p>Hello world</p>", "Hello world"},
		{"<p>You weren&#39;t planning on entering this</p>", "You weren't planning on entering this"},
		{"plain text", "plain text"},
		{"", ""},
	}
	for _, c := range cases {
		got := stripHTML(c.input)
		if got != c.want {
			t.Errorf("stripHTML(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ---- extractClips ----

func TestExtractClips(t *testing.T) {
	clips := []c4sClip{
		{ClipID: "111", Title: "Clip One"},
		{ClipID: "222", Title: "Clip Two"},
	}
	got, count, err := extractClips(pageHTML(clips))
	if err != nil {
		t.Fatalf("extractClips: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ClipID != "111" || got[1].ClipID != "222" {
		t.Errorf("IDs = %q %q", got[0].ClipID, got[1].ClipID)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestExtractClipsNoMarker(t *testing.T) {
	_, _, err := extractClips([]byte("<html>no remix here</html>"))
	if err == nil {
		t.Error("expected error for missing remixContext")
	}
}

// ---- toScene (real fixture data) ----

var fixtureClip = c4sClip{
	ClipID:           "30455757",
	Title:            "Stepmom Helps You Win 4K",
	Link:             "/studio/27897/30455757/stepmom-helps-you-win-4k",
	DateDisplay:      "1/24/25 10:22 PM",
	Description:      "<p>You weren&#39;t planning on entering this</p>",
	CDNPreviewLgLink: "https://imagecdn.clips4sale.com/accounts99/27897/clip_images/previewlg_30455757.jpg",
	CustomPreviewURL: "https://imagecdn.clips4sale.com/accounts99/27897/clips_previews/prev_30455757.mp4",
	Performers:       []c4sPerformer{{StageName: "Bettie Bondage"}},
	StudioTitle:      "Bettie Bondage",
	CategoryName:     "taboo",
	RelatedCategoryLinks: []c4sRelatedCat{
		{Category: "Confessions"},
		{Category: "Jerk Off Instruction"},
		{Category: "Milf"},
		{Category: "POV"},
	},
	KeywordLinks: []c4sKeyword{
		{Keyword: "Milf"}, // duplicate — should be deduped
	},
	TimeMinutes:    50,
	ResolutionText: "4k",
	ScreenSize:     "3840x2160",
	Format:         "mp4",
	Price:          32.99,
}

func TestToScene(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	scene, err := toScene(testStudioURL, defaultSiteBase, fixtureClip, now)
	if err != nil {
		t.Fatalf("toScene: %v", err)
	}

	str := func(field, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	num := func(field string, got, want int) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %d, want %d", field, got, want)
		}
	}

	str("ID", scene.ID, "30455757")
	str("SiteID", scene.SiteID, "clips4sale")
	str("StudioURL", scene.StudioURL, testStudioURL)
	str("Title", scene.Title, "Stepmom Helps You Win 4K")
	str("URL", scene.URL, "https://www.clips4sale.com/studio/27897/30455757/stepmom-helps-you-win-4k")
	str("Description", scene.Description, "You weren't planning on entering this")
	str("Thumbnail", scene.Thumbnail, fixtureClip.CDNPreviewLgLink)
	str("Preview", scene.Preview, fixtureClip.CustomPreviewURL)
	str("Studio", scene.Studio, "Bettie Bondage")
	str("Resolution", scene.Resolution, "4K")
	str("Format", scene.Format, "MP4")
	num("Width", scene.Width, 3840)
	num("Height", scene.Height, 2160)
	num("Duration", scene.Duration, 3000) // 50 min × 60

	wantDate := time.Date(2025, 1, 24, 22, 22, 0, 0, time.UTC)
	if !scene.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", scene.Date, wantDate)
	}

	if len(scene.Performers) != 1 || scene.Performers[0] != "Bettie Bondage" {
		t.Errorf("Performers = %v, want [Bettie Bondage]", scene.Performers)
	}

	if len(scene.Categories) != 1 || scene.Categories[0] != "taboo" {
		t.Errorf("Categories = %v, want [taboo]", scene.Categories)
	}

	// 4 from related_category_links; "Milf" from keyword_links is a dupe → 4 unique.
	if len(scene.Tags) != 4 {
		t.Errorf("Tags = %v (len %d), want 4 unique tags", scene.Tags, len(scene.Tags))
	}

	if len(scene.PriceHistory) != 1 {
		t.Fatalf("PriceHistory len = %d, want 1", len(scene.PriceHistory))
	}
	p := scene.PriceHistory[0]
	if p.Regular != 32.99 {
		t.Errorf("Price.Regular = %v, want 32.99", p.Regular)
	}
	if p.IsOnSale {
		t.Error("Price.IsOnSale = true, want false")
	}
	if scene.LowestPrice != 32.99 {
		t.Errorf("LowestPrice = %v, want 32.99", scene.LowestPrice)
	}
}

// ---- fetchPage via httptest ----

func TestFetchPage(t *testing.T) {
	clips := []c4sClip{
		{ClipID: "111", Title: "Clip One", Price: 9.99},
		{ClipID: "222", Title: "Clip Two", Price: 14.99},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(pageHTML(clips))
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL, pageLimit: defaultPageLimit}
	got, _, err := s.fetchPage(context.Background(), "27897", "bettie-bondage", 1)
	if err != nil {
		t.Fatalf("fetchPage: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("clips len = %d, want 2", len(got))
	}
	if got[0].ClipID != "111" || got[1].ClipID != "222" {
		t.Errorf("clip IDs = %q %q", got[0].ClipID, got[1].ClipID)
	}
}

// ---- ListScenes end-to-end via httptest ----

func TestListScenes(t *testing.T) {
	page1Clips := []c4sClip{
		{ClipID: "111", Title: "Scene One", Price: 9.99, DateDisplay: "1/1/25 12:00 PM", TimeMinutes: 10},
		{ClipID: "222", Title: "Scene Two", Price: 14.99, DateDisplay: "2/1/25 12:00 PM", TimeMinutes: 20},
	}
	page2Clips := []c4sClip{
		{ClipID: "333", Title: "Scene Three", Price: 19.99, DateDisplay: "3/1/25 12:00 PM", TimeMinutes: 30},
	}

	page := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		switch page {
		case 1:
			w.Write(pageHTML(page1Clips))
		case 2:
			w.Write(pageHTML(page2Clips))
		default:
			w.Write(pageHTML(nil))
		}
	}))
	defer ts.Close()

	// pageLimit:2 so page1 (2 clips) triggers fetching page2 (1 clip) then page3 (empty, stops).
	s := &Scraper{client: ts.Client(), siteBase: ts.URL, pageLimit: 2}
	ch, err := s.ListScenes(context.Background(), testStudioURL, scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	got := map[string]string{}
	for result := range ch {
		if result.Total > 0 {
			continue
		}
		if result.Err != nil {
			t.Errorf("unexpected error: %v", result.Err)
			continue
		}
		got[result.Scene.ID] = result.Scene.Title
	}

	if len(got) != 3 {
		t.Fatalf("got %d scenes, want 3: %v", len(got), got)
	}
	want := map[string]string{"111": "Scene One", "222": "Scene Two", "333": "Scene Three"}
	for id, title := range want {
		if got[id] != title {
			t.Errorf("scene %s title = %q, want %q", id, got[id], title)
		}
	}
}

// ---- KnownIDs are NOT skipped ----
//
// C4S uses recommended sort, not date order, so early-stop is impossible.
// All clips are always emitted in site order; scrapeIncremental handles
// price-history carry-over for previously-seen IDs.

func TestListScenesEmitsKnownIDs(t *testing.T) {
	clips := []c4sClip{
		{ClipID: "111", Title: "Scene One", Price: 9.99},
		{ClipID: "222", Title: "Scene Two", Price: 14.99},
		{ClipID: "333", Title: "Scene Three", Price: 19.99},
	}

	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Write(pageHTML(clips))
		} else {
			w.Write(pageHTML(nil))
		}
	}))
	defer ts.Close()

	s := &Scraper{client: ts.Client(), siteBase: ts.URL, pageLimit: defaultPageLimit}
	ch, err := s.ListScenes(context.Background(), testStudioURL, scraper.ListOpts{
		KnownIDs: map[string]bool{"222": true},
	})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	got := map[string]string{}
	for result := range ch {
		if result.Total > 0 {
			continue
		}
		if result.Err != nil {
			t.Errorf("unexpected error: %v", result.Err)
			continue
		}
		got[result.Scene.ID] = result.Scene.Title
	}

	// All 3 scenes must be emitted even though "222" is in KnownIDs.
	if _, ok := got["222"]; !ok {
		t.Error("scene 222 was not emitted; C4S must emit all clips in site order")
	}
	if len(got) != 3 {
		t.Errorf("got %d scenes, want 3: %v", len(got), got)
	}
}
