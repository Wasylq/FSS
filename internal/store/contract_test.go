package store

import (
	"sort"
	"testing"
	"time"

	"github.com/Wasylq/FSS/models"
)

const contractURL = "https://www.contract-test.com/videos"

func contractScenes(now time.Time) []models.Scene {
	return []models.Scene{
		{
			ID:         "s1",
			SiteID:     "ctest",
			StudioURL:  contractURL,
			Title:      "First Scene",
			URL:        "https://www.contract-test.com/video/1",
			Date:       now.Add(-48 * time.Hour),
			Performers: []string{"Alice", "Bob"},
			Tags:       []string{"tag-a", "tag-b"},
			Studio:     "Contract Studio",
			Duration:   600,
			ScrapedAt:  now,
		},
		{
			ID:         "s2",
			SiteID:     "ctest",
			StudioURL:  contractURL,
			Title:      "Second Scene",
			URL:        "https://www.contract-test.com/video/2",
			Date:       now.Add(-24 * time.Hour),
			Performers: []string{"Charlie"},
			Tags:       []string{"tag-c"},
			Studio:     "Contract Studio",
			Duration:   900,
			ScrapedAt:  now,
		},
	}
}

type storeFactory struct {
	name string
	new  func(t *testing.T) Store
}

func storeFactories(t *testing.T) []storeFactory {
	t.Helper()
	return []storeFactory{
		{"Flat", func(t *testing.T) Store {
			t.Helper()
			return NewFlat(t.TempDir(), []string{"json"})
		}},
		{"SQLite", func(t *testing.T) Store {
			t.Helper()
			s, err := NewSQLite(":memory:")
			if err != nil {
				t.Fatalf("NewSQLite: %v", err)
			}
			t.Cleanup(func() { _ = s.Close() })
			return s
		}},
	}
}

func TestContract_LoadEmpty(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			got, err := s.Load(contractURL)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("expected 0 scenes, got %d", len(got))
			}
		})
	}
}

func TestContract_SaveLoadRoundTrip(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			now := time.Now().UTC().Truncate(time.Second)
			scenes := contractScenes(now)

			if err := s.Save(contractURL, scenes); err != nil {
				t.Fatalf("Save: %v", err)
			}

			got, err := s.Load(contractURL)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("got %d scenes, want 2", len(got))
			}

			sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })

			g := got[0]
			if g.ID != "s1" || g.SiteID != "ctest" || g.Title != "First Scene" {
				t.Errorf("scene 0: id=%q siteID=%q title=%q", g.ID, g.SiteID, g.Title)
			}
			if g.Duration != 600 {
				t.Errorf("duration = %d, want 600", g.Duration)
			}
			if len(g.Performers) != 2 || g.Performers[0] != "Alice" || g.Performers[1] != "Bob" {
				t.Errorf("performers = %v", g.Performers)
			}
			if len(g.Tags) != 2 {
				t.Errorf("tags = %v", g.Tags)
			}

			g = got[1]
			if g.ID != "s2" || g.Title != "Second Scene" {
				t.Errorf("scene 1: id=%q title=%q", g.ID, g.Title)
			}
			if len(g.Performers) != 1 || g.Performers[0] != "Charlie" {
				t.Errorf("performers = %v", g.Performers)
			}
		})
	}
}

func TestContract_SaveDropsMissing(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			now := time.Now().UTC().Truncate(time.Second)

			if err := s.Save(contractURL, contractScenes(now)); err != nil {
				t.Fatalf("Save 1: %v", err)
			}

			kept := []models.Scene{contractScenes(now)[0]}
			if err := s.Save(contractURL, kept); err != nil {
				t.Fatalf("Save 2: %v", err)
			}

			got, err := s.Load(contractURL)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d scenes, want 1", len(got))
			}
			if got[0].ID != "s1" {
				t.Errorf("surviving scene ID = %q, want s1", got[0].ID)
			}
		})
	}
}

func TestContract_SaveRejectsEmptyKeys(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			now := time.Now().UTC().Truncate(time.Second)

			noID := []models.Scene{{SiteID: "x", Title: "no-id", ScrapedAt: now}}
			if err := s.Save(contractURL, noID); err == nil {
				t.Error("Save with empty ID should fail")
			}

			noSite := []models.Scene{{ID: "1", Title: "no-site", ScrapedAt: now}}
			if err := s.Save(contractURL, noSite); err == nil {
				t.Error("Save with empty SiteID should fail")
			}
		})
	}
}

func TestContract_MarkDeleted(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			now := time.Now().UTC().Truncate(time.Second)
			scenes := contractScenes(now)

			if err := s.Save(contractURL, scenes); err != nil {
				t.Fatalf("Save: %v", err)
			}

			if err := s.MarkDeleted(contractURL, "ctest", []string{"s1"}); err != nil {
				t.Fatalf("MarkDeleted: %v", err)
			}

			got, err := s.Load(contractURL)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(got) != 2 {
				t.Fatalf("got %d scenes, want 2 (soft-delete preserves)", len(got))
			}

			sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })

			if got[0].DeletedAt == nil {
				t.Error("s1 should have DeletedAt set")
			}
			if got[1].DeletedAt != nil {
				t.Error("s2 should NOT have DeletedAt set")
			}
		})
	}
}

func TestContract_MarkDeletedSiteIDScoped(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			now := time.Now().UTC().Truncate(time.Second)

			scenes := []models.Scene{
				{ID: "shared", SiteID: "site-a", StudioURL: contractURL, Title: "A", ScrapedAt: now},
				{ID: "shared", SiteID: "site-b", StudioURL: contractURL, Title: "B", ScrapedAt: now},
			}
			if err := s.Save(contractURL, scenes); err != nil {
				t.Fatalf("Save: %v", err)
			}

			if err := s.MarkDeleted(contractURL, "site-a", []string{"shared"}); err != nil {
				t.Fatalf("MarkDeleted: %v", err)
			}

			got, err := s.Load(contractURL)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			sort.Slice(got, func(i, j int) bool { return got[i].SiteID < got[j].SiteID })

			if got[0].SiteID != "site-a" || got[0].DeletedAt == nil {
				t.Errorf("site-a scene: deletedAt=%v, want non-nil", got[0].DeletedAt)
			}
			if got[1].SiteID != "site-b" || got[1].DeletedAt != nil {
				t.Errorf("site-b scene: deletedAt=%v, want nil", got[1].DeletedAt)
			}
		})
	}
}

func TestContract_AutoRevive(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			now := time.Now().UTC().Truncate(time.Second)
			scenes := contractScenes(now)

			if err := s.Save(contractURL, scenes); err != nil {
				t.Fatalf("Save 1: %v", err)
			}
			if err := s.MarkDeleted(contractURL, "ctest", []string{"s1"}); err != nil {
				t.Fatalf("MarkDeleted: %v", err)
			}

			// Re-save with s1 having nil DeletedAt — should auto-revive.
			if err := s.Save(contractURL, scenes); err != nil {
				t.Fatalf("Save 2: %v", err)
			}

			got, err := s.Load(contractURL)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })

			if got[0].DeletedAt != nil {
				t.Error("s1 should be revived (DeletedAt == nil)")
			}
		})
	}
}

func TestContract_PriceHistory(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			now := time.Now().UTC().Truncate(time.Second)

			scene := models.Scene{
				ID:        "p1",
				SiteID:    "ctest",
				StudioURL: contractURL,
				Title:     "Priced",
				ScrapedAt: now,
			}
			scene.AddPrice(models.PriceSnapshot{Date: now, Regular: 9.99})

			if err := s.Save(contractURL, []models.Scene{scene}); err != nil {
				t.Fatalf("Save 1: %v", err)
			}

			got, err := s.Load(contractURL)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d scenes", len(got))
			}
			if len(got[0].PriceHistory) != 1 {
				t.Fatalf("price history len = %d, want 1", len(got[0].PriceHistory))
			}
			if got[0].PriceHistory[0].Regular != 9.99 {
				t.Errorf("price = %f, want 9.99", got[0].PriceHistory[0].Regular)
			}
			if got[0].LowestPrice != 9.99 {
				t.Errorf("lowest = %f, want 9.99", got[0].LowestPrice)
			}
		})
	}
}

func TestContract_SaveOtherStudioUnaffected(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			now := time.Now().UTC().Truncate(time.Second)

			url1 := "https://studio-a.com"
			url2 := "https://studio-b.com"

			s1 := []models.Scene{{ID: "a1", SiteID: "sa", StudioURL: url1, Title: "A1", ScrapedAt: now}}
			s2 := []models.Scene{{ID: "b1", SiteID: "sb", StudioURL: url2, Title: "B1", ScrapedAt: now}}

			if err := s.Save(url1, s1); err != nil {
				t.Fatalf("Save url1: %v", err)
			}
			if err := s.Save(url2, s2); err != nil {
				t.Fatalf("Save url2: %v", err)
			}

			// Overwrite url1 with empty — should not affect url2.
			if err := s.Save(url1, nil); err != nil {
				t.Fatalf("Save url1 empty: %v", err)
			}

			got1, _ := s.Load(url1)
			got2, _ := s.Load(url2)

			if len(got1) != 0 {
				t.Errorf("url1 should be empty, got %d", len(got1))
			}
			if len(got2) != 1 || got2[0].ID != "b1" {
				t.Errorf("url2 should have b1, got %v", got2)
			}
		})
	}
}

func TestContract_MarkDeletedNonexistentID(t *testing.T) {
	for _, sf := range storeFactories(t) {
		t.Run(sf.name, func(t *testing.T) {
			s := sf.new(t)
			now := time.Now().UTC().Truncate(time.Second)

			if err := s.Save(contractURL, contractScenes(now)); err != nil {
				t.Fatalf("Save: %v", err)
			}

			// Marking a nonexistent ID should not error.
			if err := s.MarkDeleted(contractURL, "ctest", []string{"nonexistent"}); err != nil {
				t.Fatalf("MarkDeleted nonexistent: %v", err)
			}

			got, _ := s.Load(contractURL)
			for _, sc := range got {
				if sc.DeletedAt != nil {
					t.Errorf("scene %q should not be soft-deleted", sc.ID)
				}
			}
		})
	}
}
