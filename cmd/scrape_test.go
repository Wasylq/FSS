package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestMergeSiteDelays_emptyInputs(t *testing.T) {
	got, err := mergeSiteDelays(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestMergeSiteDelays_configOnly(t *testing.T) {
	got, err := mergeSiteDelays(map[string]int{"manyvids": 100, "pornhub": 2000}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got["manyvids"] != 100 || got["pornhub"] != 2000 {
		t.Errorf("got %v", got)
	}
}

func TestMergeSiteDelays_cliOverlaysConfig(t *testing.T) {
	cfg := map[string]int{"manyvids": 100, "pornhub": 2000}
	got, err := mergeSiteDelays(cfg, []string{"manyvids=500", "brazzers=300"})
	if err != nil {
		t.Fatal(err)
	}
	if got["manyvids"] != 500 {
		t.Errorf("CLI did not override config: %v", got)
	}
	if got["pornhub"] != 2000 {
		t.Errorf("config value preserved when not overridden: %v", got)
	}
	if got["brazzers"] != 300 {
		t.Errorf("CLI-only entry missing: %v", got)
	}
}

func TestMergeSiteDelays_zeroIsValid(t *testing.T) {
	// User explicitly disables delay for a site that the config set non-zero.
	got, err := mergeSiteDelays(map[string]int{"pornhub": 2000}, []string{"pornhub=0"})
	if err != nil {
		t.Fatal(err)
	}
	if got["pornhub"] != 0 {
		t.Errorf("explicit 0 should override, got %d", got["pornhub"])
	}
}

func TestMergeSiteDelays_skipsBlankPairs(t *testing.T) {
	got, err := mergeSiteDelays(nil, []string{"", "  ", "manyvids=50"})
	if err != nil {
		t.Fatalf("blank pairs should be skipped, got error: %v", err)
	}
	if len(got) != 1 || got["manyvids"] != 50 {
		t.Errorf("got %v", got)
	}
}

func TestMergeSiteDelays_rejectsMalformed(t *testing.T) {
	cases := []string{
		"no-equals-here",
		"=missing-name",
		"missing-value=",
		"manyvids=not-a-number",
		"manyvids=-100",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			_, err := mergeSiteDelays(nil, []string{p})
			if err == nil {
				t.Errorf("expected error for %q", p)
			}
		})
	}
}

func TestResolveSiteDelay(t *testing.T) {
	defaultDelay := 500 * time.Millisecond
	perSite := map[string]int{
		"manyvids": 0,    // explicit 0 = no delay even when default is non-zero
		"pornhub":  2000, // explicit override
	}

	cases := []struct {
		siteID string
		want   time.Duration
	}{
		{"manyvids", 0},
		{"pornhub", 2 * time.Second},
		{"brazzers", 500 * time.Millisecond}, // no override → default
		{"unknown", 500 * time.Millisecond},
	}
	for _, c := range cases {
		t.Run(c.siteID, func(t *testing.T) {
			got := resolveSiteDelay(c.siteID, defaultDelay, perSite)
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestResolveSiteDelay_nilMap(t *testing.T) {
	got := resolveSiteDelay("any", 500*time.Millisecond, nil)
	if got != 500*time.Millisecond {
		t.Errorf("nil map should fall back to default, got %v", got)
	}
}

// Sanity check that the error from mergeSiteDelays mentions the malformed pair.
func TestMergeSiteDelays_errorIncludesOffendingPair(t *testing.T) {
	_, err := mergeSiteDelays(nil, []string{"manyvids=oops"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "manyvids=oops") {
		t.Errorf("error should mention the bad pair, got: %v", err)
	}
}
