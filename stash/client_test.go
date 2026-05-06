package stash

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
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

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	c := &Client{
		url:  ts.URL + "/graphql",
		http: ts.Client(),
	}
	return c, ts
}

func graphqlHandler(t *testing.T, responses map[string]string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		var req graphqlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
			http.Error(w, "bad request", 400)
			return
		}
		for key, resp := range responses {
			if strings.Contains(req.Query, key) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(resp))
				return
			}
		}
		t.Errorf("unmatched query: %s", req.Query)
		http.Error(w, "no match", 500)
	}
}

func TestFindScenes(t *testing.T) {
	c, _ := newTestClient(t, graphqlHandler(t, map[string]string{
		"findScenes": `{"data":{"findScenes":{"count":2,"scenes":[
			{"id":"1","title":"Scene One","files":[{"basename":"one.mp4","path":"/v/one.mp4","duration":120}],"tags":[],"performers":[],"stash_ids":[]},
			{"id":"2","title":"Scene Two","files":[],"tags":[{"id":"10","name":"anal"}],"performers":[{"id":"20","name":"Alice"}],"stash_ids":[]}
		]}}}`,
	}))

	scenes, count, err := c.FindScenes(context.Background(), FindScenesFilter{}, 1, 25)
	if err != nil {
		t.Fatalf("FindScenes: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}
	if scenes[0].Title != "Scene One" {
		t.Errorf("title = %q", scenes[0].Title)
	}
	if len(scenes[1].Tags) != 1 || scenes[1].Tags[0].Name != "anal" {
		t.Errorf("tags = %v", scenes[1].Tags)
	}
	if len(scenes[1].Performers) != 1 || scenes[1].Performers[0].Name != "Alice" {
		t.Errorf("performers = %v", scenes[1].Performers)
	}
}

func TestFindAllScenes(t *testing.T) {
	var calls int
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		// perPage is 100 in FindAllScenes. First page returns 100, second returns 1.
		if calls == 1 {
			scenes := make([]string, 100)
			for i := range scenes {
				scenes[i] = `{"id":"` + strings.Repeat("a", i+1) + `","title":"S","files":[],"tags":[],"performers":[],"stash_ids":[]}`
			}
			_, _ = w.Write([]byte(`{"data":{"findScenes":{"count":101,"scenes":[` + strings.Join(scenes, ",") + `]}}}`))
		} else {
			_, _ = w.Write([]byte(`{"data":{"findScenes":{"count":101,"scenes":[
				{"id":"last","title":"Last","files":[],"tags":[],"performers":[],"stash_ids":[]}
			]}}}`))
		}
	})

	var progressCalls int
	scenes, err := c.FindAllScenes(context.Background(), FindScenesFilter{}, func(fetched, total int) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("FindAllScenes: %v", err)
	}
	if len(scenes) != 101 {
		t.Errorf("got %d scenes, want 101", len(scenes))
	}
	if progressCalls != 2 {
		t.Errorf("progress called %d times, want 2", progressCalls)
	}
}

func TestFindAllScenes_cancellation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		scenes := make([]string, 100)
		for i := range scenes {
			scenes[i] = `{"id":"` + string(rune('0'+i%10)) + `","title":"S","files":[],"tags":[],"performers":[],"stash_ids":[]}`
		}
		_, _ = w.Write([]byte(`{"data":{"findScenes":{"count":500,"scenes":[` + strings.Join(scenes, ",") + `]}}}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.FindAllScenes(ctx, FindScenesFilter{}, nil)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

func TestEnsureTag_findsExisting(t *testing.T) {
	c, _ := newTestClient(t, graphqlHandler(t, map[string]string{
		"findTags": `{"data":{"findTags":{"tags":[{"id":"42","name":"blowjob"}]}}}`,
	}))

	id, err := c.EnsureTag(context.Background(), "blowjob")
	if err != nil {
		t.Fatal(err)
	}
	if id != "42" {
		t.Errorf("id = %q, want 42", id)
	}
}

func TestEnsureTag_createsWhenMissing(t *testing.T) {
	var calls int
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch {
		case strings.Contains(req.Query, "findTags"):
			_, _ = w.Write([]byte(`{"data":{"findTags":{"tags":[]}}}`))
		case strings.Contains(req.Query, "tagCreate"):
			_, _ = w.Write([]byte(`{"data":{"tagCreate":{"id":"99"}}}`))
		}
	})

	id, err := c.EnsureTag(context.Background(), "new-tag")
	if err != nil {
		t.Fatal(err)
	}
	if id != "99" {
		t.Errorf("id = %q, want 99", id)
	}
}

func TestEnsurePerformer_findsExisting(t *testing.T) {
	c, _ := newTestClient(t, graphqlHandler(t, map[string]string{
		"findPerformers": `{"data":{"findPerformers":{"performers":[{"id":"7","name":"Alice"}]}}}`,
	}))

	id, err := c.EnsurePerformer(context.Background(), "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if id != "7" {
		t.Errorf("id = %q, want 7", id)
	}
}

func TestEnsurePerformer_createsWhenMissing(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch {
		case strings.Contains(req.Query, "findPerformers"):
			_, _ = w.Write([]byte(`{"data":{"findPerformers":{"performers":[]}}}`))
		case strings.Contains(req.Query, "performerCreate"):
			_, _ = w.Write([]byte(`{"data":{"performerCreate":{"id":"88"}}}`))
		}
	})

	id, err := c.EnsurePerformer(context.Background(), "New Performer")
	if err != nil {
		t.Fatal(err)
	}
	if id != "88" {
		t.Errorf("id = %q, want 88", id)
	}
}

func TestEnsureStudio_findsExisting(t *testing.T) {
	c, _ := newTestClient(t, graphqlHandler(t, map[string]string{
		"findStudios": `{"data":{"findStudios":{"studios":[{"id":"5","name":"Brazzers"}]}}}`,
	}))

	id, err := c.EnsureStudio(context.Background(), "Brazzers")
	if err != nil {
		t.Fatal(err)
	}
	if id != "5" {
		t.Errorf("id = %q, want 5", id)
	}
}

func TestEnsureStudio_createsWhenMissing(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch {
		case strings.Contains(req.Query, "findStudios"):
			_, _ = w.Write([]byte(`{"data":{"findStudios":{"studios":[]}}}`))
		case strings.Contains(req.Query, "studioCreate"):
			_, _ = w.Write([]byte(`{"data":{"studioCreate":{"id":"77"}}}`))
		}
	})

	id, err := c.EnsureStudio(context.Background(), "New Studio")
	if err != nil {
		t.Fatal(err)
	}
	if id != "77" {
		t.Errorf("id = %q, want 77", id)
	}
}

func TestUpdateScene(t *testing.T) {
	c, _ := newTestClient(t, graphqlHandler(t, map[string]string{
		"sceneUpdate": `{"data":{"sceneUpdate":{"id":"1"}}}`,
	}))

	title := "Updated Title"
	err := c.UpdateScene(context.Background(), SceneUpdateInput{
		ID:    "1",
		Title: &title,
	})
	if err != nil {
		t.Fatalf("UpdateScene: %v", err)
	}
}

func TestUpdateScene_serverError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"scene not found"}]}`))
	})

	title := "X"
	err := c.UpdateScene(context.Background(), SceneUpdateInput{ID: "999", Title: &title})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "scene not found") {
		t.Errorf("error = %v", err)
	}
}

func TestPing(t *testing.T) {
	c, _ := newTestClient(t, graphqlHandler(t, map[string]string{
		"systemStatus": `{"data":{"systemStatus":{"status":"OK"}}}`,
	}))

	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestFindSceneByID(t *testing.T) {
	c, _ := newTestClient(t, graphqlHandler(t, map[string]string{
		"findScene": `{"data":{"findScene":{"id":"42","title":"Found Scene","files":[],"tags":[],"performers":[],"stash_ids":[]}}}`,
	}))

	scene, found, err := c.FindSceneByID(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if scene.Title != "Found Scene" {
		t.Errorf("title = %q", scene.Title)
	}
}

func TestFindSceneByID_notFound(t *testing.T) {
	c, _ := newTestClient(t, graphqlHandler(t, map[string]string{
		"findScene": `{"data":{"findScene":null}}`,
	}))

	_, found, err := c.FindSceneByID(context.Background(), "999")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected found=false")
	}
}

func TestClientSendsApiKeyHeader(t *testing.T) {
	var gotKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("ApiKey")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"systemStatus":{"status":"OK"}}}`))
	}))
	defer ts.Close()

	c := &Client{
		url:    ts.URL + "/graphql",
		apiKey: "test-key-123",
		http:   ts.Client(),
	}
	if err := c.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotKey != "test-key-123" {
		t.Errorf("ApiKey header = %q, want test-key-123", gotKey)
	}
}

// TestClientDoJoinsAndRedactsGraphQLErrors covers two related concerns:
// (1) all GraphQL error messages are joined into the returned error, not just
// the first one; (2) the configured API key is redacted from each message in
// case a misbehaving server ever echoes it back.
func TestClientDoJoinsAndRedactsGraphQLErrors(t *testing.T) {
	const apiKey = "supersecretkey-do-not-leak"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"bad request: token=` + apiKey + `"},{"message":"second failure"}]}`))
	}))
	defer ts.Close()

	c := &Client{
		url:    ts.URL,
		apiKey: apiKey,
		http:   ts.Client(),
	}
	_, err := c.do(context.Background(), graphqlRequest{Query: "{ x }"})
	if err == nil {
		t.Fatal("expected error from server-returned GraphQL errors")
	}
	msg := err.Error()
	if strings.Contains(msg, apiKey) {
		t.Errorf("API key leaked in error message: %s", msg)
	}
	if !strings.Contains(msg, "[redacted]") {
		t.Errorf("expected [redacted] marker, got: %s", msg)
	}
	if !strings.Contains(msg, "second failure") {
		t.Errorf("second error was not joined into result, got: %s", msg)
	}
}
