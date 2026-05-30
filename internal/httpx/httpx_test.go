package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func noopSleep(_ context.Context, _ time.Duration) error { return nil }

func TestDo_succeedsFirstTry(t *testing.T) {
	t.Parallel()
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	resp, err := Do(context.Background(), ts.Client(), Request{URL: ts.URL})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q", body)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 call, got %d", got)
	}
}

func TestDo_retriesOn5xxThenSucceeds(t *testing.T) {
	t.Parallel()
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	resp, err := Do(context.Background(), ts.Client(), Request{URL: ts.URL, BackoffSleep: noopSleep})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 calls (1 fail + 1 success), got %d", got)
	}
}

func TestDo_failsFastOn4xx(t *testing.T) {
	t.Parallel()
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	_, err := Do(context.Background(), ts.Client(), Request{URL: ts.URL})
	if err == nil {
		t.Fatal("expected error")
	}

	var se *StatusError
	if !errors.As(err, &se) {
		t.Fatalf("expected *StatusError, got %T: %v", err, err)
	}
	if se.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", se.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("4xx must not retry, got %d calls", got)
	}
}

// TestDo_joinsErrorsViaCtxCancel — the core of the retry-exhaustion fix.
// We force one 5xx (which records the first attempt's StatusError), then
// cancel the context during backoff. The returned error must be a Join of
// both, so callers diagnosing flaky networks see the full chronology.
func TestDo_joinsErrorsViaCtxCancel(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	_, err := Do(ctx, ts.Client(), Request{
		URL:         ts.URL,
		MaxAttempts: 3,
		BackoffSleep: func(_ context.Context, _ time.Duration) error {
			cancel()
			return context.Canceled
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Both the StatusError and the context cancellation must be reachable
	// via the standard errors.As / errors.Is interface — that's what
	// errors.Join provides.
	var se *StatusError
	if !errors.As(err, &se) {
		t.Errorf("expected StatusError in chain, got: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled in chain, got: %v", err)
	}
	// And both should appear in the printed message.
	msg := err.Error()
	if !contains(msg, "HTTP 500") {
		t.Errorf("expected HTTP 500 in message: %s", msg)
	}
	if !contains(msg, "context canceled") {
		t.Errorf("expected context canceled in message: %s", msg)
	}
}

func TestDo_postWithBody(t *testing.T) {
	t.Parallel()
	var got []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		got, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte("ack"))
	}))
	defer ts.Close()

	resp, err := Do(context.Background(), ts.Client(), Request{
		URL:  ts.URL,
		Body: []byte(`{"hello":"world"}`),
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()

	if string(got) != `{"hello":"world"}` {
		t.Errorf("server received %q", got)
	}
}

func TestDo_setsCustomHeaders(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("missing custom header, got: %v", r.Header)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	resp, err := Do(context.Background(), ts.Client(), Request{
		URL:     ts.URL,
		Headers: map[string]string{"X-Custom": "value"},
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()
}

func TestDo_retriesOn429ThenSucceeds(t *testing.T) {
	t.Parallel()
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	resp, err := Do(context.Background(), ts.Client(), Request{URL: ts.URL, BackoffSleep: noopSleep})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q", body)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 calls (1 rate-limit + 1 success), got %d", got)
	}
}

func TestDo_exhaustsAllRetries(t *testing.T) {
	t.Parallel()
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	_, err := Do(context.Background(), ts.Client(), Request{URL: ts.URL, MaxAttempts: 2, BackoffSleep: noopSleep})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 calls, got %d", got)
	}
	var se *StatusError
	if !errors.As(err, &se) {
		t.Errorf("expected StatusError in chain, got: %v", err)
	}
}

// ---- DoWithStatus ----

func TestDoWithStatus_passes500Through(t *testing.T) {
	t.Parallel()
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("page body"))
	}))
	defer ts.Close()

	resp, err := DoWithStatus(context.Background(), ts.Client(), Request{URL: ts.URL})
	if err != nil {
		t.Fatalf("DoWithStatus returned err: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "page body" {
		t.Errorf("body = %q, want %q", string(body), "page body")
	}
	// 500 must NOT trigger retry — DoWithStatus delegates classification.
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 call (no retry), got %d", got)
	}
}

func TestDoWithStatus_passes4xxThrough(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	resp, err := DoWithStatus(context.Background(), ts.Client(), Request{URL: ts.URL})
	if err != nil {
		t.Fatalf("DoWithStatus on 403 returned err: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want 403", resp.StatusCode)
	}
}

func TestDoWithStatus_succeedsOn2xx(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	resp, err := DoWithStatus(context.Background(), ts.Client(), Request{URL: ts.URL})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestDoWithStatus_retriesNetworkErrorThenSucceeds(t *testing.T) {
	t.Parallel()
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			// Hijack and drop the connection to simulate a transport error.
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	resp, err := DoWithStatus(context.Background(), ts.Client(), Request{URL: ts.URL, MaxAttempts: 2, BackoffSleep: noopSleep})
	if err != nil {
		t.Fatalf("DoWithStatus should have retried network error and succeeded: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("expected 2 calls (1 dropped + 1 success), got %d", got)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestReadBody_underLimit(t *testing.T) {
	t.Parallel()
	body := io.NopCloser(strings.NewReader("hello world"))
	data, err := ReadBody(body)
	if err != nil {
		t.Fatalf("ReadBody: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q", data)
	}
}

func TestReadBody_atExactLimit(t *testing.T) {
	t.Parallel()
	payload := strings.Repeat("x", MaxPageBytes)
	body := io.NopCloser(strings.NewReader(payload))
	data, err := ReadBody(body)
	if err != nil {
		t.Fatalf("ReadBody: %v", err)
	}
	if len(data) != MaxPageBytes {
		t.Errorf("got %d bytes, want %d", len(data), MaxPageBytes)
	}
}

func TestReadBody_overLimit(t *testing.T) {
	t.Parallel()
	payload := strings.Repeat("x", MaxPageBytes+1)
	body := io.NopCloser(strings.NewReader(payload))
	_, err := ReadBody(body)
	if err == nil {
		t.Fatal("expected error for oversized body")
	}
}

func TestDecodeJSON(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(`{"name":"test","value":42}`)
	var v struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	if err := DecodeJSON(r, &v); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if v.Name != "test" || v.Value != 42 {
		t.Errorf("got %+v", v)
	}
}

func TestResolveUA(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"firefox", builtinFirefox},
		{"Firefox", builtinFirefox},
		{"  firefox  ", builtinFirefox},
		{"chrome", builtinChrome},
		{"Chrome", builtinChrome},
		{"Mozilla/5.0 Custom", "Mozilla/5.0 Custom"},
		{"", ""},
	}
	for _, c := range cases {
		if got := ResolveUA(c.in); got != c.want {
			t.Errorf("ResolveUA(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBrowserHeaders(t *testing.T) {
	t.Parallel()

	h := BrowserHeaders("")
	if h["User-Agent"] != UserAgentFirefox {
		t.Errorf("empty UA: got %q, want default Firefox", h["User-Agent"])
	}

	h = BrowserHeaders("custom-ua")
	if h["User-Agent"] != "custom-ua" {
		t.Errorf("custom UA: got %q, want %q", h["User-Agent"], "custom-ua")
	}

	for _, key := range []string{"Accept", "Accept-Language", "Sec-Fetch-Dest", "Sec-Fetch-Mode", "Sec-Fetch-Site", "Sec-Fetch-User"} {
		if h[key] == "" {
			t.Errorf("missing header %q", key)
		}
	}
}

func TestSetDefaultUA(t *testing.T) {
	save := func() (string, string) { return UserAgentFirefox, UserAgentChrome }
	restore := func(ff, ch string) { UserAgentFirefox, UserAgentChrome = ff, ch }

	t.Run("empty is no-op", func(t *testing.T) {
		ff, ch := save()
		defer restore(ff, ch)
		SetDefaultUA("")
		if UserAgentFirefox != ff || UserAgentChrome != ch {
			t.Error("empty string should not change UA vars")
		}
	})

	t.Run("shorthand chrome", func(t *testing.T) {
		ff, ch := save()
		defer restore(ff, ch)
		SetDefaultUA("chrome")
		if UserAgentFirefox != builtinChrome || UserAgentChrome != builtinChrome {
			t.Errorf("both vars should be Chrome builtin, got Firefox=%q Chrome=%q", UserAgentFirefox, UserAgentChrome)
		}
	})

	t.Run("custom string", func(t *testing.T) {
		ff, ch := save()
		defer restore(ff, ch)
		SetDefaultUA("MyBot/1.0")
		if UserAgentFirefox != "MyBot/1.0" || UserAgentChrome != "MyBot/1.0" {
			t.Errorf("both vars should be custom, got Firefox=%q Chrome=%q", UserAgentFirefox, UserAgentChrome)
		}
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		ff, ch := save()
		defer restore(ff, ch)
		SetDefaultUA("  firefox  ")
		if UserAgentFirefox != builtinFirefox {
			t.Errorf("got %q, want builtin Firefox", UserAgentFirefox)
		}
	})
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
