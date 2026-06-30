package englishmansion

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

// block mirrors the real recent-movies AJAX markup: an outer
// update-block-recent table with title rows, then an inner table holding the
// featuring line, cover image and synopsis.
func block(length, title, featuring, slug, synopsis string) string {
	return `<table width="860" class="update-block-recent">
	<tr>
		<td class="block-title-1" valign="top">Length: ` + length + `</td>
		<td class="block-title-2" valign="top">` + title + `</td>
		<td class="block-title-3" valign="top"></td>
	</tr>
	<tr><td colspan="3" class="block-body">
		<table>
			<tr><td colspan="2"><p class="featuring">Featuring ` + featuring + `</p></td></tr>
			<tr><td><p class="cover"><img src='/still/730/x/` + slug + `/poster2/` + slug + `_blur.jpg' /></p></td></tr>
			<tr><td colspan='3'><p class="synopsis" style="margin-top: 10px">` + synopsis + `</p></td></tr>
		</table>
		<table><tr><td class="showing">Showing trailer</td></tr></table>
	</td></tr>
</table>`
}

func pageHTML(blocks ...string) string {
	return "<!-- cache -->\n" + strings.Join(blocks, "\n")
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.theenglishmansion.com/updates.html", true},
		{"https://theenglishmansion.com/", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestParseListing(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	doc := pageHTML(
		block("00:09:06", "Chastity JOI - 2D POV", "Miss Suzanna Maxwell", "Chastity_JOI_POV", "A taste of freedom."),
		block("00:12:30", "Dungeon Duo", "Mistress Ezada Sinn &amp; Mistress Sidonia", "Dungeon_Duo", "Two mistresses."),
	)
	scenes := parseListing(doc, "studioURL", now)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "Chastity_JOI_POV" {
		t.Errorf("ID = %q, want Chastity_JOI_POV", sc.ID)
	}
	if sc.Title != "Chastity JOI - 2D POV" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Studio != "The English Mansion" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.URL != siteBase+"/updates.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != siteBase+"/still/730/x/Chastity_JOI_POV/poster2/Chastity_JOI_POV_blur.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Duration != 9*60+6 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 9*60+6)
	}
	if strings.Join(sc.Performers, ",") != "Miss Suzanna Maxwell" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Description != "A taste of freedom." {
		t.Errorf("Description = %q", sc.Description)
	}
	// Multi-performer split on "&".
	if strings.Join(scenes[1].Performers, ",") != "Mistress Ezada Sinn,Mistress Sidonia" {
		t.Errorf("scene1 Performers = %v", scenes[1].Performers)
	}
}

func TestListScenesDedupTerminates(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	// Page 1 (offset 0) returns two scenes; every later offset clamps and
	// repeats them, so dedup must terminate the loop.
	page := pageHTML(
		block("00:09:06", "Chastity JOI - 2D POV", "Miss Suzanna Maxwell", "Chastity_JOI_POV", "A taste of freedom."),
		block("00:12:30", "Dungeon Duo", "Mistress Sophia", "Dungeon_Duo", "In the dungeon."),
	)
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		_ = off
		_, _ = w.Write([]byte(page))
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
		t.Fatalf("got %d scenes, want 2 (dedup): %v", len(got), got)
	}
	if hits < 2 {
		t.Errorf("expected at least 2 fetches (page1 + repeat to detect dedup), got %d", hits)
	}
}
