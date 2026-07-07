package underwatershowutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func testCfg(base string) SiteConfig {
	return SiteConfig{
		ID:       "underwatershow",
		Studio:   "Underwater Show",
		SiteBase: base,
		LoadPath: "load_pics.php",
		Patterns: []string{"underwatershow.com"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?underwatershow\.com`),
	}
}

// uwsFigure is the Underwater Show layout: a "Read more" <a> toggle followed by
// a hidden_text <span> continuation that we want to keep.
const uwsFigure = `<figure id="onefig_water_shmandora_jpg" class="one-picture">
	<img src="imgs/water_shmandora.jpg" alt="">
<figcaption class="picture-descr"><p class="girlname">Nora Shmandora</p><p class="storytext">Hi, my name is Nora. I always wanted a <a id="readmorelink_x" class="readmore_link" href="#">Read more &gt;&gt;&gt;</a> <span id="hidden_x" class="hidden_text">huge toy underwater!</span></p> <a href="join.php" class="button">Join now</a></figcaption></figure>`

// acFigure is the Anal-Coach layout: a multi-source <img> (data-bigimage first)
// and a storytext that is only a "View more" toggle (no real description).
const acFigure = `<figure id="onefig_yunanoi1" class="one-picture">
<span class="expand_link" id="yunanoi1"><img src="imgs900/yuna_noir1.jpg" alt="Yuna Noir" data-bigimage="imgs1200/yuna_noir1.jpg">
<figcaption class="picture-descr">
<p class="girlname">Yuna Noir</p>
<p class="storytext"> <span id="readmorelink_yunanoi1" class="readmore_link viewmore_link">View more &gt;&gt;&gt;</span></p></p></figcaption>
</span>
</figure>`

const plainFigure = `<figure id="onefig_plain" class="one-picture">
	<img src="imgs/jane_doe.jpg" alt="">
<figcaption class="picture-descr"><p class="girlname">Jane Doe</p></figcaption></figure>`

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New(testCfg("https://underwatershow.com"))
	if !s.MatchesURL("https://www.underwatershow.com/") {
		t.Error("should match www host")
	}
	if s.MatchesURL("https://example.com/") {
		t.Error("should not match foreign host")
	}
}

// ---- TestStem ----

func TestStem(t *testing.T) {
	cases := map[string]string{
		"imgs/water_shmandora.jpg": "water_shmandora",
		"imgs900/yuna_noir1.jpg":   "yuna_noir1",
		"foo":                      "foo",
		"":                         "",
		"a/b/c.d.jpg":              "c.d",
	}
	for in, want := range cases {
		if got := stem(in); got != want {
			t.Errorf("stem(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---- TestToScene ----

func TestToScene(t *testing.T) {
	s := New(testCfg("https://underwatershow.com"))
	now := time.Now().UTC()

	// UWS: keeps hidden continuation, drops the "Read more" toggle.
	sc, ok := s.toScene("studioURL", uwsFigure, now)
	if !ok {
		t.Fatal("uws figure not parsed")
	}
	if sc.ID != "water_shmandora" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "Nora Shmandora" {
		t.Errorf("Title = %q", sc.Title)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Nora Shmandora" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Thumbnail != "https://underwatershow.com/imgs/water_shmandora.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.URL != "https://underwatershow.com" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Studio != "Underwater Show" || sc.SiteID != "underwatershow" {
		t.Errorf("Studio/SiteID = %q/%q", sc.Studio, sc.SiteID)
	}
	wantDesc := "Hi, my name is Nora. I always wanted a huge toy underwater!"
	if sc.Description != wantDesc {
		t.Errorf("Description = %q, want %q", sc.Description, wantDesc)
	}

	// AC: data-bigimage must not win over src; toggle-only storytext -> empty.
	ac, ok := s.toScene("studioURL", acFigure, now)
	if !ok {
		t.Fatal("ac figure not parsed")
	}
	if ac.ID != "yuna_noir1" {
		t.Errorf("ID = %q (data-bigimage leaked?)", ac.ID)
	}
	if ac.Title != "Yuna Noir" {
		t.Errorf("Title = %q", ac.Title)
	}
	if ac.Description != "" {
		t.Errorf("Description = %q, want empty", ac.Description)
	}
}

// ---- TestListScenes (paginated end-to-end) ----

func TestListScenes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("skip") {
		case "0":
			_, _ = fmt.Fprint(w, uwsFigure+"\n"+acFigure)
		case "6":
			_, _ = fmt.Fprint(w, plainFigure)
		default:
			_, _ = fmt.Fprint(w, "  \n")
		}
	}))
	defer ts.Close()

	s := New(testCfg(ts.URL))
	ch, err := s.ListScenes(context.Background(), "studioURL", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
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
	if len(got) != 3 {
		t.Fatalf("got %d scenes across pages, want 3: %v", len(got), got)
	}
	if got["water_shmandora"] != "Nora Shmandora" || got["yuna_noir1"] != "Yuna Noir" || got["jane_doe"] != "Jane Doe" {
		t.Errorf("scenes = %v", got)
	}
}
