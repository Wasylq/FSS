package models

import "time"

type Studio struct {
	URL           string
	SiteID        string
	Name          string // user-supplied label; falls back to creator name from scenes
	AddedAt       time.Time
	LastScrapedAt *time.Time
}
