package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Anastylosis/FSS/scraper"
)

// NewRetryClient returns an http.Client that retries the way Do does — network
// errors, 429 and 5xx, with the same jittered backoff — for libraries that take
// an *http.Client and drive the request themselves.
//
// timeout bounds each attempt rather than the sequence, so a retried request
// gets the full budget every time. Non-retryable responses, 4xx included, come
// back untouched with their body intact.
func NewRetryClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &retryTransport{
			base:     sharedTransport,
			attempts: 3,
			timeout:  timeout,
		},
	}
}

type retryTransport struct {
	base     http.RoundTripper
	attempts int
	timeout  time.Duration
	sleep    func(ctx context.Context, d time.Duration) error // nil uses the default backoff
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	sleep := t.sleep
	if sleep == nil {
		sleep = defaultBackoffSleep
	}

	var attemptErrs []error
	for attempt := 0; attempt < t.attempts; attempt++ {
		if attempt > 0 {
			if err := sleep(req.Context(), jitter(time.Duration(attempt)*backoffBase)); err != nil {
				return nil, errors.Join(append(attemptErrs, err)...)
			}
		}

		body, err := replayBody(req, attempt)
		if err != nil {
			return nil, errors.Join(append(attemptErrs, err)...)
		}

		ctx, cancel := context.WithTimeout(req.Context(), t.timeout)
		attemptReq := req.Clone(ctx)
		attemptReq.Body = body

		resp, err := t.base.RoundTrip(attemptReq)
		if err != nil {
			cancel()
			scraper.Debugf(2, "  error: %v", err)
			attemptErrs = append(attemptErrs, fmt.Errorf("attempt %d: %w", attempt+1, err))
			continue
		}

		last := attempt == t.attempts-1
		if last || (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500) {
			resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
			return resp, nil
		}

		drainAndClose(resp)
		cancel()
		attemptErrs = append(attemptErrs, fmt.Errorf("attempt %d: %w", attempt+1, &StatusError{StatusCode: resp.StatusCode}))
	}
	return nil, errors.Join(attemptErrs...)
}

// replayBody returns the body to send. The first attempt consumes the caller's
// reader; later ones need GetBody, which http.NewRequest supplies for the
// in-memory bodies this is used with.
func replayBody(req *http.Request, attempt int) (io.ReadCloser, error) {
	if req.Body == nil {
		return nil, nil
	}
	if attempt == 0 {
		return req.Body, nil
	}
	if req.GetBody == nil {
		return nil, errors.New("request body cannot be replayed")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("rewinding request body: %w", err)
	}
	return body, nil
}

// cancelOnClose releases the attempt's context once the caller is done with the
// body — cancelling any earlier would cut the read short.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
