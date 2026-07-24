// Package httpx provides a shared HTTP client and a retry helper used by every
// scraper. Keeping this in one place means retry/backoff, timeout, and
// connection-pool tuning live in exactly one file.
package httpx

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wasylq/FSS/scraper"
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
	return ReadBodyN(body, MaxPageBytes)
}

// ReadBodyN reads an HTTP response body up to maxBytes.
func ReadBodyN(body io.ReadCloser, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds %d bytes", maxBytes)
	}
	return data, nil
}

// DecodeJSON JSON-decodes from r into v, reading at most MaxPageBytes.
func DecodeJSON(r io.Reader, v any) error {
	return DecodeJSONN(r, v, MaxPageBytes)
}

// DecodeJSONN JSON-decodes from r into v, reading at most maxBytes.
//
// Reads one byte past the limit so an oversized body is reported as such
// rather than surfacing as a confusing "unexpected EOF" from the decoder.
// A body that decodes successfully within the limit is fine even if bytes
// remain unread, so the size is only reported when decoding actually failed.
func DecodeJSONN(r io.Reader, v any, maxBytes int64) error {
	c := &countingReader{r: io.LimitReader(r, maxBytes+1)}
	if err := json.NewDecoder(c).Decode(v); err != nil {
		if c.n > maxBytes {
			return fmt.Errorf("response body exceeds %d bytes", maxBytes)
		}
		return err
	}
	return nil
}

// countingReader tracks how many bytes were consumed, so DecodeJSONN can tell
// a malformed body apart from one truncated by the limit.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// maxDrainBytes caps how much of a discarded error body we read back. Draining
// lets net/http return the connection to the pool instead of tearing it down —
// which matters most on the retry path, where the next attempt goes to the same
// host — but an error page can be arbitrarily large, so the drain is bounded.
const maxDrainBytes = 64 * 1024

// drainAndClose discards a bounded amount of an unwanted response body before
// closing, so the underlying connection stays reusable.
func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
	_ = resp.Body.Close()
}

type reqIDKey struct{}

var reqIDCounter atomic.Uint64

// WithRequestID attaches an auto-incrementing request ID to the context.
// The ID appears in level-2 debug logs for correlation.
func WithRequestID(ctx context.Context) context.Context {
	id := reqIDCounter.Add(1)
	return context.WithValue(ctx, reqIDKey{}, id)
}

// RequestID returns the request ID from the context, or 0 if none is set.
func RequestID(ctx context.Context) uint64 {
	id, _ := ctx.Value(reqIDKey{}).(uint64)
	return id
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

// legacyTLSTransport is a separate pool for hosts whose TLS stack predates
// Go's default cipher list. See NewLegacyTLSClient.
var legacyTLSTransport http.RoundTripper = &http.Transport{
	MaxIdleConns:        100,
	MaxIdleConnsPerHost: 10,
	IdleConnTimeout:     90 * time.Second,
	ForceAttemptHTTP2:   true,
	TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
		// Go's default suite list is ECDHE-only. A few old servers offer
		// nothing Go will negotiate — DHE, which crypto/tls does not implement
		// at all, plus static-RSA key exchange, which it implements but no
		// longer offers by default. Naming the RSA suites explicitly is the
		// only way to reach those hosts from Go.
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		},
	},
}

// NewLegacyTLSClient returns a client for hosts that cannot complete a
// handshake with Go's default cipher list. Certificates are still verified —
// this widens the accepted key exchange, it does not disable checks. The
// negotiated suites lack forward secrecy, so use it only where a scraper has
// been shown to fail otherwise, and only for reading public metadata.
func NewLegacyTLSClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: legacyTLSTransport,
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
	// Note: there is deliberately no per-request body limit here. Do returns
	// the response without reading the body, so it could not enforce one;
	// bounding the read is the caller's job via ReadBody/ReadBodyN or
	// DecodeJSON/DecodeJSONN.
	BackoffSleep func(ctx context.Context, d time.Duration) error // nil uses default (real sleep with ctx)
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
	return doInner(ctx, client, r, true)
}

// DoWithStatus is like Do but passes any HTTP status (including 4xx and 5xx)
// through to the caller without classifying — useful for endpoints that
// legitimately return non-2xx with a meaningful body (e.g. SexMex's CMS
// returns HTTP 500 + valid HTML on model pages). Network errors are still
// retried with the same backoff as Do, but 429/5xx are NOT retried — the
// caller asked for the raw response and presumably wants to act on it.
// Default for `Do` (4xx fail-fast, 429/5xx retried) is the safer choice for
// everything else.
func DoWithStatus(ctx context.Context, client *http.Client, r Request) (*http.Response, error) {
	return doInner(ctx, client, r, false)
}

// redactURL strips query parameters from a URL for debug logging so
// query-string credentials (e.g. signed CDN URLs) don't leak to stderr.
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.RawQuery == "" {
		return rawURL
	}
	u.RawQuery = "…"
	return u.String()
}

// jitter applies ±25% randomness to a duration to prevent retry lockstep.
func jitter(d time.Duration) time.Duration {
	factor := 0.75 + rand.Float64()*0.5 // [0.75, 1.25)
	return time.Duration(float64(d) * factor)
}

func defaultBackoffSleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// doInner is the shared retry + send loop. `classifyStatus` toggles the
// status-code policy: true → 4xx fail-fast with *StatusError + retry 429/5xx
// (Do's contract); false → return any HTTP response as-is and retry only
// network errors (DoWithStatus's contract).
func doInner(ctx context.Context, client *http.Client, r Request, classifyStatus bool) (*http.Response, error) {
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
	sleep := r.BackoffSleep
	if sleep == nil {
		sleep = defaultBackoffSleep
	}

	// Collect every retry attempt's error so a flaky-network failure mode
	// shows the full chronology, not just the last error. Joined via
	// errors.Join at the end (or on ctx cancellation mid-backoff).
	var attemptErrs []error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleep(ctx, jitter(time.Duration(attempt)*2*time.Second)); err != nil {
				attemptErrs = append(attemptErrs, err)
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

		rid := RequestID(ctx)
		redacted := redactURL(r.URL)
		if rid > 0 {
			scraper.Debugf(2, "[r%d] %s %s", rid, method, redacted)
		} else {
			scraper.Debugf(2, "%s %s", method, redacted)
		}

		resp, err := client.Do(req)
		if err != nil {
			scraper.Debugf(2, "  error: %v", err)
			attemptErrs = append(attemptErrs, fmt.Errorf("attempt %d: %w", attempt+1, err))
			continue
		}

		if rid > 0 {
			scraper.Debugf(2, "  [r%d] %d %s (content-length: %d)", rid, resp.StatusCode, resp.Status, resp.ContentLength)
		} else {
			scraper.Debugf(2, "  %d %s (content-length: %d)", resp.StatusCode, resp.Status, resp.ContentLength)
		}

		if !classifyStatus {
			return resp, nil
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			drainAndClose(resp)
			attemptErrs = append(attemptErrs, fmt.Errorf("attempt %d: %w", attempt+1, &StatusError{StatusCode: resp.StatusCode}))
			continue
		}
		if resp.StatusCode >= 400 {
			drainAndClose(resp)
			return nil, &StatusError{StatusCode: resp.StatusCode}
		}
		return resp, nil
	}
	return nil, errors.Join(attemptErrs...)
}
