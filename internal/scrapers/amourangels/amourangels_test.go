package amourangels

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

// ---- fixtures ----

func listingHTML(ids ...string) string {
	var sb strings.Builder
	sb.WriteString(`<html><body><table>`)
	for _, id := range ids {
		fmt.Fprintf(&sb, `<td><a href="/z_cover_%s.html"><img src="/cm_cvl/set-%s.jpg"></a></td>`, id, id)
	}
	sb.WriteString(`</table></body></html>`)
	return sb.String()
}

func detailHTML(title, photographer, date, dur string) string {
	return fmt.Sprintf(`<html><head><title>Amour Angels - Nude Girls</title></head><body>
<div><img src="/cm_cvl/sunlit-lake-by-wart-video.jpg" width=427 height=641></div>
<TABLE WIDTH=400><TR>
<TD width=100><A class='Help2' href="/z_cover_3865.html"><b><u>&lt;&lt; Previous Set</u></b></A></TD>
<TD width=200 align=center><p style="text-align:center">
<b>%s</b><br>
<A href="/photographer_133.html" class='ulink'>%s</A><br>Added %s<br>%s min. VIDEO<br>
</TD>
<TD width=100><A class='Help' href="/z_cover_3868.html"><b><u>Next Set &gt;&gt;</u></b></A></TD>
</TR></TABLE>
</body></html>`, title, photographer, date, dur)
}

// ---- TestMatchesURL ----

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"http://amourangels.com/", true},
		{"https://www.amourangels.com/videos2.html", true},
		{"https://example.com/x", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

// ---- TestListingURL ----

func TestListingURL(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	siteBase = "http://x"
	s := New()
	if got := s.listingURL(1); got != "http://x/videos2.html" {
		t.Errorf("page1 = %q", got)
	}
	if got := s.listingURL(3); got != "http://x/videos2_3.html" {
		t.Errorf("page3 = %q", got)
	}
}

// ---- TestDecodeLatin1 ----

func TestDecodeLatin1(t *testing.T) {
	// 0xE9 is é in ISO-8859-1.
	got := string(decodeLatin1([]byte{'R', 'e', 'n', 0xE9}))
	if got != "René" {
		t.Errorf("decodeLatin1 = %q, want René", got)
	}
}

// ---- TestFetchListing ----

func TestFetchListing(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, listingHTML("3774", "3845", "3774"))
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	ids, err := s.fetchListing(context.Background(), 1)
	if err != nil {
		t.Fatalf("fetchListing error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "3774" || ids[1] != "3845" {
		t.Fatalf("ids = %v, want [3774 3845] (dup dropped)", ids)
	}
}

// ---- TestToScene ----

func TestToScene(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, detailHTML("SUNLIT LAKE VIDEO", "BY WART", "2026-06-13", "14:00"))
	}))
	defer ts.Close()
	siteBase = ts.URL

	s := &Scraper{Client: ts.Client()}
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	sc := s.toScene(context.Background(), "studioURL", "3867", now)

	if sc.ID != "3867" || sc.SiteID != "amourangels" {
		t.Errorf("identity = %q/%q", sc.ID, sc.SiteID)
	}
	if sc.Title != "SUNLIT LAKE VIDEO" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.URL != siteBase+"/z_cover_3867.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	wantDate := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	if !sc.Date.Equal(wantDate) {
		t.Errorf("Date = %v, want %v", sc.Date, wantDate)
	}
	if sc.Duration != 14*60 {
		t.Errorf("Duration = %d, want 840", sc.Duration)
	}
	if sc.Thumbnail != siteBase+"/cm_cvl/sunlit-lake-by-wart-video.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Director != "WART" {
		t.Errorf("Director = %q, want WART", sc.Director)
	}
	if sc.Studio != "Amour Angels" {
		t.Errorf("Studio = %q", sc.Studio)
	}
}

// ---- TestListScenes (end-to-end, listing -> detail -> 404 stop) ----

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos2.html":
			_, _ = fmt.Fprint(w, listingHTML("100", "101"))
		case r.URL.Path == "/videos2_2.html":
			// Past the end: 404 ends pagination cleanly.
			w.WriteHeader(http.StatusNotFound)
		case strings.HasPrefix(r.URL.Path, "/z_cover_100"):
			_, _ = fmt.Fprint(w, detailHTML("First Set", "BY ALEX", "2026-01-02", "10:30"))
		case strings.HasPrefix(r.URL.Path, "/z_cover_101"):
			_, _ = fmt.Fprint(w, detailHTML("Second Set", "BY WART", "2026-02-03", "08:00"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
		if r.Kind == scraper.KindScene {
			got[r.Scene.ID] = r.Scene.Title
		}
	}
	if len(got) != 2 {
		t.Fatalf("got %d scenes, want 2: %v", len(got), got)
	}
	if got["100"] != "First Set" || got["101"] != "Second Set" {
		t.Errorf("scenes = %v", got)
	}
}
