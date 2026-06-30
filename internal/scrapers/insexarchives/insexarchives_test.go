package insexarchives

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

func itemHTML(name, file, desc, images, clips, dur string) string {
	return fmt.Sprintf(`
      <table width="760" border="0">
        <tr>
          <td width="33%%"><div align="center"></div></td>
          <td width="33%%"><div align="center" class="articleTextRed">%s</div></td>
          <td width="33%%"><div align="center"></div></td>
        </tr>
        <tr>
          <td colspan="3"><div align="center" class="articleText">
			%s ...<a href="media.php?file=%s/index.php"> (continued)</a></div></td>
        </tr>
        <tr>
          <td><div class="mainTextRed">%s Images</div></td>
          <td><span class="mainTextRed">%s Clips </span></td>
          <td><span class="mainTextRed">%s                Minutes / 76.2 MB </span></td>
        </tr>
        <tr><td colspan="3"><div align="center">
          <a href="media.php?file=%s/index.php"><img src="https://www.insexarchives.com/images/updates/%s/promo.jpg" border="0"></a></div></td></tr>
      </table>`, name, desc, file, images, clips, dur, file, file)
}

func listingHTML() string {
	return "<html><body>" +
		`<table border="0"><tr><td align="center">1 <a href="updates_new.php?start=10">2</a></td></tr></table>` +
		itemHTML("Hot Feet", "VOL0114/26lf5_17", "My second trip to New York was eventful", "250", "10", "46:50") +
		itemHTML("The Substitute", "CELL101/substitute", "A different kind of lesson", "180", "8", "32:10") +
		"</body></html>"
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"http://www.insexarchives.com/updates_new.php?start=0": true,
		"https://insexarchives.com/":                           true,
		"https://example.com/x":                                false,
		"":                                                     false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestParseListing(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	siteBase = "http://www.insexarchives.com"

	scenes := parseListing([]byte(listingHTML()), "studioURL", time.Now().UTC())
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "VOL0114/26lf5_17" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "Hot Feet" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != "http://www.insexarchives.com/media.php?file=VOL0114/26lf5_17/index.php" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Duration != 46*60+50 {
		t.Errorf("Duration = %d, want 2810", sc.Duration)
	}
	if sc.Studio != studioName {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if strings.Contains(sc.Description, "continued") {
		t.Errorf("Description should drop (continued): %q", sc.Description)
	}
	if !strings.Contains(sc.Description, "eventful") {
		t.Errorf("Description = %q", sc.Description)
	}
	if sc.Thumbnail != "http://www.insexarchives.com/images/updates/VOL0114/26lf5_17/promo.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if strings.Join(sc.Tags, ",") != "250 Images,10 Clips" {
		t.Errorf("Tags = %v", sc.Tags)
	}
}

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start") == "0" {
			_, _ = fmt.Fprint(w, listingHTML())
			return
		}
		_, _ = fmt.Fprint(w, "<html><body>no items</body></html>")
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
	if got["VOL0114/26lf5_17"] != "Hot Feet" {
		t.Errorf("scenes = %v", got)
	}
}
