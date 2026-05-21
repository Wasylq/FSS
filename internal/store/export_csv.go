package store

import (
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/output"
)

func WriteCSV(scenes []models.Scene, path string) error {
	return output.WriteCSV(scenes, path)
}
