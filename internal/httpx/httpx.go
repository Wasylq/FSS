// Package httpx provides a shared HTTP client and a retry helper used by every
// scraper. Keeping this in one place means retry/backoff, timeout, and
// connection-pool tuning live in exactly one file.
package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Common User-Agent strings. Bump these in one place when sites start
// rejecting the version we impersonate.
const (
	UserAgentFirefox = "Mozilla/5.0 (X11; Linux x86_64; rv:124.0) Gecko/20100101 Firefox/124.0"
	UserAgentChrome  = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// sharedTransport is reused across all scrapers so TCP/TLS connections are
// pooled per host instead of being re-established on every request.
var sharedTransport http.RoundTripper = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
	ForceAttemptHTTP2:   true,
}

// NewClient returns an http.Client backed by the shared transport.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: sharedTransport,
	}
}

// Request describes a single HTTP call. Method defaults to GET (or POST if
// Body is non-nil). MaxAttempts defaults to 3.
type Request struct {
	Method      string
	URL         string
	Body        []byte
	Headers     map[string]string
	MaxAttempts int
}

// StatusError is returned when the server replies with a non-2xx status that
// we do not retry. The body has already been closed.
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string { return fmt.Sprintf("HTTP %d", e.StatusCode) }

// Do performs the request with exponential backoff: it retries network errors,
// 429, and 5xx up to MaxAttempts times, sleeping (attempt * 2s) between tries.
// Non-retryable 4xx responses fail fast with a *StatusError — the caller does
// not have to guard against decoding an error page as a successful body.
func Do(ctx context.Context, client *http.Client, r Request) (*http.Response, error) {
	method := r.Method
	if method == "" {
		if r.Body != nil {
			method = http.MethodPost
		} else {
			method = http.MethodGet
		}
	}
	maxAttempts := r.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		var body io.Reader
		if r.Body != nil {
			body = bytes.NewReader(r.Body)
		}
		req, err := http.NewRequestWithContext(ctx, method, r.URL, body)
		if err != nil {
			return nil, err
		}
		for k, v := range r.Headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = &StatusError{StatusCode: resp.StatusCode}
			continue
		}
		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			return nil, &StatusError{StatusCode: resp.StatusCode}
		}
		return resp, nil
	}
	return nil, lastErr
}
