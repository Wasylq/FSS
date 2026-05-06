package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
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

func snap(date time.Time, price float64) models.PriceSnapshot {
	return models.PriceSnapshot{Date: date, Regular: price}
}

func freeSnap(date time.Time) models.PriceSnapshot {
	return models.PriceSnapshot{Date: date, IsFree: true}
}

func TestCarryOverPriceHistory_mergesExistingAndFresh(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	existing := models.Scene{ID: "1"}
	existing.AddPrice(snap(t0, 9.99))
	existing.AddPrice(snap(t1, 4.99))

	fresh := models.Scene{ID: "1"}
	fresh.AddPrice(snap(t2, 7.99))

	got := carryOverPriceHistory(fresh, existing)
	if len(got.PriceHistory) != 3 {
		t.Fatalf("got %d snapshots, want 3", len(got.PriceHistory))
	}
	if got.PriceHistory[0].Regular != 9.99 {
		t.Errorf("first snapshot = %.2f, want 9.99", got.PriceHistory[0].Regular)
	}
	if got.PriceHistory[2].Regular != 7.99 {
		t.Errorf("last snapshot = %.2f, want 7.99 (fresh)", got.PriceHistory[2].Regular)
	}
	if got.LowestPrice != 4.99 {
		t.Errorf("lowestPrice = %.2f, want 4.99", got.LowestPrice)
	}
}

func TestCarryOverPriceHistory_noExistingHistory(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := models.Scene{ID: "1"}
	fresh := models.Scene{ID: "1"}
	fresh.AddPrice(snap(t0, 9.99))

	got := carryOverPriceHistory(fresh, existing)
	if len(got.PriceHistory) != 1 {
		t.Fatalf("got %d snapshots, want 1", len(got.PriceHistory))
	}
	if got.LowestPrice != 9.99 {
		t.Errorf("lowestPrice = %.2f", got.LowestPrice)
	}
}

func TestCarryOverPriceHistory_noFreshPrice(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := models.Scene{ID: "1"}
	existing.AddPrice(snap(t0, 5.00))

	fresh := models.Scene{ID: "1"}

	got := carryOverPriceHistory(fresh, existing)
	if len(got.PriceHistory) != 1 {
		t.Fatalf("got %d snapshots, want 1", len(got.PriceHistory))
	}
	if got.LowestPrice != 5.00 {
		t.Errorf("lowestPrice = %.2f", got.LowestPrice)
	}
}

func TestCarryOverPriceHistory_bothEmpty(t *testing.T) {
	got := carryOverPriceHistory(models.Scene{ID: "1"}, models.Scene{ID: "1"})
	if len(got.PriceHistory) != 0 {
		t.Errorf("got %d snapshots, want 0", len(got.PriceHistory))
	}
	if got.LowestPriceDate != nil {
		t.Error("lowestPriceDate should be nil")
	}
}

func TestCarryOverPriceHistory_freshLowerPrice(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	existing := models.Scene{ID: "1"}
	existing.AddPrice(snap(t0, 9.99))

	fresh := models.Scene{ID: "1"}
	fresh.AddPrice(snap(t1, 2.99))

	got := carryOverPriceHistory(fresh, existing)
	if got.LowestPrice != 2.99 {
		t.Errorf("lowestPrice = %.2f, want 2.99", got.LowestPrice)
	}
	if !got.LowestPriceDate.Equal(t1) {
		t.Errorf("lowestPriceDate = %v, want %v", got.LowestPriceDate, t1)
	}
}

func TestCarryOverPriceHistory_freeSnap(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	existing := models.Scene{ID: "1"}
	existing.AddPrice(snap(t0, 9.99))

	fresh := models.Scene{ID: "1"}
	fresh.AddPrice(freeSnap(t1))

	got := carryOverPriceHistory(fresh, existing)
	if got.LowestPrice != 0 {
		t.Errorf("lowestPrice = %.2f, want 0 (free)", got.LowestPrice)
	}
}
