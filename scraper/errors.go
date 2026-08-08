package scraper

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// FailureKind classifies why a scrape step failed.
//
// Scrapers report non-fatal failures as [Error] results on the channel, and for
// a long time every one of them was an undifferentiated `error`. Three very
// different situations were reaching the consumer as the same value:
//
//   - the page never arrived — the scenes on it are missing, and a retry
//     might well get them
//   - the page arrived but could not be understood — the scenes on it are
//     also missing, but the site changed and the parser needs fixing
//   - the requested thing is legitimately not there — nothing is missing
//
// The distinction is load-bearing rather than cosmetic. `--full` and
// `--refresh` treat a traversal as authoritative over the studio's whole scene
// set, so an unreached page has to suppress the destructive delete, while an
// optional sub-listing that genuinely 404s must not — inflating the error count
// with expected absences made every such run non-authoritative.
type FailureKind int

const (
	// FailureUnknown is a failure that has not been classified. It is the
	// conservative default: the plain errors scrapers already return land
	// here, and are treated as potentially-missing data exactly as they were
	// before this classification existed.
	FailureUnknown FailureKind = iota

	// FailureTransport means the page never arrived — a network error, a
	// timeout, a cancelled context, or a status that says the server would
	// not serve it (5xx, 429, 403).
	FailureTransport

	// FailureParse means the page arrived intact but could not be read: a
	// missing block, a selector that matches nothing, an unparseable date.
	// This is the site-redesign signal, and the one worth acting on — it
	// will not fix itself on a retry.
	FailureParse

	// FailureAbsent means the requested resource is legitimately gone (404,
	// 410) or the listing is genuinely empty. No data is missing, so a
	// traversal that only saw these is still complete.
	FailureAbsent
)

// String returns the lowercase name of the failure kind.
func (k FailureKind) String() string {
	switch k {
	case FailureTransport:
		return "transport"
	case FailureParse:
		return "parse"
	case FailureAbsent:
		return "absent"
	case FailureUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("FailureKind(%d)", int(k))
	}
}

// MissingData reports whether a failure of this kind means scenes went
// uncollected, and therefore that a traversal cannot be treated as the studio's
// complete state.
//
// Everything except [FailureAbsent] counts, including [FailureUnknown]: an
// error nobody classified might have cost us a page, and wrongly believing a
// traversal complete is what deletes a catalogue.
func (k FailureKind) MissingData() bool { return k != FailureAbsent }

// ScrapeError annotates an error with a [FailureKind] and the URL it concerns,
// so the cmd layer can tell an unreached page from an absent one without
// pattern-matching on error strings.
//
// Build one with [TransportError], [ParseError] or [AbsentError] rather than
// filling the struct directly.
type ScrapeError struct {
	// Kind is why the step failed.
	Kind FailureKind
	// URL is the page being fetched or parsed, if known.
	URL string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *ScrapeError) Error() string {
	if e.URL == "" {
		return fmt.Sprintf("%s: %v", e.Kind, e.Err)
	}
	return fmt.Sprintf("%s: %s: %v", e.Kind, e.URL, e.Err)
}

// Unwrap returns the underlying error so errors.Is/As see through the
// annotation.
func (e *ScrapeError) Unwrap() error { return e.Err }

// FailureKind reports how this error should be classified. It is what [Classify]
// looks for, and any error type may implement the same method to opt in.
func (e *ScrapeError) FailureKind() FailureKind { return e.Kind }

// TransportError marks err as a page that never arrived. Scrapers rarely need
// this — [Classify] already recognises network errors and httpx status errors —
// but it is here for transports it cannot see into.
func TransportError(url string, err error) error {
	return &ScrapeError{Kind: FailureTransport, URL: url, Err: err}
}

// ParseError marks err as a page that arrived but could not be understood. This
// is the one worth reaching for by hand: a parser that quietly returns nothing
// is indistinguishable from a site with nothing on it, and the difference
// decides whether an authoritative save may delete scenes.
func ParseError(url string, err error) error {
	return &ScrapeError{Kind: FailureParse, URL: url, Err: err}
}

// AbsentError marks err as a resource that is legitimately not there, so it does
// not count against traversal completeness. Use it for optional sub-listings a
// site may simply not have — not as a way to silence a fetch that failed.
func AbsentError(url string, err error) error {
	return &ScrapeError{Kind: FailureAbsent, URL: url, Err: err}
}

// kinded is implemented by errors that classify themselves. httpx.StatusError
// does, which is how an HTTP status reaches this package: httpx imports scraper
// for debug logging, so the dependency cannot run the other way and the check
// has to be structural rather than a type assertion on the concrete type.
type kinded interface{ FailureKind() FailureKind }

// Classify determines the [FailureKind] of an error.
//
// It looks for an error in the chain that classifies itself (see [ScrapeError]),
// then falls back to recognising cancellation and network errors as transport
// failures. Anything else is [FailureUnknown], which callers must treat as
// possibly-missing data.
//
// A nil error is [FailureUnknown]; callers should not be classifying success.
func Classify(err error) FailureKind {
	if err == nil {
		return FailureUnknown
	}

	// errors.As walks joined errors too, which matters because httpx joins
	// one error per retry attempt — the classification of any attempt in the
	// chain is the classification of the whole failure.
	var k kinded
	if errors.As(err, &k) {
		return k.FailureKind()
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return FailureTransport
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return FailureTransport
	}

	return FailureUnknown
}
