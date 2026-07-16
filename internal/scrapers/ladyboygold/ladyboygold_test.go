package ladyboygold

import "testing"

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url   string
		match bool
	}{
		{"https://ladyboygold.com", true},
		{"https://www.ladyboygold.com/", true},
		{"http://ladyboygold.com/tour/trailer/x/", true},
		{"https://ladyboygold.net/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.match {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.match)
		}
	}
}

func TestID(t *testing.T) {
	if got := New().ID(); got != "ladyboygold" {
		t.Errorf("ID() = %q", got)
	}
}

// The set_list block mixes photo sets in with videos and offers no type field,
// so the content path is the only way to tell them apart. Getting this wrong
// silently files ~1800 photo sets as scenes.
func TestSkipPathRe(t *testing.T) {
	photoPaths := []string{
		"lbg/00lfb/photos/some_set/",
		"lbg/00lfb/4kphotos/snack_miniskirt4k/",
		"lbg/00lfb/photos4k/x/",
		"lbg/00lfb/candid_photos/y/",
	}
	for _, p := range photoPaths {
		if !skipPathRe.MatchString(p) {
			t.Errorf("photo path %q was not skipped", p)
		}
	}

	videoPaths := []string{
		"lbg/00lfb/videos/x/",
		"lbg/00lfb/4kvideos/snack_skintightminiskirt4k/",
		"lbg/00lfb/videos4k/x/",
		"lbg/00lfb/remastered4k/x/",
		"lbg/00lfb/ladyboys/om/",
		"lbg/00lfb/gogoladyboys/x/",
		"lbg/00lfb/globalshemales/x/",
		"lbg/00lfb/hotspots/hot-tuna/",
		"lbg/00lfb/candid_videos/x/",
	}
	for _, p := range videoPaths {
		if skipPathRe.MatchString(p) {
			t.Errorf("video path %q was wrongly skipped", p)
		}
	}
}
