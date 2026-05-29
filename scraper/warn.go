package scraper

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// warnedDelays remembers which (siteID, recommended) pairs have already
// emitted a delay warning so the message is logged at most once per
// scraper-process lifetime even when ListScenes is called repeatedly.
var (
	warnedDelaysMu sync.Mutex
	warnedDelays   = map[string]bool{}
)

// WarnDelayBelow prints a one-shot stderr warning when `actual` is
// strictly below `recommended` for scraper `siteID`. Use it to flag
// when the operator's chosen delay is below a value the upstream site
// is known to need to avoid rate-limiting. Unlike Debugf this prints
// regardless of verbosity, but only once per siteID per process so it
// doesn't spam.
//
// `recommended` of 0 or `actual` of 0 are no-ops if recommended <= 0;
// a recommended floor of zero means "no minimum", so nothing to warn
// about. Callers should pass the package's documented minimum (e.g.
// 500ms for julesjordan / newsensations).
func WarnDelayBelow(siteID string, actual, recommended time.Duration) {
	if recommended <= 0 || actual >= recommended {
		return
	}
	warnedDelaysMu.Lock()
	if warnedDelays[siteID] {
		warnedDelaysMu.Unlock()
		return
	}
	warnedDelays[siteID] = true
	warnedDelaysMu.Unlock()
	fmt.Fprintf(os.Stderr,
		"[warn] %s: configured delay %v is below the recommended minimum %v; "+
			"upstream may rate-limit. Set `site_delays: { %s: %d }` in config or "+
			"`--site-delay %s=%d` to override.\n",
		siteID, actual, recommended, siteID, recommended.Milliseconds(), siteID, recommended.Milliseconds())
}

// ResetWarnDelayBelow clears the once-per-siteID memoisation. Intended
// for tests that need to re-trigger the warning across cases.
func ResetWarnDelayBelow() {
	warnedDelaysMu.Lock()
	warnedDelays = map[string]bool{}
	warnedDelaysMu.Unlock()
}
