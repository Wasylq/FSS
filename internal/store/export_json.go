package store

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Wasylq/FSS/models"
)

// studioFile is the top-level JSON structure written per studio.
type studioFile struct {
	StudioURL  string         `json:"studioUrl"`
	ScrapedAt  time.Time      `json:"scrapedAt"`
	SceneCount int            `json:"sceneCount"`
	Scenes     []models.Scene `json:"scenes"`
}

func WriteJSON(sf studioFile, path string) error {
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling JSON: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
