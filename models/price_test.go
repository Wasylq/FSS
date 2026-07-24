package models

import (
	"testing"
	"time"
)

func snap(d time.Time, regular float64) PriceSnapshot {
	return PriceSnapshot{Date: d, Regular: regular}
}

// AddPrice must collapse consecutive identical snapshots. carryOverPriceHistory
// replays the entire stored history through AddPrice on every scrape, so
// appending unconditionally grows the file without bound on a repeating cron.
func TestAddPriceCollapsesConsecutiveDuplicates(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var s Scene
	for i := range 50 {
		s.AddPrice(snap(base.AddDate(0, 0, i), 19.99))
	}

	if len(s.PriceHistory) != 1 {
		t.Errorf("PriceHistory has %d entries after 50 identical snapshots, want 1", len(s.PriceHistory))
	}
	if s.LowestPrice != 19.99 {
		t.Errorf("LowestPrice = %v, want 19.99", s.LowestPrice)
	}
	// The retained entry must be the first observation, not the last, so the
	// date reflects when the price started.
	if got := s.PriceHistory[0].Date; !got.Equal(base) {
		t.Errorf("kept snapshot dated %v, want the first observation %v", got, base)
	}
}

// Only *consecutive* repeats collapse: a price that changes and returns is a
// real event and must stay in the log.
func TestAddPriceKeepsNonConsecutiveRepeats(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var s Scene
	s.AddPrice(snap(base, 19.99))
	s.AddPrice(snap(base.AddDate(0, 0, 1), 9.99))
	s.AddPrice(snap(base.AddDate(0, 0, 2), 19.99))

	if len(s.PriceHistory) != 3 {
		t.Fatalf("PriceHistory has %d entries, want 3 (19.99 -> 9.99 -> 19.99)", len(s.PriceHistory))
	}
	if s.LowestPrice != 9.99 {
		t.Errorf("LowestPrice = %v, want 9.99", s.LowestPrice)
	}
}

// A snapshot differing only in a sale flag is a real change, not a duplicate.
func TestAddPriceTreatsSaleFlagChangeAsNew(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var s Scene
	s.AddPrice(PriceSnapshot{Date: base, Regular: 20})
	s.AddPrice(PriceSnapshot{Date: base.AddDate(0, 0, 1), Regular: 20, IsOnSale: true, Discounted: 15, DiscountPercent: 25})

	if len(s.PriceHistory) != 2 {
		t.Fatalf("PriceHistory has %d entries, want 2", len(s.PriceHistory))
	}
	if s.LowestPrice != 15 {
		t.Errorf("LowestPrice = %v, want 15", s.LowestPrice)
	}
}

// A 100%-off sale genuinely costs nothing, so it must set the lowest price.
// Without the DiscountPercent check it is indistinguishable from "not free,
// amount unknown" and gets skipped.
func TestAddPriceRecordsFullDiscountAsZero(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var s Scene
	s.AddPrice(PriceSnapshot{Date: base, Regular: 25})
	s.AddPrice(PriceSnapshot{
		Date: base.AddDate(0, 0, 1), Regular: 25,
		IsOnSale: true, Discounted: 0, DiscountPercent: 100,
	})

	if s.LowestPrice != 0 {
		t.Errorf("LowestPrice = %v, want 0 for a 100%%-off sale", s.LowestPrice)
	}
	if s.LowestPriceDate == nil {
		t.Fatal("LowestPriceDate not set for a full-discount sale")
	}
	if want := base.AddDate(0, 0, 1); !s.LowestPriceDate.Equal(want) {
		t.Errorf("LowestPriceDate = %v, want %v", s.LowestPriceDate, want)
	}
}

// A snapshot with no pricing information at all is still logged, but must not
// claim a zero lowest price.
func TestAddPriceIgnoresInformationFreeSnapshotForLowest(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var s Scene
	s.AddPrice(PriceSnapshot{Date: base, IsFree: false})

	if len(s.PriceHistory) != 1 {
		t.Errorf("PriceHistory has %d entries, want 1", len(s.PriceHistory))
	}
	if s.LowestPriceDate != nil {
		t.Errorf("LowestPriceDate = %v, want nil — the snapshot carries no price", s.LowestPriceDate)
	}
	if s.LowestPrice != 0 {
		t.Errorf("LowestPrice = %v, want 0 (unset)", s.LowestPrice)
	}
}

// Free really is zero and must be tracked.
func TestAddPriceTracksFree(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var s Scene
	s.AddPrice(PriceSnapshot{Date: base, Regular: 10})
	s.AddPrice(PriceSnapshot{Date: base.AddDate(0, 0, 1), IsFree: true})

	if s.LowestPrice != 0 || s.LowestPriceDate == nil {
		t.Errorf("LowestPrice = %v, LowestPriceDate = %v; want 0 and set", s.LowestPrice, s.LowestPriceDate)
	}
}
