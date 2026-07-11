package stasyqvr

import "testing"

const listingFixture = `
<div class="main-part__item">
  <div class="img-loader-wrap">
    <img src="https://stasyqvr.com/storage/vr/vr-covers/cover475.webp" alt="Fit temptress President Mermaid poses naked" style="width:100%;">
    <video src="https://vcdn.example/previews/475/preview.mp4"></video>
    <a class="main-part__item__link" href="https://stasyqvr.com/virtualreality/scene/id/475"></a>
  </div>
</div>
<h2>Fit temptress President Mermaid poses naked <a class="main-part__item__link" href="https://stasyqvr.com/virtualreality/scene/id/475"></a></h2>
<div class="main-part__item">
  <div class="img-loader-wrap">
    <img src="https://stasyqvr.com/storage/vr/vr-covers/cover474.webp" alt="Another VR Scene">
    <a class="main-part__item__link" href="https://stasyqvr.com/virtualreality/scene/id/474"></a>
  </div>
</div>
`

const detailFixture = `
<div class="main-desc">
  <div class="main-desc__date"> Jun 25, 2026 </div>
  <div class="main-desc__detail">
    <div class="detail-right">
      <h1>Fit temptress President Mermaid poses naked</h1>
      <a class="downloads-signup" href="/user/join">Sign Up</a>
      <p>She is a goddess, and she knows it. Her name is President Mermaid.</p>
    </div>
  </div>
</div>
`

func TestParseListing(t *testing.T) {
	scenes := parseListing([]byte(listingFixture))
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes (deduped), got %d", len(scenes))
	}
	if scenes[0].id != "475" {
		t.Errorf("id = %q, want 475", scenes[0].id)
	}
	if scenes[0].url != "https://stasyqvr.com/virtualreality/scene/id/475" {
		t.Errorf("url = %q", scenes[0].url)
	}
	if scenes[0].title != "Fit temptress President Mermaid poses naked" {
		t.Errorf("title = %q", scenes[0].title)
	}
	if scenes[0].thumb != "https://stasyqvr.com/storage/vr/vr-covers/cover475.webp" {
		t.Errorf("thumb = %q", scenes[0].thumb)
	}
	if scenes[1].id != "474" {
		t.Errorf("second id = %q, want 474", scenes[1].id)
	}
}

func TestParseDetail(t *testing.T) {
	d := parseDetail([]byte(detailFixture))
	if d.title != "Fit temptress President Mermaid poses naked" {
		t.Errorf("title = %q", d.title)
	}
	if d.date.Format("2006-01-02") != "2026-06-25" {
		t.Errorf("date = %v, want 2026-06-25", d.date)
	}
	if d.description == "" || d.description[:10] != "She is a g" {
		t.Errorf("description = %q", d.description)
	}
}

func TestTokenRegex(t *testing.T) {
	page := []byte(`<form><input type="hidden" name="_token" value="5AbER2LOuxUrvdbl2wMSAXa9RSHADTMjXnnHHBhR"></form>`)
	m := tokenRe.FindSubmatch(page)
	if m == nil || string(m[1]) != "5AbER2LOuxUrvdbl2wMSAXa9RSHADTMjXnnHHBhR" {
		t.Errorf("token extraction failed: %v", m)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := map[string]bool{
		"https://stasyqvr.com/":                           true,
		"https://stasyqvr.com/virtualreality/list?page=2": true,
		"https://stasyq.com/":                             false,
		"https://example.com/stasyqvr":                    false,
	}
	for u, want := range cases {
		if got := s.MatchesURL(u); got != want {
			t.Errorf("MatchesURL(%q) = %v, want %v", u, got, want)
		}
	}
}
