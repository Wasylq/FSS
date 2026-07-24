package gasm

import (
	"strings"
	"testing"
)

// Error paths in the parsers, which the end-to-end tests in gasm_run_test.go
// drive past rather than into.

func TestIDAndPatterns(t *testing.T) {
	t.Parallel()
	s := New()
	if s.ID() != "gasm" {
		t.Errorf("ID() = %q, want gasm", s.ID())
	}
	pats := s.Patterns()
	if len(pats) == 0 {
		t.Fatal("Patterns() is empty")
	}
	// Every domain the scraper claims to handle must actually resolve, or
	// `fss list-scrapers` advertises a URL that MatchesURL rejects.
	for _, p := range pats {
		if strings.Contains(p, "{") {
			continue // templated profile pattern, not a bare domain
		}
		if !s.MatchesURL("https://" + p + "/") {
			t.Errorf("Patterns() advertises %q but MatchesURL rejects it", p)
		}
	}
}

func TestParseProfileInvalidJSON(t *testing.T) {
	t.Parallel()
	body := []byte(`<div data-ajax-params="{&quot;user_ids&quot;:[42}"></div>`)
	_, _, err := parseProfile(body, "broken")
	if err == nil {
		t.Fatal("expected an error for malformed ajax params JSON")
	}
	if !strings.Contains(err.Error(), "parsing ajax params") {
		t.Errorf("error = %v", err)
	}
}

func TestParseProfileNoUserIDs(t *testing.T) {
	t.Parallel()
	body := []byte(`<div data-ajax-params="{&quot;user_ids&quot;:[]}"></div>`)
	_, _, err := parseProfile(body, "empty")
	if err == nil {
		t.Fatal("expected an error when user_ids is empty")
	}
	if !strings.Contains(err.Error(), "no user_ids") {
		t.Errorf("error = %v", err)
	}
}

// A card without a data-post-id is skipped rather than aborting the page.
func TestParseListingSkipsCardWithoutPostID(t *testing.T) {
	t.Parallel()
	body := []byte(`<div class="_results _results_posts">
<div class="_results_item _results_posts_item">
<div class="post_item video">
<a class="post_title" href="/post/details/x" title="No ID">No ID</a>
</div></div></div>
<div class="_results_item _results_posts_item">
<div class="post_item video" data-post-id="500">
<a class="preview" href="/post/details/500" data-media-poster="https://cdn.example.com/t500.jpeg"></a>
<span class="counter"><b>03:00</b></span>
<a class="post_title" href="/post/details/500" title="Real">Real</a>
</div></div></div>`)

	items, _, err := parseListing(body)
	if err != nil {
		t.Fatalf("parseListing: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (the card without a post id is skipped)", len(items))
	}
	if items[0].id != "500" {
		t.Errorf("id = %q, want 500", items[0].id)
	}
}

func TestParseDetailInvalidJSON(t *testing.T) {
	t.Parallel()
	body := []byte(`<span dev-span="{&quot;id&quot;:1,}"></span>`)
	_, err := parseDetail(body, "1", "https://www.gasm.com/studio/profile/x", "x")
	if err == nil {
		t.Fatal("expected an error for malformed dev-span JSON")
	}
	if !strings.Contains(err.Error(), "dev-span JSON") {
		t.Errorf("error = %v", err)
	}
}

func TestExtractHostInvalidURL(t *testing.T) {
	t.Parallel()
	// A control character makes url.Parse fail outright.
	if got := extractHost("http://\x7f/bad"); got != "" {
		t.Errorf("extractHost(malformed) = %q, want empty", got)
	}
}
