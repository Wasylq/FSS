package ftvutil

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
	}{
		{"Jan 15, 2026", time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"May  7, 2026", time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)},
		{"Dec 31, 2020", time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC)},
		{"  Apr  3, 2025  ", time.Date(2025, 4, 3, 0, 0, 0, 0, time.UTC)},
		{"", time.Time{}},
		{"not a date", time.Time{}},
	}
	for _, c := range cases {
		if got := ParseDate(c.input); !got.Equal(c.want) {
			t.Errorf("ParseDate(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestParseListingPage_HrefID(t *testing.T) {
	body := []byte(`
<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2>Alice</h2></div>
<div class="UpdateDate"><h3>Mar 10, 2026</h3></div>
</div>
<div id="SecondColumn" class="cf">
<div class="S2C1 cf">
<div class="VideoTime"><img alt="" /><h3>45 mins</h3></div>
</div>
<div class="S2C2 cf">
<div class="Tags cf">
<img src="updatesCategories/1st.png" title="First Time - New to adult." alt="" />
<img src="updatesCategories/bb.png" title="Busty Girl - Big, natural breasts." alt="" />
</div>
</div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/update/alice-100.html"><img class="ModelPhotoWide" src="https://cdn.test/alice.jpg" alt="" /></a>
</div>
<div class="ModelBio"><div class="Bio"><p>Alice first visit.</p></div></div>
</div>
</div><!-- ModelContainer -->`)

	entries := ParseListingPage(body)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.ID != "100" {
		t.Errorf("id = %q, want 100", e.ID)
	}
	if e.Name != "Alice" {
		t.Errorf("name = %q, want Alice", e.Name)
	}
	wantDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	if !e.Date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", e.Date, wantDate)
	}
	if e.Duration != 2700 {
		t.Errorf("duration = %d, want 2700", e.Duration)
	}
	if len(e.Tags) != 2 || e.Tags[0] != "First Time" || e.Tags[1] != "Busty Girl" {
		t.Errorf("tags = %v", e.Tags)
	}
	if e.Desc != "Alice first visit." {
		t.Errorf("desc = %q", e.Desc)
	}
	if e.Thumb != "https://cdn.test/alice.jpg" {
		t.Errorf("thumb = %q", e.Thumb)
	}
}

func TestParseListingPage_ThumbID(t *testing.T) {
	body := []byte(`
<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2>Beth</h2></div>
<div class="UpdateDate"><h3>Feb 20, 2026</h3></div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/join.html"><img class="ModelPhotoWide" src="https://cdn.test/beth-tour-200.jpg" alt="" /></a>
</div>
<div class="ModelBio"><div class="Bio"><p>Beth returns.</p></div></div>
</div>
</div><!-- ModelContainer -->`)

	entries := ParseListingPage(body)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.ID != "200" {
		t.Errorf("id = %q, want 200 (thumb-based extraction)", e.ID)
	}
	if e.Name != "Beth" {
		t.Errorf("name = %q", e.Name)
	}
	if e.Thumb != "https://cdn.test/beth-tour-200.jpg" {
		t.Errorf("thumb = %q", e.Thumb)
	}
}

func TestParseListingPage_SkipsMissingID(t *testing.T) {
	body := []byte(`
<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2>NoID</h2></div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/join.html"><img class="ModelPhotoWide" src="https://cdn.test/noid.jpg" alt="" /></a>
</div>
</div>
</div><!-- ModelContainer -->`)

	entries := ParseListingPage(body)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for container with no extractable ID, got %d", len(entries))
	}
}

func TestParseListingPage_SkipsMissingName(t *testing.T) {
	body := []byte(`
<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2></h2></div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/update/x-50.html"><img class="ModelPhotoWide" src="https://cdn.test/tour-50.jpg" alt="" /></a>
</div>
</div>
</div><!-- ModelContainer -->`)

	entries := ParseListingPage(body)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for container with empty name, got %d", len(entries))
	}
}

func TestParseListingPage_Empty(t *testing.T) {
	entries := ParseListingPage([]byte(`<html><body></body></html>`))
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty page, got %d", len(entries))
	}
}

func TestParseListingPage_HTMLEntities(t *testing.T) {
	body := []byte(`
<div class="ModelContainer">
<div class="ModelHeader cf">
<div id="FirstColumn" class="cf">
<div class="ModelName"><h2>A&amp;B</h2></div>
<div class="UpdateDate"><h3>Jan 1, 2026</h3></div>
</div>
</div>
<div class="Model">
<div class="ModelPhoto">
<a href="/update/ab-10.html"><img class="ModelPhotoWide" src="https://cdn.test/ab.jpg" alt="" /></a>
</div>
<div class="ModelBio"><div class="Bio"><p>She&#39;s great.</p></div></div>
</div>
</div><!-- ModelContainer -->`)

	entries := ParseListingPage(body)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Name != "A&B" {
		t.Errorf("name = %q, want A&B (html unescaped)", entries[0].Name)
	}
	if entries[0].Desc != "She's great." {
		t.Errorf("desc = %q, want She's great.", entries[0].Desc)
	}
}

func TestParseDetailPage_DataTitle(t *testing.T) {
	body := []byte(`
<a class="jackbox" data-title="<b>Name: </b><span>Carla</span> <b>Age: </b><span>22</span> <b>Figure: </b><span>34C-24-35</span> <b>Release date: </b><span>Jun 15, 2026</span>" href="t.mp4"></a>
<div id="BioHeader"><h2><b>Height:</b> 5'7"</h2></div>
<div class="OneHeader" id="Bio"><p>Carla loves the outdoors.</p></div>
<div id="MagazineContainer"><img id="Magazine" src="https://cdn.test/carla-mag.jpg" /></div>`)

	d := ParseDetailPage(body)
	if d.Name != "Carla" {
		t.Errorf("name = %q", d.Name)
	}
	if d.Age != 22 {
		t.Errorf("age = %d", d.Age)
	}
	if d.Figure != "34C-24-35" {
		t.Errorf("figure = %q", d.Figure)
	}
	if d.Height != `5'7"` {
		t.Errorf("height = %q", d.Height)
	}
	wantDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if !d.Date.Equal(wantDate) {
		t.Errorf("date = %v, want %v", d.Date, wantDate)
	}
	if d.Desc != "Carla loves the outdoors." {
		t.Errorf("desc = %q", d.Desc)
	}
	if d.Thumb != "https://cdn.test/carla-mag.jpg" {
		t.Errorf("thumb = %q", d.Thumb)
	}
}

func TestParseDetailPage_TitleFallback(t *testing.T) {
	body := []byte(`<title>Dana on FTVGirls.com Released Sep 20, 2025!</title>
<div class="OneHeader" id="Bio"><p>Dana is new.</p></div>`)

	d := ParseDetailPage(body)
	if d.Name != "Dana" {
		t.Errorf("name = %q, want Dana (title fallback)", d.Name)
	}
	wantDate := time.Date(2025, 9, 20, 0, 0, 0, 0, time.UTC)
	if !d.Date.Equal(wantDate) {
		t.Errorf("date = %v", d.Date)
	}
	if d.Desc != "Dana is new." {
		t.Errorf("desc = %q", d.Desc)
	}
}

func TestParseDetailPage_FTVMilfsTitleFallback(t *testing.T) {
	body := []byte(`<title>Eva on FTVMilfs.com Released Nov 11, 2025!</title>`)

	d := ParseDetailPage(body)
	if d.Name != "Eva" {
		t.Errorf("name = %q, want Eva", d.Name)
	}
}

func TestParseDetailPage_Empty(t *testing.T) {
	d := ParseDetailPage([]byte(`<html><body></body></html>`))
	if d.Name != "" || d.Desc != "" || d.Age != 0 {
		t.Errorf("expected empty detail, got %+v", d)
	}
}

func TestNew(t *testing.T) {
	cfg := SiteConfig{
		SiteID:    "ftvgirls",
		Domain:    "ftvgirls.com",
		Studio:    "FTV Girls",
		TitleSite: "FTVGirls.com",
	}
	s := New(cfg)
	if s.ID() != "ftvgirls" {
		t.Errorf("ID() = %q, want ftvgirls", s.ID())
	}
	if s.Base != "https://ftvgirls.com" {
		t.Errorf("Base = %q, want https://ftvgirls.com", s.Base)
	}
	if s.Client == nil {
		t.Fatal("Client is nil")
	}
}

func TestMatchesURL_ftvutil(t *testing.T) {
	s := New(SiteConfig{
		SiteID: "ftvgirls",
		Domain: "ftvgirls.com",
	})
	tests := []struct {
		url  string
		want bool
	}{
		{"https://www.ftvgirls.com", true},
		{"https://ftvgirls.com", true},
		{"http://ftvgirls.com", true},
		{"https://ftvgirls.com/updates.html", true},
		{"https://www.ftvgirls.com/update/alice-100.html", true},
		{"https://otherdomain.com", false},
		{"https://ftvmilfs.com", false},
	}
	for _, tc := range tests {
		if got := s.MatchesURL(tc.url); got != tc.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

func TestPatterns_ftvutil(t *testing.T) {
	s := New(SiteConfig{
		SiteID: "ftvgirls",
		Domain: "ftvgirls.com",
	})
	pats := s.Patterns()
	if len(pats) < 1 {
		t.Fatal("no patterns")
	}
	found := false
	for _, p := range pats {
		if p == "ftvgirls.com" {
			found = true
		}
	}
	if !found {
		t.Errorf("Patterns() = %v, expected to contain %q", pats, "ftvgirls.com")
	}
}
