package ifeelmyself

import (
	"fmt"
	"testing"

	"github.com/Wasylq/FSS/internal/scrapers/testutil"
	"github.com/Wasylq/FSS/scraper"
)

func buildIFMPage(items []struct {
	sceneID, price, artistID, performer, title, duration, date, thumb string
	categories, tags                                                  []string
}) string {
	html := ""
	for _, it := range items {
		catHTML := ""
		if len(it.categories) > 0 {
			catHTML = `<b>Categories:</b><table>`
			for _, c := range it.categories {
				catHTML += fmt.Sprintf(`<td>%s</td>`, c)
			}
			catHTML += `</table>`
		}
		tagHTML := ""
		for _, tg := range it.tags {
			tagHTML += fmt.Sprintf(`<span class="tags-list-item-tag">%s</span>`, tg)
		}
		titleHTML := ""
		if it.title != "" {
			titleHTML = fmt.Sprintf(`&nbsp;in&nbsp;"%s"`, it.title)
		}
		thumbHTML := ""
		if it.thumb != "" {
			thumbHTML = fmt.Sprintf(`<img src='%s'>`, it.thumb)
		}
		durationHTML := ""
		if it.duration != "" {
			durationHTML = fmt.Sprintf(`HD Video, %s min`, it.duration)
		}

		html += fmt.Sprintf(`<div class="ThumbRec">
			<table class="ThumbTab ppss-scene" data-scene-id="%s" data-scene-price="%s">
			<b><a href='javascript:openLink2(0,artist_bio&amp;artist_id=%s')'>%s</a></b>
			%s
			%s
			%s
			%s
			%s
			%s
			</TABLE>
		</div>`, it.sceneID, it.price, it.artistID, it.performer, titleHTML, durationHTML, it.date, thumbHTML, catHTML, tagHTML)
	}
	return html
}

func TestParseListingPage(t *testing.T) {
	html := buildIFMPage([]struct {
		sceneID, price, artistID, performer, title, duration, date, thumb string
		categories, tags                                                  []string
	}{
		{
			sceneID:    "12345",
			price:      "1.95",
			artistID:   "ABC123",
			performer:  "Luna",
			title:      "Morning Light",
			duration:   "9:48",
			date:       "08 May 2026",
			thumb:      "https://bcdn.ifeelmyself.com/img/12345.jpg",
			categories: []string{"Solo", "Outdoor"},
			tags:       []string{"blonde", "natural"},
		},
		{
			sceneID:   "12344",
			price:     "0",
			artistID:  "DEF456",
			performer: "Stella",
			title:     "",
			duration:  "5:30",
			date:      "01 May 2026",
			thumb:     "https://bcdn.ifeelmyself.com/img/12344.jpg",
		},
	})

	scenes := parseListingPage([]byte(html), "https://ifeelmyself.com")
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	s := scenes[0]
	if s.ID != "12345" {
		t.Errorf("ID = %q, want 12345", s.ID)
	}
	if s.Title != "Morning Light" {
		t.Errorf("Title = %q, want %q", s.Title, "Morning Light")
	}
	if len(s.Performers) != 1 || s.Performers[0] != "Luna" {
		t.Errorf("Performers = %v, want [Luna]", s.Performers)
	}
	if s.Duration != 9*60+48 {
		t.Errorf("Duration = %d, want %d", s.Duration, 9*60+48)
	}
	if s.Date.Format("2006-01-02") != "2026-05-08" {
		t.Errorf("Date = %v, want 2026-05-08", s.Date)
	}
	if s.Thumbnail != "https://bcdn.ifeelmyself.com/img/12345.jpg" {
		t.Errorf("Thumbnail = %q", s.Thumbnail)
	}
	if len(s.Categories) != 2 {
		t.Errorf("Categories = %v, want 2 items", s.Categories)
	}
	if len(s.Tags) != 2 {
		t.Errorf("Tags = %v, want 2 items", s.Tags)
	}
	if s.SiteID != "ifeelmyself" {
		t.Errorf("SiteID = %q, want ifeelmyself", s.SiteID)
	}
	if s.Studio != "I Feel Myself" {
		t.Errorf("Studio = %q, want %q", s.Studio, "I Feel Myself")
	}
	if s.LowestPrice != 1.95 {
		t.Errorf("LowestPrice = %f, want 1.95", s.LowestPrice)
	}

	s2 := scenes[1]
	if s2.Title != "Scene #12344" {
		t.Errorf("Title = %q, want %q", s2.Title, "Scene #12344")
	}
	if s2.LowestPrice != 0 {
		t.Errorf("LowestPrice = %f, want 0 (free)", s2.LowestPrice)
	}
}

func TestParseListingPageEmpty(t *testing.T) {
	scenes := parseListingPage([]byte(`<div>no results</div>`), "https://ifeelmyself.com")
	if len(scenes) != 0 {
		t.Fatalf("got %d scenes, want 0", len(scenes))
	}
}

func TestSplitBlocks(t *testing.T) {
	html := `<div class="ThumbRec">block1</div><div class="ThumbRec">block2</div>`
	blocks := splitBlocks([]byte(html))
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
}

func TestKnownIDsEarlyStop(t *testing.T) {
	html := buildIFMPage([]struct {
		sceneID, price, artistID, performer, title, duration, date, thumb string
		categories, tags                                                  []string
	}{
		{sceneID: "100", price: "0", artistID: "A", performer: "X", date: "01 Jan 2021"},
		{sceneID: "99", price: "0", artistID: "B", performer: "Y", date: "31 Dec 2020"},
		{sceneID: "98", price: "0", artistID: "C", performer: "Z", date: "30 Dec 2020"},
	})

	scenes := parseListingPage([]byte(html), "https://ifeelmyself.com")
	known := map[string]bool{"99": true}
	var collected []string
	for _, sc := range scenes {
		if known[sc.ID] {
			break
		}
		collected = append(collected, sc.ID)
	}
	if len(collected) != 1 || collected[0] != "100" {
		t.Errorf("collected = %v, want [100]", collected)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	tests := []struct {
		url  string
		want bool
	}{
		{"https://ifeelmyself.com", true},
		{"https://www.ifeelmyself.com", true},
		{"http://ifeelmyself.com/public/main.php", true},
		{"https://ifeelmyself.com/public/main.php?page=artist_bio&artist_id=ABC", true},
		{"https://beautifulagony.com", false},
		{"https://example.com", false},
	}
	for _, tc := range tests {
		if got := s.MatchesURL(tc.url); got != tc.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestPatterns(t *testing.T) {
	s := New()
	pats := s.Patterns()
	if len(pats) < 1 {
		t.Fatal("no patterns")
	}
}

func TestInterface(t *testing.T) {
	var s scraper.StudioScraper = New()
	_ = s
}

func TestSceneValidation(t *testing.T) {
	html := buildIFMPage([]struct {
		sceneID, price, artistID, performer, title, duration, date, thumb string
		categories, tags                                                  []string
	}{
		{
			sceneID:   "12345",
			price:     "1.95",
			artistID:  "ABC123",
			performer: "Luna",
			title:     "Morning Light",
			duration:  "9:48",
			date:      "08 May 2026",
			thumb:     "https://bcdn.ifeelmyself.com/img/12345.jpg",
		},
	})
	scenes := parseListingPage([]byte(html), "https://ifeelmyself.com")
	if len(scenes) != 1 {
		t.Fatal("expected 1 scene")
	}
	testutil.ValidateScene(t, scenes[0])
}
