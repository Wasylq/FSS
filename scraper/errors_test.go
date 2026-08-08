package scraper_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/Wasylq/FSS/scraper"
)

// kindedStub stands in for httpx.StatusError, which cannot be used here: httpx
// imports scraper, so the test for the extension point has to supply its own
// implementor rather than import the real one.
type kindedStub struct{ kind scraper.FailureKind }

func (k kindedStub) Error() string                    { return "stub" }
func (k kindedStub) FailureKind() scraper.FailureKind { return k.kind }

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want scraper.FailureKind
	}{
		// The conservative default is the whole point: the plain errors the
		// existing scrapers return must keep counting as missing data.
		{"plain error", errors.New("boom"), scraper.FailureUnknown},
		{"nil", nil, scraper.FailureUnknown},

		{"tagged transport", scraper.TransportError("u", errors.New("boom")), scraper.FailureTransport},
		{"tagged parse", scraper.ParseError("u", errors.New("boom")), scraper.FailureParse},
		{"tagged absent", scraper.AbsentError("u", errors.New("boom")), scraper.FailureAbsent},

		{"cancelled context", context.Canceled, scraper.FailureTransport},
		{"deadline exceeded", context.DeadlineExceeded, scraper.FailureTransport},
		{"net error", &net.DNSError{Err: "no such host"}, scraper.FailureTransport},

		// Any type may opt in by implementing FailureKind() — this is how an
		// HTTP status reaches the classifier without an import cycle.
		{"self-classifying error", kindedStub{scraper.FailureAbsent}, scraper.FailureAbsent},

		// Wrapping must not lose the annotation: scrapers routinely add
		// context with %w before the error reaches the channel.
		{
			"wrapped with fmt.Errorf",
			fmt.Errorf("page 3: %w", scraper.ParseError("u", errors.New("boom"))),
			scraper.FailureParse,
		},
		// httpx joins one error per retry attempt; errors.As walks the tree,
		// so a classified attempt anywhere in it classifies the whole failure.
		{
			"joined attempts",
			errors.Join(errors.New("attempt 1"), scraper.AbsentError("u", errors.New("404"))),
			scraper.FailureAbsent,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scraper.Classify(c.err); got != c.want {
				t.Errorf("Classify(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// Only an absent resource leaves the traversal complete. Unknown counting as
// missing is what keeps unclassified errors behaving as they always have.
func TestMissingData(t *testing.T) {
	cases := map[scraper.FailureKind]bool{
		scraper.FailureUnknown:   true,
		scraper.FailureTransport: true,
		scraper.FailureParse:     true,
		scraper.FailureAbsent:    false,
	}
	for kind, want := range cases {
		if got := kind.MissingData(); got != want {
			t.Errorf("%v.MissingData() = %v, want %v", kind, got, want)
		}
	}
}

// The annotation is additive — errors.Is must still reach the cause, or adding
// a kind would break every caller that tests for a sentinel.
func TestScrapeErrorUnwraps(t *testing.T) {
	cause := errors.New("cause")
	err := scraper.ParseError("https://example.com/p/2", cause)

	if !errors.Is(err, cause) {
		t.Errorf("errors.Is could not reach the cause through %T", err)
	}

	var se *scraper.ScrapeError
	if !errors.As(err, &se) {
		t.Fatalf("errors.As did not match *ScrapeError")
	}
	if se.URL != "https://example.com/p/2" {
		t.Errorf("URL = %q, want the page URL", se.URL)
	}

	// The message has to name the URL: a warning line saying only "parse
	// failed" sends the maintainer looking through every page of the site.
	msg := err.Error()
	for _, want := range []string{"parse", "example.com/p/2", "cause"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}
}

func TestScrapeErrorWithoutURL(t *testing.T) {
	err := scraper.TransportError("", errors.New("cause"))
	if msg := err.Error(); msg != "transport: cause" {
		t.Errorf("Error() = %q, want %q", msg, "transport: cause")
	}
}
