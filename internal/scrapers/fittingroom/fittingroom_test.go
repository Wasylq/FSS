package fittingroom

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

// card markup mirrors the real fitting-room.com listing block.
func cardHTML(base, id, slug, titleAttr, dur, model string) string {
	return fmt.Sprintf(`
<div class="item premium ">
	<a href="%s/video/%s/%s/" title="%s" >
		<div class="img">
			<img class="thumb lazy-load" src="data:image/gif;base64,AAAA"
				data-original="%s/contents/videos_screenshots/0/%s/320x180/1.jpg"
				alt="%s" width="320" height="180"/>
			<span class="is-hd is-4k">4K</span>
		</div>
		<div class="wrap">
			<div class="duration">%s</div>
			<div class="model">
				%s
			</div>
		</div>
		<div class="wrap">
			<div class="added"><em>3 weeks ago</em></div>
		</div>
	</a>
</div>`, base, id, slug, titleAttr, base, id, titleAttr, dur, model)
}

func listingHTML(base string, cards ...string) string {
	return `<div class="list-videos"><div class="margin-fix">` +
		strings.Join(cards, "\n") +
		`</div></div>`
}

func detailHTML(name, uploadDate, dur string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><head>
<script type="application/ld+json">
{
	"@context": "https://schema.org",
	"@type": "VideoObject",
	"name": "%s",
	"description": "desc",
	"thumbnailUrl": "https://www.fitting-room.com/poster.jpg",
	"uploadDate": "%s",
	"duration": "%s"
}
</script>
</head><body></body></html>`, name, uploadDate, dur)
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server

	details := map[string]string{
		"640": detailHTML("Candice Demellza | Romanian Mombshell", "2026-06-01T00:01:00Z", "PT0H16M16S"),
		"641": detailHTML("Elena Vedem | Time To Fuck", "2026-05-25T00:01:00Z", "PT0H12M59S"),
		"628": detailHTML("Ryana | Solo Session", "2026-05-10T00:01:00Z", "PT0H10M00S"),
	}

	mux.HandleFunc("/videos_list.php", func(w http.ResponseWriter, r *http.Request) {
		base := srv.URL
		switch r.URL.Query().Get("from") {
		case "1":
			// Full page (== pageSize) so pagination continues to page 2.
			cards := make([]string, 0, pageSize)
			cards = append(cards,
				cardHTML(base, "640", "candice-demellza-romanian-mombshell", "Candice Demellza | Romanian Mombshell", "16:16", "Candice Demellza"),
				cardHTML(base, "641", "elena-vedem-time-to-fuck", "Elena Vedem | Time To Fuck", "12:59", "Elena Vedem"),
			)
			// pad to pageSize with reused card 640 so Done is not triggered early;
			// duplicates are harmless for the count-based Done check.
			for len(cards) < pageSize {
				cards = append(cards, cardHTML(base, "640", "candice-demellza-romanian-mombshell", "Candice Demellza | Romanian Mombshell", "16:16", "Candice Demellza"))
			}
			_, _ = fmt.Fprint(w, listingHTML(base, cards...))
		case "2":
			// Short page (< pageSize) -> Done.
			_, _ = fmt.Fprint(w, listingHTML(base,
				cardHTML(base, "628", "ryana-solo-session", "Ryana | Solo Session", "10:00", "Ryana"),
			))
		default:
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/video/", func(w http.ResponseWriter, r *http.Request) {
		// /video/{id}/{slug}/
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		body, ok := details[parts[1]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, body)
	})

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestScrapeEndToEnd(t *testing.T) {
	srv := newTestServer(t)

	s := New()
	s.base = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch, err := s.ListScenes(ctx, srv.URL+"/videos_list.php", scraper.ListOpts{})
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}

	scenes := map[string]bool{}
	var first640 bool
	for res := range ch {
		switch res.Kind {
		case scraper.KindScene:
			sc := res.Scene
			scenes[sc.ID] = true
			if sc.ID == "640" {
				first640 = true
				if sc.Title != "Romanian Mombshell" {
					t.Errorf("title = %q, want %q", sc.Title, "Romanian Mombshell")
				}
				if len(sc.Performers) != 1 || sc.Performers[0] != "Candice Demellza" {
					t.Errorf("performers = %v, want [Candice Demellza]", sc.Performers)
				}
				if sc.Duration != 16*60+16 {
					t.Errorf("duration = %d, want %d", sc.Duration, 16*60+16)
				}
				want := time.Date(2026, 6, 1, 0, 1, 0, 0, time.UTC)
				if !sc.Date.Equal(want) {
					t.Errorf("date = %v, want %v", sc.Date, want)
				}
				if sc.SiteID != siteID {
					t.Errorf("siteID = %q, want %q", sc.SiteID, siteID)
				}
				if sc.Studio != studio {
					t.Errorf("studio = %q, want %q", sc.Studio, studio)
				}
				if !strings.HasSuffix(sc.URL, "/video/640/candice-demellza-romanian-mombshell/") {
					t.Errorf("url = %q, missing detail path", sc.URL)
				}
				if !strings.Contains(sc.Thumbnail, "/0/640/320x180/1.jpg") {
					t.Errorf("thumbnail = %q", sc.Thumbnail)
				}
			}
		case scraper.KindError:
			t.Errorf("unexpected error result: %v", res.Err)
		}
	}

	if !first640 {
		t.Fatal("scene 640 not emitted")
	}
	// Page 1 (640, 641) plus page 2 (628) must all appear, proving async path
	// pagination advanced from=1 -> from=2.
	for _, id := range []string{"640", "641", "628"} {
		if !scenes[id] {
			t.Errorf("scene %s missing (pagination did not advance)", id)
		}
	}
}

func TestMatchesURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.fitting-room.com/videos_list.php", true},
		{"https://fitting-room.com/", true},
		{"http://www.fitting-room.com/video/640/slug/", true},
		{"https://www.pornhub.com/", false},
		{"https://fittingroom.example.com/", false},
	}
	s := New()
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestSplitTitle(t *testing.T) {
	cases := []struct {
		in          string
		model, want string
	}{
		{"Candice Demellza | Romanian Mombshell", "Candice Demellza", "Romanian Mombshell"},
		{"Elena Vedem | Time To Fuck", "Elena Vedem", "Time To Fuck"},
		{"No Separator Here", "", "No Separator Here"},
		{"A | B | C", "A", "B | C"},
	}
	for _, c := range cases {
		model, title := splitTitle(c.in)
		if model != c.model || title != c.want {
			t.Errorf("splitTitle(%q) = (%q, %q), want (%q, %q)", c.in, model, title, c.model, c.want)
		}
	}
}
