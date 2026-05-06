package models

import (
	"testing"
	"time"
)

func TestAddPriceFirstCall(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	s.AddPrice(PriceSnapshot{Date: now, Regular: 9.99})
	if s.LowestPrice != 9.99 {
		t.Errorf("LowestPrice = %v, want 9.99", s.LowestPrice)
	}
	if s.LowestPriceDate == nil || !s.LowestPriceDate.Equal(now) {
		t.Errorf("LowestPriceDate = %v, want %v", s.LowestPriceDate, now)
	}
}

func TestAddPriceLowerUpdates(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	s.AddPrice(PriceSnapshot{Date: now, Regular: 19.99})
	later := now.Add(time.Hour)
	s.AddPrice(PriceSnapshot{Date: later, Regular: 9.99})
	if s.LowestPrice != 9.99 {
		t.Errorf("LowestPrice = %v, want 9.99", s.LowestPrice)
	}
	if !s.LowestPriceDate.Equal(later) {
		t.Errorf("LowestPriceDate = %v, want %v", s.LowestPriceDate, later)
	}
}

func TestAddPriceHigherKeepsLowest(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	s.AddPrice(PriceSnapshot{Date: now, Regular: 9.99})
	s.AddPrice(PriceSnapshot{Date: now.Add(time.Hour), Regular: 19.99})
	if s.LowestPrice != 9.99 {
		t.Errorf("LowestPrice = %v, want 9.99", s.LowestPrice)
	}
}

func TestAddPriceFreeIsValidLowest(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	s.AddPrice(PriceSnapshot{Date: now, Regular: 9.99})
	later := now.Add(time.Hour)
	s.AddPrice(PriceSnapshot{Date: later, IsFree: true})
	if s.LowestPrice != 0 {
		t.Errorf("LowestPrice = %v, want 0 (free)", s.LowestPrice)
	}
	if !s.LowestPriceDate.Equal(later) {
		t.Errorf("LowestPriceDate = %v, want %v", s.LowestPriceDate, later)
	}
}

func TestAddPriceUnknownDoesNotAffectLowest(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	// "Not free, but no price info" — should not set LowestPrice.
	s.AddPrice(PriceSnapshot{Date: now, IsFree: false})
	if s.LowestPriceDate != nil {
		t.Errorf("LowestPriceDate should be nil for unknown-price snapshot, got %v", s.LowestPriceDate)
	}
	if s.LowestPrice != 0 {
		t.Errorf("LowestPrice = %v, want 0", s.LowestPrice)
	}
}

func TestAddPriceUnknownThenReal(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	// First: unknown price (IsFree: false, Regular: 0).
	s.AddPrice(PriceSnapshot{Date: now, IsFree: false})
	// Second: real price appears.
	later := now.Add(time.Hour)
	s.AddPrice(PriceSnapshot{Date: later, Regular: 14.99})
	if s.LowestPrice != 14.99 {
		t.Errorf("LowestPrice = %v, want 14.99", s.LowestPrice)
	}
	if !s.LowestPriceDate.Equal(later) {
		t.Errorf("LowestPriceDate = %v, want %v", s.LowestPriceDate, later)
	}
	if len(s.PriceHistory) != 2 {
		t.Errorf("PriceHistory len = %d, want 2", len(s.PriceHistory))
	}
}

func TestAddPriceFreeThenPaid(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	s.AddPrice(PriceSnapshot{Date: now, IsFree: true})
	s.AddPrice(PriceSnapshot{Date: now.Add(time.Hour), Regular: 9.99})
	// Free ($0) is still the real lowest — scene was genuinely free once.
	if s.LowestPrice != 0 {
		t.Errorf("LowestPrice = %v, want 0", s.LowestPrice)
	}
	if !s.LowestPriceDate.Equal(now) {
		t.Errorf("LowestPriceDate should be the free date")
	}
}

func TestAddPriceOnSale(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	s.AddPrice(PriceSnapshot{Date: now, Regular: 29.99})
	later := now.Add(time.Hour)
	s.AddPrice(PriceSnapshot{Date: later, Regular: 29.99, Discounted: 14.99, IsOnSale: true})
	if s.LowestPrice != 14.99 {
		t.Errorf("LowestPrice = %v, want 14.99", s.LowestPrice)
	}
}

func TestAddPriceNegativeRegular(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	s.AddPrice(PriceSnapshot{Date: now, Regular: -5.00})
	if s.LowestPrice != -5.00 {
		t.Errorf("LowestPrice = %v, want -5.00", s.LowestPrice)
	}
	if len(s.PriceHistory) != 1 {
		t.Errorf("PriceHistory len = %d, want 1", len(s.PriceHistory))
	}
}

func TestAddPriceZeroRegularNotFree(t *testing.T) {
	var s Scene
	now := time.Now().UTC()
	s.AddPrice(PriceSnapshot{Date: now, Regular: 0, IsFree: false})
	if s.LowestPriceDate != nil {
		t.Error("zero regular + not free should not set LowestPriceDate")
	}
	if len(s.PriceHistory) != 1 {
		t.Errorf("PriceHistory len = %d, want 1 (still recorded)", len(s.PriceHistory))
	}
}

func TestEffective(t *testing.T) {
	cases := []struct {
		name string
		snap PriceSnapshot
		want float64
	}{
		{"regular", PriceSnapshot{Regular: 9.99}, 9.99},
		{"free", PriceSnapshot{IsFree: true}, 0},
		{"on sale", PriceSnapshot{Regular: 19.99, Discounted: 9.99, IsOnSale: true}, 9.99},
		{"unknown", PriceSnapshot{IsFree: false}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.snap.Effective(); got != c.want {
				t.Errorf("Effective() = %v, want %v", got, c.want)
			}
		})
	}
}
