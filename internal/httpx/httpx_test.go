package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

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

	resp, err := Do(context.Background(), ts.Client(), Request{URL: ts.URL})
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
	go func() {
		// Cancel after the first attempt + tiny window into the backoff.
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := Do(ctx, ts.Client(), Request{URL: ts.URL, MaxAttempts: 3})
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

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
