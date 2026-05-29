package parseutil

import "testing"

func TestOpenGraph_basic(t *testing.T) {
	body := []byte(`<html><head>
<meta property="og:title" content="The Title">
<meta property="og:description" content="A description.">
<meta property="og:image" content="https://example.com/cover.jpg">
<meta property="og:video:duration" content="1234">
</head></html>`)

	og := OpenGraph(body)
	want := map[string]string{
		"og:title":          "The Title",
		"og:description":    "A description.",
		"og:image":          "https://example.com/cover.jpg",
		"og:video:duration": "1234",
	}
	for k, v := range want {
		if got := og[k]; got != v {
			t.Errorf("og[%q] = %q, want %q", k, got, v)
		}
	}
}

func TestOpenGraph_reverseAttributeOrder(t *testing.T) {
	body := []byte(`<meta content="Reverse" property="og:title">`)
	og := OpenGraph(body)
	if og["og:title"] != "Reverse" {
		t.Errorf("og[og:title] = %q, want %q", og["og:title"], "Reverse")
	}
}

func TestOpenGraph_caseInsensitiveTag(t *testing.T) {
	body := []byte(`<META Property="og:title" Content="Mixed">`)
	og := OpenGraph(body)
	if og["og:title"] != "Mixed" {
		t.Errorf("og[og:title] = %q, want %q", og["og:title"], "Mixed")
	}
}

func TestOpenGraph_duplicateLastWins(t *testing.T) {
	body := []byte(`<meta property="og:image" content="first">
<meta property="og:image" content="second">`)
	og := OpenGraph(body)
	if og["og:image"] != "second" {
		t.Errorf("og[og:image] = %q, want %q (last-wins)", og["og:image"], "second")
	}
}

func TestOpenGraph_noTags(t *testing.T) {
	og := OpenGraph([]byte(`<html><head><title>plain</title></head></html>`))
	if og == nil {
		t.Error("OpenGraph must return non-nil map even when empty")
	}
	if len(og) != 0 {
		t.Errorf("expected empty map, got %v", og)
	}
}

func TestOpenGraph_rawEntitiesPreserved(t *testing.T) {
	// The helper does NOT decode HTML entities — callers handle that.
	body := []byte(`<meta property="og:title" content="A &amp; B">`)
	og := OpenGraph(body)
	if og["og:title"] != "A &amp; B" {
		t.Errorf("og[og:title] = %q, want %q (raw)", og["og:title"], "A &amp; B")
	}
}

func TestOpenGraph_ignoresNonOgMeta(t *testing.T) {
	body := []byte(`<meta name="description" content="not og">
<meta property="twitter:card" content="summary">
<meta property="og:title" content="yes og">`)
	og := OpenGraph(body)
	if len(og) != 1 {
		t.Errorf("expected exactly 1 og: tag, got %v", og)
	}
	if og["og:title"] != "yes og" {
		t.Errorf("og[og:title] = %q", og["og:title"])
	}
}
