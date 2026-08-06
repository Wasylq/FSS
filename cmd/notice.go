package cmd

import (
	"fmt"
	"os"
	"sync"
)

// noticeOnce keeps a heads-up to one printing per process, so a run that
// scrapes several URLs does not repeat itself.
var noticeOnce sync.Once

// onceReset returns a fresh sync.Once; used by tests to re-arm the guard.
func onceReset() sync.Once { return sync.Once{} }

// noticesSuppressed reports whether the operator has opted out of advisory
// notices. Cron jobs and scripts want quiet output, and a notice about a
// future release is not worth polluting their logs with.
func noticesSuppressed() bool {
	if os.Getenv("FSS_NO_NOTICES") != "" {
		return true
	}
	return cfg != nil && cfg.Notices != nil && !*cfg.Notices
}

// warnFlatStoreDefaultChanging tells operators still on the flat JSON store
// that SQLite is going to become the default, before it happens rather than
// after.
//
// Changing where a tool keeps its data is the kind of surprise that costs
// people an afternoon, so the notice ships ahead of the change and names the
// two ways to stay in control: adopt the database now with `fss import`, or
// pin the flat store in config.
//
// Printed at most once per process, to stderr so it never contaminates piped
// output, and skipped entirely when notices are suppressed.
func warnFlatStoreDefaultChanging(usingDB bool) {
	if usingDB || noticesSuppressed() {
		return
	}
	noticeOnce.Do(func() {
		fmt.Fprint(os.Stderr,
			"[notice] A future release will use the SQLite store by default.\n"+
				"         Nothing changes today. To move now: `fss import --db ./`\n"+
				"         To keep JSON files: set `db: \"\"` in your config.\n"+
				"         Silence this: `notices: false` in config, or FSS_NO_NOTICES=1.\n")
	})
}
