package cmd

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Wasylq/FSS/internal/store"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
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

// TestCarryOverPriceHistory_multipleFreshSnapshots locks in the contract
// that EVERY snapshot a scraper emits on a fresh scene survives the
// carry-over — not just the last one. A previous implementation only
// kept the trailing snapshot, which silently dropped data on any
// scraper that ever emitted >1 snapshot per scrape (backfill, multi-
// tier pricing, batched updates).
func TestCarryOverPriceHistory_multipleFreshSnapshots(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	existing := models.Scene{ID: "1"}
	existing.AddPrice(snap(t0, 19.99))

	fresh := models.Scene{ID: "1"}
	fresh.AddPrice(snap(t1, 14.99))
	fresh.AddPrice(snap(t2, 9.99))
	fresh.AddPrice(snap(t3, 12.99))

	got := carryOverPriceHistory(fresh, existing)
	if len(got.PriceHistory) != 4 {
		t.Fatalf("got %d snapshots, want 4 (1 existing + 3 fresh)", len(got.PriceHistory))
	}
	// Order must be existing-first, fresh-in-emit-order.
	wantRegular := []float64{19.99, 14.99, 9.99, 12.99}
	for i, w := range wantRegular {
		if got.PriceHistory[i].Regular != w {
			t.Errorf("snapshot[%d].Regular = %.2f, want %.2f", i, got.PriceHistory[i].Regular, w)
		}
	}
	if got.LowestPrice != 9.99 {
		t.Errorf("LowestPrice = %.2f, want 9.99 (must consider every fresh snapshot)", got.LowestPrice)
	}
	if got.LowestPriceDate == nil || !got.LowestPriceDate.Equal(t2) {
		t.Errorf("LowestPriceDate = %v, want %v", got.LowestPriceDate, t2)
	}
}

// fakeScraper streams a fixed slice of scenes on each ListScenes call.
// Each invocation reads from the next entry in batches, mimicking
// successive scrape runs over time.
type fakeScraper struct {
	id      string
	batches [][]models.Scene
	call    int
}

func (f *fakeScraper) ID() string             { return f.id }
func (f *fakeScraper) Patterns() []string     { return nil }
func (f *fakeScraper) MatchesURL(string) bool { return true }

func (f *fakeScraper) ListScenes(_ context.Context, _ string, _ scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	ch := make(chan scraper.SceneResult, len(f.batches[f.call])+1)
	for _, s := range f.batches[f.call] {
		ch <- scraper.Scene(s)
	}
	close(ch)
	f.call++
	return ch, nil
}

// TestScrapeAll_preservesPriceHistory pins finding 1.1 from AUDIT.md: --full
// must carry forward existing price history across re-scrapes for both stores.
// Before the fix, scrapeAll discarded all prior snapshots — Flat by overwrite,
// SQLite by diff-based syncPriceHistory.
func TestScrapeAll_preservesPriceHistory(t *testing.T) {
	const studioURL = "https://example.com/studio/test"
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	build := func(date time.Time, price float64) models.Scene {
		s := models.Scene{
			ID:        "scene-1",
			SiteID:    "fakesite",
			StudioURL: studioURL,
			Title:     "Test Scene",
			URL:       "https://example.com/scene/1",
			ScrapedAt: date,
		}
		s.AddPrice(snap(date, price))
		return s
	}

	stores := map[string]func(t *testing.T) store.Store{
		"flat": func(t *testing.T) store.Store {
			return store.NewFlat(t.TempDir(), []string{"json"})
		},
		"sqlite": func(t *testing.T) store.Store {
			path := filepath.Join(t.TempDir(), "test.db")
			db, err := store.NewSQLite(path)
			if err != nil {
				t.Fatalf("NewSQLite: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })
			return db
		},
	}

	for name, factory := range stores {
		t.Run(name, func(t *testing.T) {
			st := factory(t)

			// Initial scrape: store one snapshot at $9.99.
			if err := st.Save(studioURL, []models.Scene{build(t0, 9.99)}); err != nil {
				t.Fatalf("seed save: %v", err)
			}

			// --full re-scrape with a different price.
			sc := &fakeScraper{
				id:      "fakesite",
				batches: [][]models.Scene{{build(t1, 4.99)}},
			}
			scenes, err := scrapeAll(context.Background(), sc, st, studioURL, 1, 0)
			if err != nil {
				t.Fatalf("scrapeAll: %v", err)
			}
			if err := st.Save(studioURL, scenes); err != nil {
				t.Fatalf("save: %v", err)
			}

			got, err := st.Load(studioURL)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d scenes, want 1", len(got))
			}
			ph := got[0].PriceHistory
			if len(ph) != 2 {
				t.Fatalf("price history has %d snapshots, want 2: %+v", len(ph), ph)
			}
			if ph[0].Regular != 9.99 {
				t.Errorf("snapshot[0] = %.2f, want 9.99 (original)", ph[0].Regular)
			}
			if ph[1].Regular != 4.99 {
				t.Errorf("snapshot[1] = %.2f, want 4.99 (fresh)", ph[1].Regular)
			}
			if got[0].LowestPrice != 4.99 {
				t.Errorf("lowestPrice = %.2f, want 4.99", got[0].LowestPrice)
			}
		})
	}
}

// TestScrapeAll_dropsMissingScenes verifies the documented difference between
// --full and --refresh: scenes that no longer appear on the site are dropped,
// not soft-deleted.
func TestScrapeAll_dropsMissingScenes(t *testing.T) {
	const studioURL = "https://example.com/studio/test"
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	build := func(id string) models.Scene {
		return models.Scene{
			ID:        id,
			SiteID:    "fakesite",
			StudioURL: studioURL,
			Title:     "Scene " + id,
			URL:       "https://example.com/scene/" + id,
			ScrapedAt: t0,
		}
	}

	st := store.NewFlat(t.TempDir(), []string{"json"})
	if err := st.Save(studioURL, []models.Scene{build("1"), build("2")}); err != nil {
		t.Fatalf("seed save: %v", err)
	}

	// --full sees only scene 2; scene 1 should be dropped (no soft-delete).
	sc := &fakeScraper{
		id:      "fakesite",
		batches: [][]models.Scene{{build("2")}},
	}
	scenes, err := scrapeAll(context.Background(), sc, st, studioURL, 1, 0)
	if err != nil {
		t.Fatalf("scrapeAll: %v", err)
	}
	if len(scenes) != 1 || scenes[0].ID != "2" {
		t.Fatalf("got %d scenes (ids=%v), want [2]", len(scenes), sceneIDs(scenes))
	}
	if scenes[0].DeletedAt != nil {
		t.Error("scene 2 should not be soft-deleted")
	}
}

func sceneIDs(s []models.Scene) []string {
	ids := make([]string, len(s))
	for i, sc := range s {
		ids[i] = sc.ID
	}
	return ids
}

func TestParseFormats_json(t *testing.T) {
	got, err := parseFormats("json")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"json"}) {
		t.Errorf("got %v, want [json]", got)
	}
}

func TestParseFormats_csv(t *testing.T) {
	got, err := parseFormats("csv")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"csv"}) {
		t.Errorf("got %v, want [csv]", got)
	}
}

func TestParseFormats_both(t *testing.T) {
	got, err := parseFormats("json,csv")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"json", "csv"}) {
		t.Errorf("got %v, want [json csv]", got)
	}
}

func TestParseFormats_dedup(t *testing.T) {
	got, err := parseFormats("json,json")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"json"}) {
		t.Errorf("got %v, want [json]", got)
	}
}

func TestParseFormats_unknown(t *testing.T) {
	_, err := parseFormats("xml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Errorf("error should mention the unknown format, got: %v", err)
	}
}

func TestParseFormats_empty(t *testing.T) {
	got, err := parseFormats("")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestParseFormats_whitespace(t *testing.T) {
	got, err := parseFormats(" json , csv ")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"json", "csv"}) {
		t.Errorf("got %v, want [json csv]", got)
	}
}
