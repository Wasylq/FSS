package store

import (
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/output"
)

func WriteCSV(scenes []models.Scene, path string) error {
	return output.WriteCSV(scenes, path)
}
