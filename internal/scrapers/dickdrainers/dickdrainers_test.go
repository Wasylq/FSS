package dickdrainers

import "testing"

// listingFixture is a trimmed two-card capture from a real
// /tour/categories/movies/{N}/latest/ page. The first card has a performer; the
// second has an empty "Featuring:" block (performers absent) — both still carry a
// "Tags:" block.
const listingFixture = `
<div class="section-video">
	<div class="inner-area clear">
	<div class="videoDetails clear">
		<h3><a href="https://dickdrainers.com/tour/trailers/Ronniesreminder.html">Petite Shy Pretty Girl Forgot About The BBC Delivery Fee&#128119;&#127999;</a></h3>
		<p><p><span style="color:#00ff00;"> I'm so grateful for this package delivery job.</p>
		<p><span style="color:#00ff00;"> Hope she happy to see me.</p></div>
	<div class="episode_thumbs">
		<div class="left">
			<a href="https://dickdrainers.com/tour/trailers/Ronniesreminder.html">
				<img id="set-target-486" class="mainThumb" src0_1x="/tour/content//contentthumbs/04/27/10427-1x.jpg" src0_2x="/tour/content//contentthumbs/04/27/10427-2x.jpg" cnt="1" v="0" />
			</a>
		</div>
	</div>
	<div class="videoInfo clear">
		<p><span>Date Added:</span> June 18, 2026</p>
		<i>|</i>
		<p>
220&nbsp;Pics, 61&nbsp;min&nbsp;of video</p>
	</div>
	<div class="featuring clear">
		<ul>
			<li class="label">Featuring:</li>
					<li class="update_models">
		<a href="https://dickdrainers.com/tour/models/Ronnie-Violet.html">Ronnie Violet</a>	</li>
			</ul>
	</div>
	<div class="featuring clear">
		<ul>
			<li class="label">Tags:</li><li><a href="https://dickdrainers.com/tour/categories/creampies/1/latest/">Creampies</a></li><li><a href="https://dickdrainers.com/tour/categories/petite/1/latest/">Petite</a></li>		</ul>
	</div>
	</div>
</div><!--//section-video-->

<div class="section-video">
	<div class="inner-area clear">
	<div class="videoDetails clear">
		<h3><a href="https://dickdrainers.com/tour/trailers/PAWGComputer.html">PAWG Need Her Computer Fixed Tonite &amp; I Need Sum Pussy!</a></h3>
		<p><p>Some description here.</p></div>
	<div class="episode_thumbs">
		<div class="left">
			<a href="https://dickdrainers.com/tour/trailers/PAWGComputer.html">
				<img id="set-target-482" class="mainThumb" src0_1x="/tour/content//contentthumbs/03/74/10374-1x.jpg" cnt="1" v="0" />
			</a>
		</div>
	</div>
	<div class="videoInfo clear">
		<p><span>Date Added:</span> May 24, 2026</p>
		<i>|</i>
		<p>
209&nbsp;Pics, 49&nbsp;min&nbsp;of video</p>
	</div>
	<div class="featuring clear">
		<ul>
			<li class="label">Featuring:</li>
			</ul>
	</div>
	<div class="featuring clear">
		<ul>
			<li class="label">Tags:</li><li><a href="https://dickdrainers.com/tour/categories/Fucking/1/latest/">Fucking</a></li>		</ul>
	</div>
	</div>
</div><!--//section-video-->
`

func TestParseListing(t *testing.T) {
	scenes := parseListing([]byte(listingFixture), "https://dickdrainers.com/tour/categories/movies/1/latest/")
	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	s := scenes[0]
	if s.ID != "Ronniesreminder" {
		t.Errorf("ID = %q, want Ronniesreminder", s.ID)
	}
	if s.SiteID != siteID || s.Studio != studioName {
		t.Errorf("SiteID/Studio = %q/%q", s.SiteID, s.Studio)
	}
	if s.Title != "Petite Shy Pretty Girl Forgot About The BBC Delivery Fee👷🏿" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.URL != "https://dickdrainers.com/tour/trailers/Ronniesreminder.html" {
		t.Errorf("URL = %q", s.URL)
	}
	if got := s.Date.Format("2006-01-02"); got != "2026-06-18" {
		t.Errorf("Date = %s, want 2026-06-18", got)
	}
	if s.Duration != 61*60 {
		t.Errorf("Duration = %d, want %d", s.Duration, 61*60)
	}
	if len(s.Performers) != 1 || s.Performers[0] != "Ronnie Violet" {
		t.Errorf("Performers = %v, want [Ronnie Violet]", s.Performers)
	}
	wantTags := []string{"Creampies", "Petite"}
	if len(s.Tags) != len(wantTags) {
		t.Fatalf("Tags = %v, want %v", s.Tags, wantTags)
	}
	for i, tg := range wantTags {
		if s.Tags[i] != tg {
			t.Errorf("Tags[%d] = %q, want %q", i, s.Tags[i], tg)
		}
	}
	if s.Thumbnail != "https://dickdrainers.com/tour/content//contentthumbs/04/27/10427-1x.jpg" {
		t.Errorf("Thumbnail = %q", s.Thumbnail)
	}
	if s.Description == "" {
		t.Error("Description is empty")
	}

	// Second card: empty Featuring block -> no performers, but Tags still parsed.
	s2 := scenes[1]
	if s2.ID != "PAWGComputer" {
		t.Errorf("scene[1].ID = %q, want PAWGComputer", s2.ID)
	}
	if len(s2.Performers) != 0 {
		t.Errorf("scene[1].Performers = %v, want empty", s2.Performers)
	}
	if len(s2.Tags) != 1 || s2.Tags[0] != "Fucking" {
		t.Errorf("scene[1].Tags = %v, want [Fucking]", s2.Tags)
	}
	if s2.Title != "PAWG Need Her Computer Fixed Tonite & I Need Sum Pussy!" {
		t.Errorf("scene[1].Title = %q", s2.Title)
	}
}

const modelFixture = `
<div class="section-profile">
	<div class="profile-details clear">
		<h3>About Ronnie Violet</h3>
	</div>
	<div class="featured-scenes">
		<h3>Ronnie Violet Video Updates</h3>
		<div class="models clear">
	<div class="item-video hover">
		<div class="item-thumb">
			<a href="https://dickdrainers.com/tour/trailers/Ronniesreminder.html" title="Petite Shy Pretty Girl">
				<img id="set-target-486" class="mainThumb" src0_1x="/tour/content//contentthumbs/04/20/10420-1x.jpg" cnt="6" v="0" />
			</a>
		</div>
		<div class="item-info clear">
			<h4>
				<a href="https://dickdrainers.com/tour/trailers/Ronniesreminder.html" title="Petite Shy Pretty Girl">
					Petite Shy Pretty Girl				</a>
			</h4>
			<div class="time">220&nbsp;Pics, 01:01:09</div>
			<div class="date">2026-06-18</div>
		</div>
	</div>
		</div>
	</div>
	<div class="featured-scenes">
		<h3>Ronnie Violet Photo Updates</h3>
		<div class="models clear">
		<div class="item-portrait">
			<a href="https://secure.verotel.com/startorder?shopID=114410" title="Petite Shy Pretty Girl">
				<img id="set-target-486" class="mainThumb" src0_1x="/tour/content//contentthumbs/04/26/10426-1x.jpg" cnt="1" v="0" />
			</a>
			<div class="item-info clear">
				<h4><a href="https://secure.verotel.com/startorder?shopID=114410">Petite Shy Pretty Girl</a></h4>
				<div class="photos">220 Photos</div>
				<div class="date">2026-06-18</div>
			</div>
		</div>
		</div>
	</div>
</div>
`

func TestParseModelPage(t *testing.T) {
	scenes := parseModelPage([]byte(modelFixture), "https://dickdrainers.com/tour/models/Ronnie-Violet.html")
	if len(scenes) != 1 {
		t.Fatalf("got %d scenes, want 1 (photo cards must be ignored)", len(scenes))
	}
	s := scenes[0]
	if s.ID != "Ronniesreminder" {
		t.Errorf("ID = %q, want Ronniesreminder", s.ID)
	}
	if s.Title != "Petite Shy Pretty Girl" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.URL != "https://dickdrainers.com/tour/trailers/Ronniesreminder.html" {
		t.Errorf("URL = %q", s.URL)
	}
	if got := s.Date.Format("2006-01-02"); got != "2026-06-18" {
		t.Errorf("Date = %s, want 2026-06-18", got)
	}
	if s.Duration != 3669 { // 01:01:09
		t.Errorf("Duration = %d, want 3669", s.Duration)
	}
	if len(s.Performers) != 1 || s.Performers[0] != "Ronnie Violet" {
		t.Errorf("Performers = %v, want [Ronnie Violet]", s.Performers)
	}
	if s.Thumbnail != "https://dickdrainers.com/tour/content//contentthumbs/04/20/10420-1x.jpg" {
		t.Errorf("Thumbnail = %q", s.Thumbnail)
	}
}

func TestMatchesURL(t *testing.T) {
	s := New()
	good := []string{
		"https://dickdrainers.com",
		"https://dickdrainers.com/",
		"https://www.dickdrainers.com/tour/categories/movies/1/latest/",
		"http://dickdrainers.com/tour/models/Ronnie-Violet.html",
	}
	for _, u := range good {
		if !s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = false, want true", u)
		}
	}
	bad := []string{"https://example.com", "https://dickdrainersx.com"}
	for _, u := range bad {
		if s.MatchesURL(u) {
			t.Errorf("MatchesURL(%q) = true, want false", u)
		}
	}
}
