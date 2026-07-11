package pervcityutil

import "testing"

func newPervCity() *Scraper {
	return New(SiteConfig{SiteID: "pervcity", Domain: "pervcity.com", Host: "https://pervcity.com", StudioName: "PervCity", PathStem: "updates"})
}

func newAnalOverdose() *Scraper {
	return New(SiteConfig{SiteID: "analoverdose", Domain: "analoverdose.com", Host: "https://analoverdose.com", StudioName: "Anal Overdose", PathStem: "movies"})
}

// pervCityFixture mirrors a real pervcity.com card: videoContent container,
// <h2> title, an absolute trailer href, a date div, "Runtime: MM:SS minutes",
// and a <p> description. Pagination links seed estimateTotal.
const pervCityFixture = `
<div class="paginate"><a href="/categories/updates_2_d.html">2</a> <a href="/categories/updates_132_d.html">Last</a></div>
<div class="videoBlock">
	<div class="videoPic"><a  href="https://pervcity.com/trailers/Tattooed-Baddie-Naomi-Korae-Loves-The-Taste-Of-Hardcore-Anal-Pounding.html" onclick="tload('/trailers/uha-0450-us.mp4'); return false;">
<img id="set-target-1623" class=" stdimage thumbs" src="https://c758cac692.mjedge.net/content/contentthumbs/05/24/30524-1x.jpg?expires=1&l=41&token=abc" src0_1x="https://c758cac692.mjedge.net/content/contentthumbs/05/24/30524-1x.jpg?expires=1&l=41&token=abc" cnt="1" v="0" />		<span class="badge badge--4k"></span>		</a>
	</div>
	<div class="videoContent">
		<h2><a  href="https://pervcity.com/trailers/Tattooed-Baddie-Naomi-Korae-Loves-The-Taste-Of-Hardcore-Anal-Pounding.html" onclick="return false;">Tattooed Baddie Naomi Korae Loves The Taste Of Hardcore Anal Pounding</a></h2>
		<h3>
	<span class="tour_update_models">
		<a href="https://pervcity.com/models/NaomiKorae.html">Naomi Korae</a>, <a href="https://pervcity.com/models/VinceKarter.html">Vince Karter</a>
	</span>
</h3>
		<div class="date">06-25-2026</div>
		<div class="runtime">Runtime:
		40:58 minutes		</div>
		<p>Blushing because of Maestro&rsquo;s compliments, Naomi Korae shyly covers her face.</p>
	</div>
</div>
`

// analOverdoseFixture mirrors a real analoverdose.com card: videoDetails
// container, <h3> title, NO date div, "Runtime: MM:SS" without the "minutes"
// suffix and no <p>. Uses RELATIVE trailer/thumb URLs to exercise absURL.
const analOverdoseFixture = `
<div class="videoBlock">
	<div class="videoPic">
		<a  href="/trailers/Hot-MILF-Nicole-Ends-Her-First-Professional-Anal-With-A-Facial.html" onclick="return false;">
<img id="set-target-1622" class=" stdimage thumbs" src="/content/contentthumbs/05/03/30503-1x.jpg" src0_1x="/content/contentthumbs/05/03/30503-1x.jpg" cnt="1" v="0" />		</a>
	</div>
	<div class="videoDetails">
		<h3><a  href="/trailers/Hot-MILF-Nicole-Ends-Her-First-Professional-Anal-With-A-Facial.html" onclick="return false;">Hot MILF Nicole Ends Her First Professional Anal With A Facial</a></h3>
		<div class="modelList">
	<span class="tour_update_models">
		<a href="https://analoverdose.com/models/HotMILFNicole.html">Hot MILF Nicole</a>, <a href="https://analoverdose.com/models/SoloTheBull.html">Solo The Bull</a>
	</span>
		</div>
		<div class="runtime">Runtime:
		41:06		</div>
	</div>
</div>
`

func TestParsePervCity(t *testing.T) {
	s := newPervCity()
	scenes := s.parseListing([]byte(pervCityFixture), "https://pervcity.com/categories/updates_1_d.html")
	if len(scenes) != 1 {
		t.Fatalf("expected 1 scene, got %d", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "Tattooed-Baddie-Naomi-Korae-Loves-The-Taste-Of-Hardcore-Anal-Pounding" {
		t.Errorf("ID = %q", sc.ID)
	}
	if sc.URL != "https://pervcity.com/trailers/Tattooed-Baddie-Naomi-Korae-Loves-The-Taste-Of-Hardcore-Anal-Pounding.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Title != "Tattooed Baddie Naomi Korae Loves The Taste Of Hardcore Anal Pounding" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.SiteID != "pervcity" || sc.Studio != "PervCity" {
		t.Errorf("SiteID/Studio = %q/%q", sc.SiteID, sc.Studio)
	}
	if sc.Date.Format("2006-01-02") != "2026-06-25" {
		t.Errorf("Date = %v", sc.Date)
	}
	if sc.Duration != 40*60+58 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 40*60+58)
	}
	if len(sc.Performers) != 2 || sc.Performers[0] != "Naomi Korae" || sc.Performers[1] != "Vince Karter" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Description != "Blushing because of Maestro’s compliments, Naomi Korae shyly covers her face." {
		t.Errorf("Description = %q", sc.Description)
	}
	if sc.Thumbnail != "https://c758cac692.mjedge.net/content/contentthumbs/05/24/30524-1x.jpg?expires=1&l=41&token=abc" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
}

func TestParseAnalOverdose(t *testing.T) {
	s := newAnalOverdose()
	scenes := s.parseListing([]byte(analOverdoseFixture), "https://analoverdose.com/categories/movies_1_d.html")
	if len(scenes) != 1 {
		t.Fatalf("expected 1 scene, got %d", len(scenes))
	}
	sc := scenes[0]
	if sc.ID != "Hot-MILF-Nicole-Ends-Her-First-Professional-Anal-With-A-Facial" {
		t.Errorf("ID = %q", sc.ID)
	}
	// Relative href resolved against Host.
	if sc.URL != "https://analoverdose.com/trailers/Hot-MILF-Nicole-Ends-Her-First-Professional-Anal-With-A-Facial.html" {
		t.Errorf("URL = %q", sc.URL)
	}
	if sc.Title != "Hot MILF Nicole Ends Her First Professional Anal With A Facial" {
		t.Errorf("Title = %q", sc.Title)
	}
	if sc.Duration != 41*60+6 {
		t.Errorf("Duration = %d, want %d", sc.Duration, 41*60+6)
	}
	if len(sc.Performers) != 2 || sc.Performers[0] != "Hot MILF Nicole" || sc.Performers[1] != "Solo The Bull" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	// No date or description on this template.
	if !sc.Date.IsZero() {
		t.Errorf("Date should be zero, got %v", sc.Date)
	}
	if sc.Description != "" {
		t.Errorf("Description should be empty, got %q", sc.Description)
	}
	// Relative thumb resolved against Host.
	if sc.Thumbnail != "https://analoverdose.com/content/contentthumbs/05/03/30503-1x.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
}

func TestEstimateTotal(t *testing.T) {
	if got := estimateTotal([]byte(pervCityFixture), "updates"); got != 132*perPage {
		t.Errorf("estimateTotal = %d, want %d", got, 132*perPage)
	}
	// Stem must match: "movies" finds nothing in the pervcity fixture.
	if got := estimateTotal([]byte(pervCityFixture), "movies"); got != 0 {
		t.Errorf("estimateTotal(movies) = %d, want 0", got)
	}
}

func TestMatchesURL(t *testing.T) {
	pc := newPervCity()
	ao := newAnalOverdose()
	if !pc.MatchesURL("https://pervcity.com/categories/updates_1_d.html") {
		t.Error("pc should match pervcity.com")
	}
	if pc.MatchesURL("https://analoverdose.com/categories/movies_1_d.html") {
		t.Error("pc should not match analoverdose.com")
	}
	if !ao.MatchesURL("https://www.analoverdose.com/models/HotMILFNicole.html") {
		t.Error("ao should match www.analoverdose.com")
	}
}
