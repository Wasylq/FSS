package httpx

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewBrowserTLSClientReusesOnePool(t *testing.T) {
	a := NewBrowserTLSClient(5 * time.Second)
	b := NewBrowserTLSClient(9 * time.Second)
	if a.Transport != b.Transport {
		t.Error("each call built a new transport; the pool should be shared")
	}
	if a.Timeout != 5*time.Second || b.Timeout != 9*time.Second {
		t.Errorf("timeouts = %v, %v; want 5s, 9s", a.Timeout, b.Timeout)
	}
}

// The point of the client is that it does NOT use the shared transport — one
// site needing a browser fingerprint must not change how every other site is
// reached.
func TestBrowserTLSPoolIsSeparateFromTheDefault(t *testing.T) {
	if NewBrowserTLSClient(time.Second).Transport == NewClient(time.Second).Transport {
		t.Error("browser client shares the default transport")
	}
	if NewBrowserTLSClient(time.Second).Transport == NewLegacyTLSClient(time.Second).Transport {
		t.Error("browser client shares the legacy-TLS transport")
	}
}

// It completes a real handshake, and verification is still on: the httptest
// server presents an untrusted certificate, so the request must fail on that
// rather than sail through.
func TestBrowserTLSVerifiesCertificates(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	resp, err := NewBrowserTLSClient(10 * time.Second).Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("request to a server with an untrusted certificate succeeded")
	}
	// utls wraps verification failures in its own error type rather than
	// crypto/tls's, so match on the x509 cause, which both share.
	var unknown x509.UnknownAuthorityError
	if !errors.As(err, &unknown) {
		t.Errorf("error was %v, want a certificate-verification failure", err)
	}
}

// The success path: a real handshake with a browser ClientHello, and the
// request completing over HTTP/2. This is what the transport exists to do, so
// it is worth proving offline rather than only against a live site.
func TestBrowserTLSCompletesRequestOverHTTP2(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto))
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	prev := browserRootCAs
	browserRootCAs = pool
	t.Cleanup(func() { browserRootCAs = prev })

	resp, err := NewBrowserTLSClient(15 * time.Second).Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if got := string(body); got != "HTTP/2.0" {
		t.Errorf("server saw %q, want HTTP/2.0 — the transport must not fall back", got)
	}
}

// A dial that cannot connect must surface as an error rather than a nil conn.
func TestDialBrowserTLSReportsDialFailure(t *testing.T) {
	// Port 1 on the loopback interface: nothing listens, connection refused.
	if _, err := dialBrowserTLS(context.Background(), "tcp", "127.0.0.1:1"); err == nil {
		t.Error("dial to a closed port succeeded")
	}
}

func TestDialBrowserTLSRejectsAnAddressWithNoPort(t *testing.T) {
	if _, err := dialBrowserTLS(context.Background(), "tcp", "127.0.0.1"); err == nil {
		t.Error("dial with a portless address succeeded")
	}
}
