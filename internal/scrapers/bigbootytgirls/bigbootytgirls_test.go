package bigbootytgirls

import "testing"

// The two sibling domains that redirect here must NOT be claimed by this
// scraper: their catalogues live on the network tour and belong to trans500,
// and scraper.ForURL takes the first registration that matches.
func TestMatchesURL(t *testing.T) {
	s := New()
	for _, u := range []string{
		"https://bigbootytgirls.com",
		"https://bigbootytgirls.com/",
		"https://www.bigbootytgirls.com/categories/updates_2_d.html",
		"http://bigbootytgirls.com/updates/Taking-On-Tatyana.html",
	} {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"https://ikillitts.com/",
		"https://tsgirlfriendexperience.com/",
		"https://trans500.com/",
		"https://bigbootytgirlsfan.com/",
	} {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}

func TestIdentity(t *testing.T) {
	s := New()
	if s.ID() != "bigbootytgirls" {
		t.Errorf("ID() = %q", s.ID())
	}
	if len(s.Patterns()) == 0 {
		t.Error("Patterns() is empty")
	}
}
