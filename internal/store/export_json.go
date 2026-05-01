package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	return atomicWriteFile(path, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// atomicWriteFile writes to a temporary file in the same directory as path,
// then renames it into place. This prevents a crash mid-write from corrupting
// the target file. The writeFn callback receives the temp file as an io.Writer.
func atomicWriteFile(path string, writeFn func(io.Writer) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".fss-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	defer func() {
		// Clean up the temp file on any failure path.
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}()

	if err := writeFn(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming %s → %s: %w", tmpPath, path, err)
	}
	return nil
}
