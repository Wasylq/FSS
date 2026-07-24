package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// An oversized body must be reported as oversized. Before the +1 sentinel the
// decoder just saw a truncated stream and returned "unexpected EOF", which
// looks like a malformed response rather than a limit being hit.
func TestDecodeJSONNReportsOversizeRatherThanEOF(t *testing.T) {
	t.Parallel()

	big := `{"name":"` + strings.Repeat("x", 512) + `"}`
	var v struct {
		Name string `json:"name"`
	}

	err := DecodeJSONN(strings.NewReader(big), &v, 64)
	if err == nil {
		t.Fatal("expected an error for a body over the limit")
	}
	if !strings.Contains(err.Error(), "exceeds 64 bytes") {
		t.Errorf("error = %v, want it to name the byte limit", err)
	}
}

// A body that fits must decode normally.
func TestDecodeJSONNAcceptsBodyWithinLimit(t *testing.T) {
	t.Parallel()

	var v struct {
		Name string `json:"name"`
	}
	if err := DecodeJSONN(strings.NewReader(`{"name":"ok"}`), &v, 1024); err != nil {
		t.Fatalf("DecodeJSONN: %v", err)
	}
	if v.Name != "ok" {
		t.Errorf("Name = %q, want ok", v.Name)
	}
}

// Malformed JSON inside the limit must still surface as a JSON error, not be
// mislabelled as an oversize body.
func TestDecodeJSONNKeepsSyntaxErrors(t *testing.T) {
	t.Parallel()

	var v struct {
		Name string `json:"name"`
	}
	err := DecodeJSONN(strings.NewReader(`{"name":}`), &v, 1024)
	if err == nil {
		t.Fatal("expected a JSON syntax error")
	}
	if strings.Contains(err.Error(), "exceeds") {
		t.Errorf("syntax error was misreported as an oversize body: %v", err)
	}
}

// Retried responses must have their bodies drained so net/http can put the
// connection back in the pool. Without the drain, each retry burns a fresh
// connection.
func TestRetryReusesConnectionAfterServerError(t *testing.T) {
	t.Parallel()

	var conns atomic.Int64
	var attempts atomic.Int64

	// Unstarted, so ConnState can be installed before the server goroutine
	// starts reading it — setting it on a running httptest.Server is a data
	// race that only shows up under -race.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			// A sizeable error page: exactly what would otherwise be left
			// unread on the connection.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, strings.Repeat("error page ", 500))
			return
		}
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()

	client := srv.Client()
	resp, err := Do(t.Context(), client, Request{
		URL:          srv.URL,
		BackoffSleep: func(_ context.Context, _ time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`"ok":true`)) {
		t.Errorf("body = %s", body)
	}
	if attempts.Load() != 3 {
		t.Fatalf("server saw %d attempts, want 3", attempts.Load())
	}
	if got := conns.Load(); got != 1 {
		t.Errorf("server accepted %d connections across 3 attempts, want 1 — "+
			"retried bodies are not being drained, so the pooled connection is discarded", got)
	}
}
