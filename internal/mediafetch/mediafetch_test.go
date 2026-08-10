package mediafetch

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValidateURL_publicURLsAccepted(t *testing.T) {
	// Use IP literals to avoid DNS lookups in the test environment.
	cases := []string{
		"https://1.1.1.1/cover.jpg",
		"http://1.1.1.1/cover.jpg",
		"https://8.8.8.8/path/to/file.jpg?token=abc",
		"http://[2606:4700:4700::1111]/cover.jpg",
	}
	for _, c := range cases {
		if err := ValidateURL(c, false); err != nil {
			t.Errorf("ValidateURL(%q) rejected public URL: %v", c, err)
		}
	}
}

func TestValidateURL_rejectsBadScheme(t *testing.T) {
	cases := []string{
		"file:///etc/passwd",
		"gopher://example.com/",
		"ftp://example.com/file.jpg",
		"javascript:alert(1)",
		"data:image/png;base64,xxx",
	}
	for _, c := range cases {
		if err := ValidateURL(c, false); err == nil {
			t.Errorf("ValidateURL(%q) should have rejected non-http scheme", c)
		}
	}
}

func TestValidateURL_rejectsPrivateAndLocalIPs(t *testing.T) {
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
		if err := ValidateURL(c, false); err == nil {
			t.Errorf("ValidateURL(%q) should have rejected private/local IP", c)
		}
	}
}

func TestValidateURL_allowPrivateNetworksBypass(t *testing.T) {
	cases := []string{
		"http://127.0.0.1/foo",
		"http://192.168.1.1/cover.jpg",
		"http://localhost/x",
	}
	for _, c := range cases {
		if err := ValidateURL(c, true); err != nil {
			t.Errorf("ValidateURL(%q, allowPrivate=true) should have accepted: %v", c, err)
		}
	}
}

func TestValidateURL_rejectsMalformed(t *testing.T) {
	cases := []string{
		"://no-scheme",
		"http://",  // no host
		"https://", // no host
		"not a url at all",
	}
	for _, c := range cases {
		if err := ValidateURL(c, false); err == nil {
			t.Errorf("ValidateURL(%q) should have rejected malformed URL", c)
		}
	}
}

func TestExpired(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour).Unix()
	future := now.Add(24 * time.Hour).Unix()

	cases := []struct {
		name string
		url  string
		want bool
	}{
		// The real shape seen in a pervcity studio file.
		{"mjedge expired", "https://c758cac692.mjedge.net/a.jpg?expires=" + itoa(past) + "&l=40&token=abc", true},
		{"mjedge still valid", "https://c758cac692.mjedge.net/a.jpg?expires=" + itoa(future) + "&token=abc", false},

		{"exp param", "https://cdn.example.com/a.jpg?exp=" + itoa(past), true},
		{"capitalised Expires", "https://cdn.example.com/a.jpg?Expires=" + itoa(past) + "&Signature=x", true},
		{"milliseconds", "https://cdn.example.com/a.jpg?expires=" + itoa(past*1000), true},

		// Nothing to go on: never claim expiry.
		{"no query", "https://cdn.example.com/a.jpg", false},
		{"unrelated numeric params", "https://cdn.example.com/a.jpg?w=1280&h=720&v=3", false},
		{"token but no expiry", "https://cdn.example.com/a.jpg?token=abc", false},
		{"non-numeric expiry", "https://cdn.example.com/a.jpg?expires=soon", false},
		{"implausible timestamp", "https://cdn.example.com/a.jpg?expires=42", false},
		{"malformed url", "://nope?expires=" + itoa(past), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Expired(c.url, now); got != c.want {
				t.Errorf("Expired(%q) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}

func TestExpiredAWSPresigned(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	base := "https://bucket.s3.amazonaws.com/a.jpg?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Date="

	// Signed 2 hours ago with a 1-hour lifetime: expired.
	signed := now.Add(-2 * time.Hour).Format("20060102T150405Z")
	if !Expired(base+signed+"&X-Amz-Expires=3600", now) {
		t.Error("expected an AWS presigned URL past its lifetime to be expired")
	}
	// Same signing time, 24-hour lifetime: still valid.
	if Expired(base+signed+"&X-Amz-Expires=86400", now) {
		t.Error("expected an AWS presigned URL within its lifetime to be valid")
	}
	// Missing lifetime: undecidable, so not expired.
	if Expired(base+signed, now) {
		t.Error("expected a URL with no X-Amz-Expires to be undecidable")
	}
}

func TestFetchReadsImage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpegbytes"))
	}))
	defer ts.Close()

	asset, err := Fetch(context.Background(), ts.Client(), ts.URL+"/a.jpg", true)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(asset.Data) != "jpegbytes" || asset.ContentType != "image/jpeg" {
		t.Errorf("got %+v", asset)
	}
}

func TestFetchRejectsOversized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, MaxBytes+1))
	}))
	defer ts.Close()

	if _, err := Fetch(context.Background(), ts.Client(), ts.URL+"/big.jpg", true); err == nil {
		t.Error("expected an oversized image to be rejected")
	}
}

// A dead signed URL must fail fast rather than retry.
func TestFetchGoneFailsFast(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusGone)
	}))
	defer ts.Close()

	if _, err := Fetch(context.Background(), ts.Client(), ts.URL+"/gone.jpg", true); err == nil {
		t.Fatal("expected HTTP 410 to be an error")
	}
	if hits != 1 {
		t.Errorf("made %d requests, want 1 (410 is not retryable)", hits)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestDataURIEncodesAsset(t *testing.T) {
	payload := []byte("\x89PNG\r\n\x1a\nfake-png-bytes")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(payload)
	}))
	defer ts.Close()

	got, err := DataURI(context.Background(), ts.Client(), ts.URL+"/cover.png", true)
	if err != nil {
		t.Fatalf("DataURI: %v", err)
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("missing data URI prefix: %s", got)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, prefix))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Error("decoded payload does not match what was served")
	}
}

func TestDataURIDetectsContentTypeWhenMissing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header()["Content-Type"] = nil
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
	defer ts.Close()

	got, err := DataURI(context.Background(), ts.Client(), ts.URL, true)
	if err != nil {
		t.Fatalf("DataURI: %v", err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Errorf("content type was not detected: %s", got)
	}
}

func TestDataURIRejectsLoopbackByDefault(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("img"))
	}))
	defer ts.Close()

	_, err := DataURI(context.Background(), ts.Client(), ts.URL+"/cover.jpg", false)
	if err == nil || !strings.Contains(err.Error(), "private/loopback") {
		t.Fatalf("want a private/loopback rejection, got: %v", err)
	}
}

// An expired signature is rejected before any request is made.
func TestDataURISkipsExpiredURL(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte("img"))
	}))
	defer ts.Close()

	url := ts.URL + "/cover.jpg?expires=1000000000"
	if _, err := DataURI(context.Background(), ts.Client(), url, true); err == nil {
		t.Fatal("want an error for an expired URL")
	}
	if hits != 0 {
		t.Errorf("made %d requests, want 0", hits)
	}
}
