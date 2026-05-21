package store

import (
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/output"
)

func WriteJSON(sf models.StudioFile, path string) error {
	return output.WriteJSON(sf, path)
}
