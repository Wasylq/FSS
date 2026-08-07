package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/Wasylq/FSS/internal/store"
)

// sameStudioKey reduces a studio URL to what actually identifies the resource:
// lowercase host, no trailing slash, scheme ignored. Two URLs sharing a key are
// almost certainly the same catalogue.
//
// This is used to *warn*, never to rewrite. `studio_url` is both the storage key
// and the address scrapers fetch, so canonicalising it would change which page
// is requested — and some sites are http-only. Rewriting it would also need a
// migration across the scenes table, five child tables and the flat store's
// filenames, since Slugify hashes the raw URL.
func sameStudioKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(strings.ToLower(raw), "/")
	}
	return strings.ToLower(u.Host) + strings.TrimRight(u.Path, "/")
}

// warnStudioURLVariant reports when studioURL differs from an already-tracked
// URL only cosmetically — http vs https, a trailing slash, host casing.
//
// Because studio_url is part of the primary key, such a variant silently forks
// one catalogue into two studios that never merge. Naming the stored spelling
// lets the operator reuse it instead.
func warnStudioURLVariant(st store.Store, studioURL string) {
	tracked, err := st.ListStudios()
	if err != nil || len(tracked) == 0 {
		return // flat store returns nothing; nothing to compare against
	}
	key := sameStudioKey(studioURL)
	for _, s := range tracked {
		if s.URL != studioURL && sameStudioKey(s.URL) == key {
			fmt.Fprintf(os.Stderr,
				"warning: %s differs from the stored %s only cosmetically, but studio URLs are "+
					"stored verbatim — this creates a second studio rather than updating the first. "+
					"Use the stored spelling to merge them.\n",
				studioURL, s.URL)
			return
		}
	}
}
