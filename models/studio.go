package models

import "time"

// Studio is the tracked identity of a scraped site or creator page. It is the
// key under which scenes are grouped and persisted.
type Studio struct {
	URL           string
	SiteID        string
	Name          string // user-supplied label; falls back to creator name from scenes
	AddedAt       time.Time
	LastScrapedAt *time.Time
}
