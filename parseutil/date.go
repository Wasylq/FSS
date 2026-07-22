// Package parseutil holds the parsing helpers shared across FSS scrapers:
// duration parsing (colon-separated and ISO 8601), date normalisation,
// Open Graph meta extraction, and schema.org VideoObject extraction from
// JSON-LD. Scrapers should reach for these rather than reimplementing them.
package parseutil

import (
	"fmt"
	"time"
)

// TryParseDate attempts each layout in order and returns the first
// successful parse. Returns a zero time and an error if none match.
// Callers choose the layout set — there is no grab-bag of every
// known format, since mixing ambiguous formats (e.g. M/D/Y vs D/M/Y)
// would risk silent mis-classification.
func TryParseDate(s string, layouts ...string) (time.Time, error) {
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parseutil.TryParseDate: no layout matched %q", s)
}
