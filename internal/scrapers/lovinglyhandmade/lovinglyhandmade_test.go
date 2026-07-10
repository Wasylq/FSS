package lovinglyhandmade

import (
	"testing"
)

const sitemapFixture = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url>
		<loc>https://lovinglyhandmadepornography.com/detail/taking-advantage-of-tenderness</loc>
	</url>
	<url>
		<loc>https://lovinglyhandmadepornography.com/detail/existence-proof</loc>
	</url>
	<url>
		<loc>https://lovinglyhandmadepornography.com/detail/taking-advantage-of-tenderness</loc>
	</url>
	<url>
		<loc>https://lovinglyhandmadepornography.com/detail/cow-doooooooom</loc>
	</url>
</urlset>`

// detailFixture mirrors the real /detail/{slug} markup (meta tags, the
// class-specific h1, the rel="tag" links mixing performers + content tags,
// and the data-full-duration attribute).
const detailFixture = `<!DOCTYPE html>
<html>
<head>
	<meta name="description" content="Autumn&rsquo;s various bits are very tender, and we both enjoy that.">
	<meta name="twitter:card" content="summary_large_image">
	<meta name="twitter:title" content="Taking Advantage of Tenderness | 2025-12-26">
	<meta name="twitter:description" content="Autumn&rsquo;s various bits are very tender, and we both enjoy that.">
	<meta name="twitter:image:src" content="https://r2media.lovinglyhandmadepornography.com/4vkl3ry5rs1tufg2ww6ot00r7hd7">
</head>
<body>
	<h1 align="center">Lovingly Handmade Pornography</h1>
	<h1 class="text-2xl font-bold mb-2">Taking Advantage of Tenderness</h1>
	<div data-full-duration="17" data-full-url="x">player</div>
	<a class="border" rel="tag" href="/tagged/autumn">Autumn</a>
	<a class="border text-[80%]" rel="tag" href="/tagged/dilemmas-conflicts">dilemmas &amp; conflicts</a>
	<a class="border text-[80%]" rel="tag" href="/tagged/orgasms">orgasms</a>
	<a class="border text-[80%]" rel="tag" href="/tagged/redheads">redheads</a>
	<a class="border text-[80%]" rel="tag" href="/tagged/1080p60">1080p60</a>
	<a class="border text-[80%]" rel="tag" href="/tagged/hevc">HEVC</a>
	<a class="border text-[80%]" rel="tag" href="/tagged/orgasms">orgasms</a>
</body>
</html>`

func TestParseSitemap(t *testing.T) {
	slugs := parseSitemap([]byte(sitemapFixture))
	want := []string{"taking-advantage-of-tenderness", "existence-proof", "cow-doooooooom"}
	if len(slugs) != len(want) {
		t.Fatalf("got %d slugs %v, want %d %v", len(slugs), slugs, len(want), want)
	}
	for i := range want {
		if slugs[i] != want[i] {
			t.Errorf("slug[%d] = %q, want %q", i, slugs[i], want[i])
		}
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))

	if d.title != "Taking Advantage of Tenderness" {
		t.Errorf("title = %q", d.title)
	}
	if got := d.date.Format("2006-01-02"); got != "2025-12-26" {
		t.Errorf("date = %q, want 2025-12-26", got)
	}
	if d.description != "Autumn’s various bits are very tender, and we both enjoy that." {
		t.Errorf("description = %q", d.description)
	}
	if d.thumbnail != "https://r2media.lovinglyhandmadepornography.com/4vkl3ry5rs1tufg2ww6ot00r7hd7" {
		t.Errorf("thumbnail = %q", d.thumbnail)
	}
	if d.duration != 17 {
		t.Errorf("duration = %d, want 17", d.duration)
	}

	wantTags := []string{"Autumn", "dilemmas & conflicts", "orgasms", "redheads", "1080p60", "HEVC"}
	if len(d.tags) != len(wantTags) {
		t.Fatalf("got %d tags %v, want %d %v", len(d.tags), d.tags, len(wantTags), wantTags)
	}
	for i := range wantTags {
		if d.tags[i] != wantTags[i] {
			t.Errorf("tag[%d] = %q, want %q", i, d.tags[i], wantTags[i])
		}
	}
}

func TestParseDetailTitleFallback(t *testing.T) {
	// No class-specific h1 -> fall back to twitter:title with date suffix stripped.
	body := `<meta name="twitter:title" content="Existence Proof | 2025-10-19">`
	d := parseDetail([]byte(body))
	if d.title != "Existence Proof" {
		t.Errorf("title fallback = %q, want %q", d.title, "Existence Proof")
	}
	if got := d.date.Format("2006-01-02"); got != "2025-10-19" {
		t.Errorf("date = %q, want 2025-10-19", got)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	good := []string{
		"https://lovinglyhandmadepornography.com/",
		"https://lovinglyhandmadepornography.com/detail/existence-proof",
		"http://www.lovinglyhandmadepornography.com",
	}
	for _, u := range good {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	bad := []string{
		"https://example.com/",
		"https://notlovinglyhandmadepornography.com/",
	}
	for _, u := range bad {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}
