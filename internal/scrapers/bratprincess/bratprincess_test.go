package bratprincess

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

func node(slug, title, poster, body string) string {
	return `<div  about="/content/` + slug + `" typeof="sioc:Item foaf:Document" class="ds-1col node node-videos node-odd published with-comments view-mode-video_list clearfix">
  <div class="field field-name-title field-type-ds field-label-hidden"><div class="field-items"><div class="field-item even" property="dc:title"><h6>` + title + `</h6></div></div></div>
  <div class="field field-name-field-poster field-type-image field-label-hidden"><div class="field-items"><div class="field-item even"><img typeof="foaf:Image" src="` + poster + `" width="3840" height="2160" alt="" /></div></div></div>
  <div class="field field-name-body field-type-text-with-summary field-label-hidden"><div class="field-items"><div class="field-item even" property="content:encoded"><p>` + body + `</p></div></div></div>
</div>`
}

func listingHTML() string {
	return `<html><body><div class="view view-id-video_list"><div class="view-content">` +
		node("grace-pov-first-time-cum-eater", "Grace POV - First Time Cum Eater", "https://www.bratprincess.us/sites/default/files/Grace21.jpg", "Grace wants you to listen carefully.") +
		node("misty-naked-ballbusting", "Misty - Naked Ballbusting MUST SEE", "/sites/default/files/Misty.jpg", "Misty has no mercy.") +
		`</div></div></body></html>`
}

func emptyHTML() string {
	return `<html><body><div class="view view-id-video_list"><div class="view-content"></div></div></body></html>`
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://www.bratprincess.us/video-list?page=0", true},
		{"https://bratprincess.us/", true},
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
	scenes := parseListing(listingHTML(), "studioURL", now)
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "grace-pov-first-time-cum-eater" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.Title != "Grace POV - First Time Cum Eater" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Studio != "Brat Princess" {
		t.Errorf("Studio = %q", sc.Studio)
	}
	if sc.URL != siteBase+"/content/grace-pov-first-time-cum-eater" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Thumbnail != "https://www.bratprincess.us/sites/default/files/Grace21.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Description != "Grace wants you to listen carefully." {
		t.Errorf("Description = %q", sc.Description)
	}
	// Root-relative poster gets absolutized.
	if scenes[1].Thumbnail != siteBase+"/sites/default/files/Misty.jpg" {
		t.Errorf("scene1 Thumbnail = %q", scenes[1].Thumbnail)
	}
}

func TestListScenes(t *testing.T) {
	orig := siteBase
	defer func() { siteBase = orig }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "0" {
			_, _ = fmt.Fprint(w, listingHTML())
			return
		}
		_, _ = fmt.Fprint(w, emptyHTML())
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
	if !strings.Contains(got["misty-naked-ballbusting"], "Ballbusting") {
		t.Errorf("scenes = %v", got)
	}
}
