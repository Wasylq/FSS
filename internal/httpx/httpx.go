// Package httpx provides a shared HTTP client and a retry helper used by every
// scraper. Keeping this in one place means retry/backoff, timeout, and
// connection-pool tuning live in exactly one file.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	builtinFirefox = "Mozilla/5.0 (X11; Linux x86_64; rv:138.0) Gecko/20100101 Firefox/138.0"
	builtinChrome  = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
)

// UserAgentFirefox and UserAgentChrome are the active UA strings used by
// scrapers. Call SetDefaultUA at startup to override both with a config value.
var (
	UserAgentFirefox = builtinFirefox
	UserAgentChrome  = builtinChrome
)

// SetDefaultUA overrides both exported UA variables. Accepts "firefox",
// "chrome", or a full custom string. Empty is a no-op (keeps built-ins).
// Call once at startup before any scrapers run.
func SetDefaultUA(ua string) {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return
	}
	resolved := ResolveUA(ua)
	UserAgentFirefox = resolved
	UserAgentChrome = resolved
}

// ResolveUA maps a shorthand to a full UA string. "firefox" and "chrome"
// return the built-in strings; anything else is returned as-is.
func ResolveUA(ua string) string {
	switch strings.ToLower(strings.TrimSpace(ua)) {
	case "firefox":
		return builtinFirefox
	case "chrome":
		return builtinChrome
	default:
		return ua
	}
}

// BrowserHeaders returns headers that mimic a real browser navigation request,
// including Sec-Fetch-* headers that WAFs like Wordfence check. Pass a UA
// string, or empty to use the active default.
func BrowserHeaders(ua string) map[string]string {
	if ua == "" {
		ua = UserAgentFirefox
	}
	return map[string]string{
		"User-Agent":                ua,
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language":           "en-US,en;q=0.5",
		"DNT":                       "1",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
	}
}

// MaxPageBytes caps ReadBody response reads to prevent an oversized or
// malicious response from exhausting memory.
const MaxPageBytes = 10 * 1024 * 1024

// ReadBody reads an HTTP response body up to MaxPageBytes. Use this instead of
// io.ReadAll(resp.Body) in scrapers to bound memory usage.
func ReadBody(body io.ReadCloser) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, MaxPageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxPageBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", MaxPageBytes)
	}
	return data, nil
}

// DecodeJSON JSON-decodes from r into v, reading at most MaxPageBytes.
func DecodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(io.LimitReader(r, MaxPageBytes)).Decode(v)
}

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

	// Collect every retry attempt's error so a flaky-network failure mode
	// shows the full chronology, not just the last error. Joined via
	// errors.Join at the end (or on ctx cancellation mid-backoff).
	var attemptErrs []error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			case <-ctx.Done():
				attemptErrs = append(attemptErrs, ctx.Err())
				return nil, errors.Join(attemptErrs...)
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
			attemptErrs = append(attemptErrs, fmt.Errorf("attempt %d: %w", attempt+1, err))
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			attemptErrs = append(attemptErrs, fmt.Errorf("attempt %d: %w", attempt+1, &StatusError{StatusCode: resp.StatusCode}))
			continue
		}
		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			return nil, &StatusError{StatusCode: resp.StatusCode}
		}
		return resp, nil
	}
	return nil, errors.Join(attemptErrs...)
}
