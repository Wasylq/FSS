package czechvr

import (
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

var _ scraper.StudioScraper = (*siteScraper)(nil)

func TestFindPostBlocks(t *testing.T) {
	t.Run("splits blocks correctly", func(t *testing.T) {
		page := `<a name="post1"></a>block one content<a name="post2"></a>block two content<a name="post3"></a>block three content`
		blocks := findPostBlocks(page)
		if len(blocks) != 3 {
			t.Fatalf("expected 3 blocks, got %d", len(blocks))
		}
		if blocks[0] != `<a name="post1"></a>block one content` {
			t.Errorf("block[0] = %q", blocks[0])
		}
		if blocks[1] != `<a name="post2"></a>block two content` {
			t.Errorf("block[1] = %q", blocks[1])
		}
		if blocks[2] != `<a name="post3"></a>block three content` {
			t.Errorf("block[2] = %q", blocks[2])
		}
	})

	t.Run("single block extends to end", func(t *testing.T) {
		page := `preamble<a name="post42"></a>the only block`
		blocks := findPostBlocks(page)
		if len(blocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(blocks))
		}
		if blocks[0] != `<a name="post42"></a>the only block` {
			t.Errorf("block[0] = %q", blocks[0])
		}
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		blocks := findPostBlocks("")
		if blocks != nil {
			t.Fatalf("expected nil, got %v", blocks)
		}
	})

	t.Run("no anchors returns nil", func(t *testing.T) {
		blocks := findPostBlocks("<div>no post anchors here</div>")
		if blocks != nil {
			t.Fatalf("expected nil, got %v", blocks)
		}
	})
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{"full month single-digit day", "January 2, 2024", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)},
		{"abbreviated month single-digit day", "Mar 5, 2023", time.Date(2023, 3, 5, 0, 0, 0, 0, time.UTC)},
		{"full month zero-padded day", "December 05, 2022", time.Date(2022, 12, 5, 0, 0, 0, 0, time.UTC)},
		{"abbreviated month zero-padded day", "Sep 09, 2021", time.Date(2021, 9, 9, 0, 0, 0, 0, time.UTC)},
		{"with leading/trailing whitespace", "  June 15, 2020  ", time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDate(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("invalid date returns error", func(t *testing.T) {
		_, err := parseDate("not-a-date")
		if err == nil {
			t.Fatal("expected error for invalid date")
		}
	})

	t.Run("empty string returns error", func(t *testing.T) {
		_, err := parseDate("")
		if err == nil {
			t.Fatal("expected error for empty string")
		}
	})
}

func TestStripTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"removes simple tags", "<b>bold</b> text", "bold text"},
		{"removes nested tags", "<div><span>hello</span></div>", "hello"},
		{"removes self-closing tags", "before<br/>after", "beforeafter"},
		{"removes tags with attributes", `<a href="url">link</a>`, "link"},
		{"no tags", "plain text", "plain text"},
		{"empty input", "", ""},
		{"trims whitespace", "  <b>text</b>  ", "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripTags(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveURL(t *testing.T) {
	s := newSiteScraper(sites[0]) // czechvrnetwork.com
	base := "https://www.czechvrnetwork.com"

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"dot-slash relative", "./images/thumb.jpg", base + "/images/thumb.jpg"},
		{"absolute path", "/detail-123-scene", base + "/detail-123-scene"},
		{"bare path", "detail-456-scene", base + "/detail-456-scene"},
		{"full http URL", "http://cdn.example.com/img.jpg", "http://cdn.example.com/img.jpg"},
		{"full https URL", "https://cdn.example.com/img.jpg", "https://cdn.example.com/img.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.resolveURL(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseListingCards(t *testing.T) {
	s := newSiteScraper(sites[0])
	base := "https://www.czechvrnetwork.com"

	page := `<html>
<a name="post100"></a>
<div class="card">
  <a href="./detail-42-my-scene-title">
  <h2><a href="./detail-42-my-scene-title">My Scene Title</a></h2>
  <div class="featuring"><a href="./model-alice">Alice</a>, <a href="./model-bob">Bob</a></div>
  <span class="datum">March 15, 2024</span><
  <span class="cas"><span class="icon">30:45</span>
  <img data-src="./thumbs/42.jpg" />
  <video><source src="https://preview.cdn.com/42.mp4"></video>
</div>
<a name="post200"></a>
<div class="card">
  <a href="./detail-99-another-scene">
  <h2><a href="./detail-99-another-scene">Another Scene</a></h2>
  <div class="featuring"><a href="./model-carol">Carol</a></div>
  <span class="datum">Apr 1, 2025</span><
  <span class="cas"><span class="time">15:30</span>
  <img data-src="/thumbs/99.jpg" />
</div>
</html>`

	items := s.parseListingCards(page)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	first := items[0]
	if first.id != "42" {
		t.Errorf("id = %q, want %q", first.id, "42")
	}
	if first.url != base+"/detail-42-my-scene-title" {
		t.Errorf("url = %q", first.url)
	}
	if first.title != "My Scene Title" {
		t.Errorf("title = %q, want %q", first.title, "My Scene Title")
	}
	if len(first.performers) != 2 || first.performers[0] != "Alice" || first.performers[1] != "Bob" {
		t.Errorf("performers = %v, want [Alice Bob]", first.performers)
	}
	wantDate := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if !first.date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", first.date, wantDate)
	}
	if first.duration != 30*60+45 {
		t.Errorf("duration = %d, want %d", first.duration, 30*60+45)
	}
	if first.thumb != base+"/thumbs/42.jpg" {
		t.Errorf("thumb = %q", first.thumb)
	}
	if first.preview != "https://preview.cdn.com/42.mp4" {
		t.Errorf("preview = %q", first.preview)
	}

	second := items[1]
	if second.id != "99" {
		t.Errorf("second id = %q, want %q", second.id, "99")
	}
	if second.title != "Another Scene" {
		t.Errorf("second title = %q", second.title)
	}
	if len(second.performers) != 1 || second.performers[0] != "Carol" {
		t.Errorf("second performers = %v", second.performers)
	}
	if second.thumb != base+"/thumbs/99.jpg" {
		t.Errorf("second thumb = %q", second.thumb)
	}
	if second.preview != "" {
		t.Errorf("second preview = %q, want empty", second.preview)
	}
}

func TestParseListingCards_deduplication(t *testing.T) {
	s := newSiteScraper(sites[0])

	page := `
<a name="post1"></a>
<a href="./detail-42-some-scene"><h2><a href="./detail-42-some-scene">First</a></h2>
<a name="post2"></a>
<a href="./detail-42-some-scene"><h2><a href="./detail-42-some-scene">Duplicate</a></h2>
<a name="post3"></a>
<a href="./detail-99-other-scene"><h2><a href="./detail-99-other-scene">Other</a></h2>`

	items := s.parseListingCards(page)
	if len(items) != 2 {
		t.Fatalf("expected 2 items after dedup, got %d", len(items))
	}
	if items[0].id != "42" {
		t.Errorf("first id = %q, want %q", items[0].id, "42")
	}
	if items[0].title != "First" {
		t.Errorf("first title = %q, want %q", items[0].title, "First")
	}
	if items[1].id != "99" {
		t.Errorf("second id = %q, want %q", items[1].id, "99")
	}
}

func TestMatchesURL(t *testing.T) {
	tests := []struct {
		name   string
		siteID string
		url    string
		want   bool
	}{
		{"czechvrnetwork bare domain", "czechvrnetwork", "https://czechvrnetwork.com", true},
		{"czechvrnetwork www", "czechvrnetwork", "https://www.czechvrnetwork.com/vr-porn-videos", true},
		{"czechvr", "czechvr", "https://www.czechvr.com/detail-100-scene", true},
		{"czechvrfetish", "czechvrfetish", "https://czechvrfetish.com/model-alice", true},
		{"czechvrcasting", "czechvrcasting", "https://www.czechvrcasting.com", true},
		{"vrintimacy", "vrintimacy", "https://vrintimacy.com/tag-something", true},
		{"czechar", "czechar", "http://www.czechar.com/vr-porn-videos", true},
		{"wrong domain", "czechvr", "https://www.example.com", false},
		{"partial domain mismatch", "czechvr", "https://www.czechvrfetish.com", false},
	}

	scraperMap := map[string]*siteScraper{}
	for _, cfg := range sites {
		scraperMap[cfg.SiteID] = newSiteScraper(cfg)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, ok := scraperMap[tt.siteID]
			if !ok {
				t.Fatalf("no scraper for site %q", tt.siteID)
			}
			got := s.MatchesURL(tt.url)
			if got != tt.want {
				t.Errorf("MatchesURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
