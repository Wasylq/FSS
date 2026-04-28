// Package models defines the data types produced by scrapers.
package models

import "time"

// PriceSnapshot captures pricing at a single point in time.
type PriceSnapshot struct {
	Date            time.Time `json:"date"`
	Regular         float64   `json:"regular"`
	Discounted      float64   `json:"discounted,omitempty"`
	IsFree          bool      `json:"isFree"`
	IsOnSale        bool      `json:"isOnSale"`
	DiscountPercent int       `json:"discountPercent,omitempty"`
}

// Effective returns what you would pay at this snapshot.
func (p PriceSnapshot) Effective() float64 {
	if p.IsFree {
		return 0
	}
	if p.IsOnSale {
		return p.Discounted
	}
	return p.Regular
}

// Scene holds all metadata for a single scraped scene. Fields vary by site —
// only ID, SiteID, Title, URL, and ScrapedAt are guaranteed to be populated.
type Scene struct {
	// Identity
	ID        string `json:"id"`
	SiteID    string `json:"siteId"`
	StudioURL string `json:"studioUrl"`

	// Core
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Date        time.Time `json:"date"`
	Description string    `json:"description,omitempty"`

	// Media
	Thumbnail string `json:"thumbnail,omitempty"`
	Preview   string `json:"preview,omitempty"`

	// People & production
	Performers []string `json:"performers,omitempty"`
	Director   string   `json:"director,omitempty"`
	Studio     string   `json:"studio,omitempty"`

	// Classification
	Tags       []string `json:"tags,omitempty"`
	Categories []string `json:"categories,omitempty"`

	// Series
	Series     string `json:"series,omitempty"`
	SeriesPart int    `json:"seriesPart,omitempty"`

	// Technical
	Duration   int    `json:"duration,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Format     string `json:"format,omitempty"`

	// Engagement
	Views    int `json:"views,omitempty"`
	Likes    int `json:"likes,omitempty"`
	Comments int `json:"comments,omitempty"`

	// Pricing history
	PriceHistory    []PriceSnapshot `json:"priceHistory,omitempty"`
	LowestPrice     float64         `json:"lowestPrice,omitempty"`
	LowestPriceDate *time.Time      `json:"lowestPriceDate,omitempty"`

	// Scraper housekeeping
	ScrapedAt time.Time  `json:"scrapedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// AddPrice appends a new PriceSnapshot and updates LowestPrice/LowestPriceDate if the
// effective price is lower than the current record.
func (s *Scene) AddPrice(p PriceSnapshot) {
	s.PriceHistory = append(s.PriceHistory, p)
	effective := p.Effective()
	if s.LowestPriceDate == nil || effective < s.LowestPrice {
		s.LowestPrice = effective
		s.LowestPriceDate = &p.Date
	}
}
