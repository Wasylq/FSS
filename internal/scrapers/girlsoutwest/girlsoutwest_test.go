package girlsoutwest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wasylq/FSS/scraper"
)

func TestMatchesURL(t *testing.T) {
	s := New()
	cases := []struct {
		url  string
		want bool
	}{
		{"https://tour.girlsoutwest.com/", true},
		{"https://www.girlsoutwest.com/", true},
		{"https://girlsoutwest.com/", true},
		{"https://tour.girlsoutwest.com/categories/Movies.html", true},
		{"https://tour.girlsoutwest.com/models/Sage-Cherie.html", true},
		{"https://www.example.com/", false},
		{"https://girlsouteast.com/", false},
	}
	for _, c := range cases {
		if got := s.MatchesURL(c.url); got != c.want {
			t.Errorf("MatchesURL(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestExtractSlugID(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://tour.girlsoutwest.com/trailers/Sage-Cherie-Sensations.html", "Sage-Cherie-Sensations"},
		{"/trailers/Ruby-and-Anya-Touch.html", "Ruby-and-Anya-Touch"},
		{"/categories/Movies.html", ""},
		{"https://join2.girlsoutwest.com/signup/signup.php", ""},
	}
	for _, c := range cases {
		if got := extractSlugID(c.url); got != c.want {
			t.Errorf("extractSlugID(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
	}{
		{"Jun 8, 2026", time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)},
		{"June 8, 2026", time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)},
		{"Dec 25, 2024", time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)},
		{"", time.Time{}},
		{"invalid", time.Time{}},
	}
	for _, c := range cases {
		if got := parseDate(c.input); !got.Equal(c.want) {
			t.Errorf("parseDate(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

const listingHTML = `<html><body>
<div class="iLatestScene">
	<div class="iLScenePic item-thumb-videothumb">
		<div class="b10187_videothumb_123">
			<a href="https://tour.girlsoutwest.com/trailers/Sage-Cherie-Sensations.html"></a>
			<img src="/content//contentthumbs/05/39/200539-1x.jpg" alt="" class="video_placeholder" />
			<div class="video-progress"><div></div><div></div></div>
		</div>
	</div>
	<div class="iLSceneDetails">
		<h4><a href="https://tour.girlsoutwest.com/trailers/Sage-Cherie-Sensations.html" title="Sage Cherie - Sensations">Sage Cherie - Sensations</a></h4>
		<div class="featuring">Featuring: <a href="https://tour.girlsoutwest.com/models/Sage-Cherie.html">Sage Cherie</a></div>
		<ul class="sceneInfo">
			<li><i class="fas fa-clock"></i> 25:46</li>
			<li><i class="fas fa-calendar"></i> Jun 8, 2026</li>
			<li><i class="fas fa-comment"></i> 2</li>
			<li><span><i class="fas fa-star"></i> 8.57</span></li>
		</ul>
	</div>
</div>
<div class="iLatestScene">
	<div class="iLScenePic item-thumb-videothumb">
		<div class="b10187_videothumb_456">
			<a href="/trailers/Ruby-and-Anya-Touch.html"></a>
			<img src="/content//contentthumbs/01/22/190122-1x.jpg" alt="" class="video_placeholder" />
		</div>
	</div>
	<div class="iLSceneDetails">
		<h4><a href="/trailers/Ruby-and-Anya-Touch.html" title="Ruby &amp; Anya - Touch">Ruby &amp; Anya - Touch</a></h4>
		<div class="featuring">Featuring: <a href="/models/Ruby.html">Ruby</a>, <a href="/models/Anya.html">Anya</a></div>
		<ul class="sceneInfo">
			<li><i class="fas fa-clock"></i> 1:02:30</li>
			<li><i class="fas fa-calendar"></i> Jun 5, 2026</li>
			<li><i class="fas fa-comment"></i> 0</li>
			<li><span><i class="fas fa-star"></i> 9.00</span></li>
		</ul>
	</div>
</div>
<div class="iLatestScene">
	<div class="iLScenePic">
		<a href="https://join2.girlsoutwest.com/signup/signup.php?nats=&step=2" title="Sage Cherie - Silk Pics">
			<img src0_1x="/content//contentthumbs/03/77/200377-set-1x.jpg" />
		</a>
	</div>
	<div class="iLSceneDetails">
		<h4><a href="https://join2.girlsoutwest.com/signup/signup.php?nats=&step=2" title="Sage Cherie - Silk Pics">Sage Cherie - Silk Pics</a></h4>
		<div class="featuring">Featuring: <a href="/models/Sage-Cherie.html">Sage Cherie</a></div>
		<ul class="sceneInfo">
			<li><i class="fas fa-clock"></i> 45&nbsp;Photos</li>
			<li><i class="fas fa-calendar"></i> May 21, 2026</li>
		</ul>
	</div>
</div>
<div class="pagination shorterPadding">
	<a href="/categories/Movies.html" class="active">1</a>
	<a href="/categories/Movies_2_d.html">2</a>
	<a href="/categories/Movies_3_d.html" class="hideMobile">3</a>
	<span>...</span>
	<a href="/categories/Movies_257_d.html"><i class="fas fa-angle-double-right"></i></a>
</div>
</body></html>`

func TestParseListingPage(t *testing.T) {
	scenes := parseListingPage([]byte(listingHTML), "https://tour.girlsoutwest.com")
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2 (photo set should be filtered)", len(scenes))
	}

	s := scenes[0]
	if s.id != "Sage-Cherie-Sensations" {
		t.Errorf("id = %q, want Sage-Cherie-Sensations", s.id)
	}
	if s.title != "Sage Cherie - Sensations" {
		t.Errorf("title = %q, want 'Sage Cherie - Sensations'", s.title)
	}
	if s.url != "https://tour.girlsoutwest.com/trailers/Sage-Cherie-Sensations.html" {
		t.Errorf("url = %q", s.url)
	}
	if len(s.performers) != 1 || s.performers[0] != "Sage Cherie" {
		t.Errorf("performers = %v, want [Sage Cherie]", s.performers)
	}
	if s.duration != 1546 {
		t.Errorf("duration = %d, want 1546 (25:46)", s.duration)
	}
	if !s.date.Equal(time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date = %v, want 2026-06-08", s.date)
	}
	if s.rating != 85 {
		t.Errorf("rating = %d, want 85 (8.57 * 10 truncated)", s.rating)
	}
	if s.thumb != "https://tour.girlsoutwest.com/content//contentthumbs/05/39/200539-1x.jpg" {
		t.Errorf("thumb = %q", s.thumb)
	}

	s2 := scenes[1]
	if s2.id != "Ruby-and-Anya-Touch" {
		t.Errorf("scene 2 id = %q, want Ruby-and-Anya-Touch", s2.id)
	}
	if s2.title != "Ruby & Anya - Touch" {
		t.Errorf("scene 2 title = %q", s2.title)
	}
	if len(s2.performers) != 2 {
		t.Errorf("scene 2 performers = %v, want 2 performers", s2.performers)
	}
	if s2.duration != 3750 {
		t.Errorf("scene 2 duration = %d, want 3750 (1:02:30)", s2.duration)
	}
}

func TestParseMaxPage(t *testing.T) {
	got := parseMaxPage([]byte(listingHTML))
	if got != 257 {
		t.Errorf("parseMaxPage = %d, want 257", got)
	}
}

const detailHTML = `<html><body>
<div class="addInfo">
	<h5>Added:</h5>
	<p>June 8, 2026</p>
</div>
<div class="addInfo">
	<h5>Runtime:</h5>
	25:46
</div>
<div class="tags">
	<h5>Tags:</h5>
	<ul class="tags">
		<li><a href="https://tour.girlsoutwest.com/categories/anal.html"><i class="fas fa-tag"></i>Anal</a></li>
		<li><a href="https://tour.girlsoutwest.com/categories/big-boobs.html"><i class="fas fa-tag"></i>Big Boobs</a></li>
		<li><a href="https://tour.girlsoutwest.com/categories/brunette.html"><i class="fas fa-tag"></i>Brunette</a></li>
	</ul>
</div>
<div class="tags">
	<h5>Featuring:</h5>
	<ul>
		<li><a href="https://tour.girlsoutwest.com/models/Sage-Cherie.html"><i class="fas fa-star"></i>Sage Cherie</a></li>
	</ul>
</div>
<div class="descriptionR">
	<div class="description">
		<h4>Description</h4>
		<p>As Sage Cherie traces her fingers over different textures, she imagines how they would feel.
<br><br>
Her imagination runs wild as she plays.</p>
	</div>
</div>
</body></html>`

func TestParseDetailPage(t *testing.T) {
	d := parseDetailPage([]byte(detailHTML))

	if !d.date.Equal(time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date = %v, want 2026-06-08", d.date)
	}
	if d.duration != 1546 {
		t.Errorf("duration = %d, want 1546", d.duration)
	}
	if len(d.tags) != 3 {
		t.Errorf("tags = %v, want 3 tags", d.tags)
	} else {
		if d.tags[0] != "Anal" {
			t.Errorf("tags[0] = %q, want Anal", d.tags[0])
		}
		if d.tags[1] != "Big Boobs" {
			t.Errorf("tags[1] = %q, want Big Boobs", d.tags[1])
		}
	}
	if len(d.performers) != 1 || d.performers[0] != "Sage Cherie" {
		t.Errorf("performers = %v, want [Sage Cherie]", d.performers)
	}
	if d.description == "" {
		t.Error("description is empty")
	}
	if want := "As Sage Cherie traces her fingers over different textures, she imagines how they would feel. Her imagination runs wild as she plays."; d.description != want {
		t.Errorf("description = %q, want %q", d.description, want)
	}
}

func TestParseModelHelpers(t *testing.T) {
	html := `<div class="bioInfo"><h1>Sage Cherie</h1></div>
	<a href="/sets.php?id=42&page=1">next</a>`

	name := parseModelName([]byte(html))
	if name != "Sage Cherie" {
		t.Errorf("parseModelName = %q, want Sage Cherie", name)
	}

	id := parseModelID([]byte(html))
	if id != "42" {
		t.Errorf("parseModelID = %q, want 42", id)
	}
}

func TestRunListing(t *testing.T) {
	page1 := fmt.Sprintf(`<html><body>
%s
%s
<div class="pagination shorterPadding">
	<a href="/categories/Movies.html" class="active">1</a>
</div>
</body></html>`,
		makeSceneEntry("Scene-One", "Scene One", "Ava", "10:00", "Jan 1, 2026", "7.50", "/content//contentthumbs/01/01/100001-1x.jpg"),
		makeSceneEntry("Scene-Two", "Scene Two", "Bella", "20:00", "Jan 2, 2026", "8.00", "/content//contentthumbs/02/02/100002-1x.jpg"),
	)

	detail := `<html><body>
<div class="addInfo"><h5>Added:</h5><p>January 1, 2026</p></div>
<div class="addInfo"><h5>Runtime:</h5> 10:00</div>
<div class="tags"><h5>Tags:</h5><ul class="tags">
	<li><a href="/categories/solo.html"><i class="fas fa-tag"></i>Solo</a></li>
</ul></div>
<div class="tags"><h5>Featuring:</h5><ul>
	<li><a href="/models/Ava.html"><i class="fas fa-star"></i>Ava</a></li>
</ul></div>
<div class="descriptionR"><div class="description"><h4>Description</h4><p>Test description.</p></div></div>
</body></html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/categories/Movies.html":
			_, _ = fmt.Fprint(w, page1)
		case "/trailers/Scene-One.html", "/trailers/Scene-Two.html":
			_, _ = fmt.Fprint(w, detail)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New()
	s.Client = ts.Client()
	s.base = ts.URL

	ctx := context.Background()
	ch, err := s.ListScenes(ctx, ts.URL+"/", scraper.ListOpts{Workers: 1, Delay: 0})
	if err != nil {
		t.Fatal(err)
	}

	var scenes []string
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes = append(scenes, r.Scene.ID)
			if r.Scene.Description != "Test description." {
				t.Errorf("scene %s: description = %q", r.Scene.ID, r.Scene.Description)
			}
			if len(r.Scene.Tags) != 1 || r.Scene.Tags[0] != "Solo" {
				t.Errorf("scene %s: tags = %v", r.Scene.ID, r.Scene.Tags)
			}
		case scraper.KindError:
			t.Errorf("got error: %v", r.Err)
		}
	}

	if len(scenes) != 2 {
		t.Errorf("got %d scenes, want 2", len(scenes))
	}
}

func TestRunModel(t *testing.T) {
	modelPage := fmt.Sprintf(`<html><body>
<div class="bioInfo"><h1>Test Model</h1></div>
%s
</body></html>`,
		makeSceneEntry("Model-Scene", "Model Scene", "Test Model", "15:00", "Mar 1, 2026", "9.00", "/content//contentthumbs/01/01/100001-1x.jpg"),
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models/Test-Model.html":
			_, _ = fmt.Fprint(w, modelPage)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	s := New()
	s.Client = ts.Client()
	s.base = ts.URL

	ctx := context.Background()
	ch, err := s.ListScenes(ctx, ts.URL+"/models/Test-Model.html", scraper.ListOpts{Delay: 0})
	if err != nil {
		t.Fatal(err)
	}

	var scenes int
	for r := range ch {
		switch r.Kind {
		case scraper.KindScene:
			scenes++
			if r.Scene.Title != "Model Scene" {
				t.Errorf("title = %q, want Model Scene", r.Scene.Title)
			}
			if len(r.Scene.Performers) != 1 || r.Scene.Performers[0] != "Test Model" {
				t.Errorf("performers = %v", r.Scene.Performers)
			}
		case scraper.KindError:
			t.Errorf("got error: %v", r.Err)
		}
	}

	if scenes != 1 {
		t.Errorf("got %d scenes, want 1", scenes)
	}
}

func makeSceneEntry(slug, title, performer, duration, date, rating, thumb string) string {
	return fmt.Sprintf(`<div class="iLatestScene">
	<div class="iLScenePic item-thumb-videothumb">
		<div class="vid_123">
			<a href="/trailers/%s.html"></a>
			<img src="%s" alt="" class="video_placeholder" />
		</div>
	</div>
	<div class="iLSceneDetails">
		<h4><a href="/trailers/%s.html" title="%s">%s</a></h4>
		<div class="featuring">Featuring: <a href="/models/%s.html">%s</a></div>
		<ul class="sceneInfo">
			<li><i class="fas fa-clock"></i> %s</li>
			<li><i class="fas fa-calendar"></i> %s</li>
			<li><i class="fas fa-comment"></i> 0</li>
			<li><span><i class="fas fa-star"></i> %s</span></li>
		</ul>
	</div>
</div>`, slug, thumb, slug, title, title, performer, performer, duration, date, rating)
}
