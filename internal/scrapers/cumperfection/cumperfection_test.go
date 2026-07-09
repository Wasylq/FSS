package cumperfection

import "testing"

// fixture mirrors the real cumperfection.com listing card structure.
const listingFixture = `
<div class="update_block">
<div class="update_details" data-setid="791">
	<!-- Update Thumbnail -->
	<a title="" href="/join/">
		<img id="set-target-791" class="update_thumb thumbs stdimage" src="/content//contentthumbs/11/22/11122-1x.jpg" data-src0_1x="/content//contentthumbs/11/22/11122-1x.jpg" />
	</a>
	<!-- Title -->
	<a title="" href="/join/">
	Pineapple Shock	</a>
	<br />
	<span class="update_models">
	<a href="https://www.cumperfection.com/models/Pina-Wiley.html">Pina Wiley</a>
	</span>
	<div class="update_counts">
		14&nbsp;minute(s)&nbsp;of video
	</div>
	<div class="cell update_date">
		<!-- Date -->
	June 25, 2026
	</div>
</div>
<div class="update_details" data-setid="790">
	<a title="" href="/join/">
		<img id="set-target-790" class="update_thumb thumbs stdimage" data-src0_1x="/content//contentthumbs/11/20/11120-1x.jpg" />
	</a>
	<!-- Title -->
	<a title="" href="/join/">
	Double Trouble	</a>
	<br />
	<span class="update_models">
	<a href="/models/Jane-Doe.html">Jane Doe</a>
	<a href="/models/Mary-Sue.html">Mary Sue</a>
	</span>
	<div class="update_counts">
		22&nbsp;minute(s)&nbsp;of video
	</div>
	<div class="cell update_date">
		<!-- Date -->
	May 1, 2026
	</div>
</div>
</div>
<div class="pagination">Page 1 of 23</div>
`

func TestParseListing(t *testing.T) {
	scenes := parseListing([]byte(listingFixture), "https://www.cumperfection.com/categories/movies.html")
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}

	s0 := scenes[0]
	if s0.ID != "791" {
		t.Errorf("ID = %q, want 791", s0.ID)
	}
	if s0.Title != "Pineapple Shock" {
		t.Errorf("Title = %q, want Pineapple Shock", s0.Title)
	}
	if len(s0.Performers) != 1 || s0.Performers[0] != "Pina Wiley" {
		t.Errorf("Performers = %v, want [Pina Wiley]", s0.Performers)
	}
	if s0.URL != "https://www.cumperfection.com/models/Pina-Wiley.html" {
		t.Errorf("URL = %q, want featured model page", s0.URL)
	}
	if s0.Duration != 14*60 {
		t.Errorf("Duration = %d, want %d", s0.Duration, 14*60)
	}
	if s0.Date.IsZero() || s0.Date.Format("2006-01-02") != "2026-06-25" {
		t.Errorf("Date = %v, want 2026-06-25", s0.Date)
	}
	if s0.Thumbnail != "https://www.cumperfection.com/content//contentthumbs/11/22/11122-1x.jpg" {
		t.Errorf("Thumbnail = %q", s0.Thumbnail)
	}
	if s0.SiteID != siteID || s0.Studio != studioName {
		t.Errorf("SiteID/Studio = %q/%q", s0.SiteID, s0.Studio)
	}

	s1 := scenes[1]
	if len(s1.Performers) != 2 {
		t.Errorf("expected 2 performers, got %v", s1.Performers)
	}
	if s1.URL != "https://www.cumperfection.com/models/Jane-Doe.html" {
		t.Errorf("URL = %q, want first model page (relative resolved)", s1.URL)
	}
}

func TestEstimateTotal(t *testing.T) {
	if got := estimateTotal([]byte("Page 1 of 23")); got != 23*perPage {
		t.Errorf("estimateTotal = %d, want %d", got, 23*perPage)
	}
	if got := estimateTotal([]byte("no pager")); got != 0 {
		t.Errorf("estimateTotal = %d, want 0", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://www.cumperfection.com/":                       true,
		"https://cumperfection.com/categories/movies.html":     true,
		"https://www.cumperfection.com/models/Pina-Wiley.html": true,
		"https://example.com/cumperfection":                    false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}
