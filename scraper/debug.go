package scraper

import (
	"fmt"
	"os"
	"sync/atomic"
)

var verboseLevel atomic.Int32

// SetVerbose sets the global debug verbosity level.
// Level 0 = silent (default), 1+ = increasingly verbose.
func SetVerbose(level int) { verboseLevel.Store(int32(level)) }

// Verbose returns the current debug verbosity level.
func Verbose() int { return int(verboseLevel.Load()) }

// Debugf prints a debug message to stderr if the current verbosity level
// is >= the requested level. Format follows fmt.Fprintf conventions.
//
// Levels by convention:
//
//	1 — high-level operations (pages fetched, items found, categories discovered)
//	2 — HTTP requests (URL, method, status, size)
//	3 — parsing details (regex matches, extracted fields)
func Debugf(level int, format string, args ...any) {
	if int(verboseLevel.Load()) >= level {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}
