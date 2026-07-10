package footfetishdaily

import (
	"testing"
)

const listingFixture = `
<div class="container">
  <div class="row">
    <div class="col-md-3">
      <div class="carousel-cell">
        <a href="/update/788/Alyssa_Reece_Sugar_Toes_Remastered">
          <img class="img-fluid img-thumbnail content-img" src="https://cdn01.kickass.com/subscriber/ffd_update_thumbs/788_b.jpg?ttl=1&ip=1&token=1" alt="Alyssa Reece Sugar Toes Remastered" loading="lazy">
        </a>
      </div>
      <h2 class="text-fred-font"><a href="/update/788/Alyssa_Reece_Sugar_Toes_Remastered">Alyssa Reece Sugar Toes Remastered</a></h2>
    </div>
    <div class="col-md-3">
      <div class="carousel-cell">
        <a href="/update/5656/Anetta_Moor_Lollipop_Toes">
          <img class="img-fluid img-thumbnail content-img" src="https://cdn01.kickass.com/subscriber/ffd_update_thumbs/5656_b.jpg" alt="Anetta Moor Lollipop Toes" loading="lazy">
        </a>
      </div>
      <h2 class="text-fred-font"><a href="/update/5656/Anetta_Moor_Lollipop_Toes">Anetta Moor Lollipop Toes</a></h2>
    </div>
  </div>
</div>
`

func TestParseListing(t *testing.T) {
	scenes := parseListing([]byte(listingFixture))
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	first := scenes[0]
	if first.id != "788" {
		t.Errorf("id = %q, want 788", first.id)
	}
	if first.slug != "Alyssa_Reece_Sugar_Toes_Remastered" {
		t.Errorf("slug = %q", first.slug)
	}
	if first.url != "https://www.footfetishdaily.com/update/788/Alyssa_Reece_Sugar_Toes_Remastered" {
		t.Errorf("url = %q", first.url)
	}

	if scenes[1].id != "5656" {
		t.Errorf("second id = %q, want 5656", scenes[1].id)
	}
}

func TestParseListing_empty(t *testing.T) {
	if got := parseListing([]byte(`<div class="row"></div>`)); len(got) != 0 {
		t.Fatalf("got %d scenes, want 0", len(got))
	}
}

const detailFixture = `
<!DOCTYPE html>
<html>
<head>
<meta property="og:title" content="Anetta Moor Lollipop Toes">
<meta property="og:description" content="Anetta keeps the sweet tease going...">
<meta property="og:url" content="https://www.footfetishdaily.com/update/5656/Anetta_Moor_Lollipop_Toes">
<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Organization","name":"Foot Fetish Daily","url":"https://www.footfetishdaily.com"}
</script>
<script type="application/ld+json">
{
    "@context": "https://schema.org/",
    "@type": "VideoObject",
    "name": "Anetta Moor Lollipop Toes",
    "@id": "https://www.footfetishdaily.com/update/5656/Anetta_Moor_Lollipop_Toes",
    "contentUrl": "https://delivery.kickass.com/subscriber/ffd_trailers/5656.mp4?expires=1&token=1",
    "datePublished": "2026-06-19T00:00:00-04:00",
    "uploadDate": "2026-06-19T00:00:00-04:00",
    "description": "Anetta keeps the sweet tease going, using sugary treats to draw your attention to her beautiful natural toes.",
    "thumbnailUrl": "https://delivery.kickass.com/subscriber/ffd_samples/5656/4.jpg?expires=1&token=1",
    "author": {"@type": "Organization", "name": "Kickass Pictures, inc"}
}
</script>
</head>
<body>
  <h1>Anetta Moor Lollipop Toes</h1>
  <p>Date: 2026-06-19</p>
  <div class="models">
    Starring: <a href="/model/1035/Anetta_Moor">Anetta Moor</a>
  </div>
</body>
</html>
`

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))

	if d.title != "Anetta Moor Lollipop Toes" {
		t.Errorf("title = %q", d.title)
	}
	if d.description != "Anetta keeps the sweet tease going, using sugary treats to draw your attention to her beautiful natural toes." {
		t.Errorf("description = %q", d.description)
	}
	if d.thumbnail != "https://delivery.kickass.com/subscriber/ffd_samples/5656/4.jpg?expires=1&token=1" {
		t.Errorf("thumbnail = %q", d.thumbnail)
	}
	if d.date.IsZero() {
		t.Fatal("date is zero")
	}
	if y, m, day := d.date.Date(); y != 2026 || m != 6 || day != 19 {
		t.Errorf("date = %v, want 2026-06-19", d.date)
	}
	// -04:00 means 00:00 local == 04:00 UTC.
	if d.date.Hour() != 4 {
		t.Errorf("UTC hour = %d, want 4", d.date.Hour())
	}
	if len(d.performers) != 1 || d.performers[0] != "Anetta Moor" {
		t.Errorf("performers = %v, want [Anetta Moor]", d.performers)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://www.footfetishdaily.com":                                  true,
		"https://www.footfetishdaily.com/videos":                           true,
		"https://footfetishdaily.com/videos/2":                             true,
		"https://www.footfetishdaily.com/update/5656/Anetta_Moor_Lollipop": true,
		"https://example.com/videos":                                       false,
		"https://footfetishdailyfake.com":                                  false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestSlugToTitle(t *testing.T) {
	if got := slugToTitle("Anetta_Moor_Lollipop_Toes"); got != "Anetta Moor Lollipop Toes" {
		t.Errorf("got %q", got)
	}
}
