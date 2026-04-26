package stash

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateCoverURL_publicURLsAccepted(t *testing.T) {
	// Use IP literals to avoid DNS lookups in the test environment.
	cases := []string{
		"https://1.1.1.1/cover.jpg",
		"http://1.1.1.1/cover.jpg",
		"https://8.8.8.8/path/to/file.jpg?token=abc",
		"http://[2606:4700:4700::1111]/cover.jpg",
	}
	for _, c := range cases {
		if err := validateCoverURL(c, false); err != nil {
			t.Errorf("validateCoverURL(%q) rejected public URL: %v", c, err)
		}
	}
}

func TestValidateCoverURL_rejectsBadScheme(t *testing.T) {
	cases := []string{
		"file:///etc/passwd",
		"gopher://example.com/",
		"ftp://example.com/file.jpg",
		"javascript:alert(1)",
		"data:image/png;base64,xxx",
	}
	for _, c := range cases {
		if err := validateCoverURL(c, false); err == nil {
			t.Errorf("validateCoverURL(%q) should have rejected non-http scheme", c)
		}
	}
}

func TestValidateCoverURL_rejectsPrivateAndLocalIPs(t *testing.T) {
	cases := []string{
		"http://127.0.0.1/foo",
		"http://127.0.0.1:9999/graphql",
		"http://localhost/admin", // resolves to loopback
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://172.16.0.1/",
		"http://169.254.169.254/latest/meta-data/", // cloud metadata
		"http://0.0.0.0/",
		"http://[::1]/",     // ipv6 loopback
		"http://[fe80::1]/", // ipv6 link-local
	}
	for _, c := range cases {
		if err := validateCoverURL(c, false); err == nil {
			t.Errorf("validateCoverURL(%q) should have rejected private/local IP", c)
		}
	}
}

func TestValidateCoverURL_allowPrivateNetworksBypass(t *testing.T) {
	cases := []string{
		"http://127.0.0.1/foo",
		"http://192.168.1.1/cover.jpg",
		"http://localhost/x",
	}
	for _, c := range cases {
		if err := validateCoverURL(c, true); err != nil {
			t.Errorf("validateCoverURL(%q, allowPrivate=true) should have accepted: %v", c, err)
		}
	}
}

func TestValidateCoverURL_rejectsMalformed(t *testing.T) {
	cases := []string{
		"://no-scheme",
		"http://",  // no host
		"https://", // no host
		"not a url at all",
	}
	for _, c := range cases {
		if err := validateCoverURL(c, false); err == nil {
			t.Errorf("validateCoverURL(%q) should have rejected malformed URL", c)
		}
	}
}

func TestDownloadCoverImage_rejectsLoopbackByDefault(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("img"))
	}))
	defer ts.Close()

	c := &Client{http: ts.Client()}
	_, err := c.DownloadCoverImage(context.Background(), ts.URL+"/cover.jpg", false)
	if err == nil {
		t.Fatal("expected loopback URL to be rejected")
	}
	if !strings.Contains(err.Error(), "private/loopback") {
		t.Errorf("error should mention private/loopback, got: %v", err)
	}
}

func TestDownloadCoverImage_succeedsWithAllowPrivate(t *testing.T) {
	payload := []byte("\x89PNG\r\n\x1a\nfake-png-bytes")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer ts.Close()

	c := &Client{http: ts.Client()}
	got, err := c.DownloadCoverImage(context.Background(), ts.URL+"/cover.png", true)
	if err != nil {
		t.Fatalf("DownloadCoverImage: %v", err)
	}
	wantPrefix := "data:image/png;base64,"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("missing data URL prefix: %s", got)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, wantPrefix))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Errorf("decoded payload mismatch")
	}
}

func TestDownloadCoverImage_sizeCap(t *testing.T) {
	// Serve one byte over the cap to trigger the limit check.
	oversized := bytes.Repeat([]byte("a"), MaxCoverImageBytes+1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(oversized)
	}))
	defer ts.Close()

	c := &Client{http: ts.Client()}
	_, err := c.DownloadCoverImage(context.Background(), ts.URL+"/big.jpg", true)
	if err == nil {
		t.Fatal("expected size cap error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should mention size cap, got: %v", err)
	}
}

func TestDownloadCoverImage_detectsContentTypeWhenMissing(t *testing.T) {
	pngHeader := []byte("\x89PNG\r\n\x1a\n")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// No Content-Type header set; trigger http.DetectContentType.
		w.Header()["Content-Type"] = nil
		_, _ = w.Write(pngHeader)
	}))
	defer ts.Close()

	c := &Client{http: ts.Client()}
	got, err := c.DownloadCoverImage(context.Background(), ts.URL, true)
	if err != nil {
		t.Fatalf("DownloadCoverImage: %v", err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Errorf("expected detected image/png, got prefix of: %s", got[:40])
	}
}
