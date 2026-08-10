package store

import (
	"github.com/Anastylosis/FSS/models"
	"github.com/Anastylosis/FSS/output"
)

func WriteJSON(sf models.StudioFile, path string) error {
	return output.WriteJSON(sf, path)
}
