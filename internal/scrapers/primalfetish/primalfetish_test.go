package primalfetish

import (
	"testing"
	"time"
)

// tabooCfg is one real paysite config used to drive the parser.
var tabooCfg = SiteConfig{
	SiteID:     "primalstaboofamily",
	PaysiteID:  14,
	Slug:       "primals-taboo-relations",
	StudioName: "Primal's Taboo Family Relations",
}

// fixtureCard is a verbatim listing card captured from a real
// /paysites/14/primals-taboo-relations/videos/ page.
const fixtureCard = `
<div class="main__videoElement item-col" data-video="https://cdn.primalfetishnetwork.com/videos/6/6/1/e/5/661e5a6f9a137.mp4">
    <a href="https://primalfetishnetwork.com/video/summer-brook-discovering-her-step-brother-part-two-3211.html" class="main__videoWrapper">
        <span class="image image-ar">
                            <div class="image-wrapp">
                                    <img class=""src="https://cdn.primalfetishnetwork.com/thumbs/6/6/1/e/5/65eb0ba3c8c95-65707c133253b-13tab-66002-2818-summer-brook-sister-js-1080p.mkv/65eb0ba3c8c95-65707c133253b-13tab-66002-2818-summer-brook-sister-js-1080p.mkv-8.jpg" alt="Summer Brook - Discovering her Step-Brother PART TWO " >
                                </div>
        </span>
    </a>
    <a href="https://primalfetishnetwork.com/video/summer-brook-discovering-her-step-brother-part-two-3211.html" class="main__videoTitle">
        Summer Brook - Discovering her Step-Brother PART TWO     </a>
    <div class="main__info">
                    <div class="main__quality">HD</div>
                <div class="main__data">
                            <div class="main__models">
                    <span class="title">Models: &nbsp;</span>
                    <a class="video__listModel" title="Rion King" href="https://primalfetishnetwork.com/models/rion-king-198.html" class="model"><span class="sub-label">Rion King</span></a>, <a class="video__listModel" title="Summer Brooks" href="https://primalfetishnetwork.com/models/summer-brooks-221.html" class="model"><span class="sub-label">Summer Brooks</span></a>                </div>
            <div class="main__timedate">
                <div class="date">
                    Date: 12th Jun 2026                </div>
                <div class="time">
                    Time: 25:04                </div>
            </div>
        </div>
    </div>
</div>

<div class="main__videoElement item-col">
    <a href="#" class="main__videoTitle" style="pointer-events: none; cursor: default;">placeholder</a>
</div>
`

func TestParseListing(t *testing.T) {
	s := New(tabooCfg)
	scenes := s.parseListing([]byte(fixtureCard), "https://primalfetishnetwork.com/paysites/14/primals-taboo-relations/videos/")

	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1 (placeholder card must be skipped)", len(scenes))
	}
	sc := scenes[0]

	if sc.ID != "3211" {
		t.Errorf("ID = %q, want 3211", sc.ID)
	}
	if sc.SiteID != "primalstaboofamily" {
		t.Errorf("SiteID = %q, want primalstaboofamily", sc.SiteID)
	}
	if sc.Studio != "Primal's Taboo Family Relations" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.Title != "Summer Brook - Discovering her Step-Brother PART TWO" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != "https://primalfetishnetwork.com/video/summer-brook-discovering-her-step-brother-part-two-3211.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	want := time.Date(2026, time.June, 12, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(want) {
		t.Errorf("Date = %v, want %v", sc.Date, want)
	}
	if sc.Duration != 25*60+4 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 25*60+4)
	}
	wantPerf := []string{"Rion King", "Summer Brooks"}
	if len(sc.Performers) != len(wantPerf) {
		t.Fatalf("Performers = %v, want %v", sc.Performers, wantPerf)
	}
	for i, p := range wantPerf {
		if sc.Performers[i] != p {
			t.Errorf("Performers[%d] = %q, want %q", i, sc.Performers[i], p)
		}
	}
	if sc.Thumbnail != "https://cdn.primalfetishnetwork.com/thumbs/6/6/1/e/5/65eb0ba3c8c95-65707c133253b-13tab-66002-2818-summer-brook-sister-js-1080p.mkv/65eb0ba3c8c95-65707c133253b-13tab-66002-2818-summer-brook-sister-js-1080p.mkv-8.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.ScrapedAt.IsZero() {
		t.Error("ScrapedAt is zero")
	}
}

func TestPageURL(t *testing.T) {
	s := New(tabooCfg)
	tests := []struct {
		page int
		want string
	}{
		{1, "https://primalfetishnetwork.com/paysites/14/primals-taboo-relations/videos/"},
		{2, "https://primalfetishnetwork.com/paysites/14/primals-taboo-relations/videos/page2.html"},
		{5, "https://primalfetishnetwork.com/paysites/14/primals-taboo-relations/videos/page5.html"},
	}
	for _, tt := range tests {
		if got := s.pageURL(tt.page); got != tt.want {
			t.Errorf("pageURL(%d) = %q, want %q", tt.page, got, tt.want)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	taboo := New(tabooCfg)
	cosplay := New(SiteConfig{SiteID: "primalscosplay", PaysiteID: 32, Slug: "primals-cosplay", StudioName: "Primal's Cosplay"})

	if !taboo.MatchesURL("https://primalfetishnetwork.com/paysites/14/primals-taboo-relations/videos/") {
		t.Error("taboo should match its own id-14 URL")
	}
	if !taboo.MatchesURL("https://primalfetishnetwork.com/paysites/14/primals-taboo-relations/") {
		t.Error("taboo should match the bare slug URL")
	}
	if taboo.MatchesURL("https://primalfetishnetwork.com/paysites/32/primals-cosplay/videos/") {
		t.Error("taboo must NOT match cosplay's id-32 URL")
	}
	if !cosplay.MatchesURL("https://primalfetishnetwork.com/paysites/32/primals-cosplay/videos/") {
		t.Error("cosplay should match its own id-32 URL")
	}
	if cosplay.MatchesURL("https://example.com/paysites/32/primals-cosplay/") {
		t.Error("must not match non-primalfetish hosts")
	}
}
