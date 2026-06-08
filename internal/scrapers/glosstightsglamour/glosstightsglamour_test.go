package glosstightsglamour

import (
	"testing"
)

const fixtureHTML = `<html>
<div class="update_block">
<div class="update_table_left">
<div class="update_block_info index-page">
<span class="update_title">Tillie in purple dress with grey sheer glossy tights</span>
<br />
<span class="tour_update_models">
<a href="https://www.glosstightsglamour.com/models/tillie.html">Tillie</a>
</span>
<br />
<span class="update_date">08/05/2025</span>
<hr class="update_hr" />
<span class="latest_update_description">Tillie wears a purple dress with grey sheer glossy tights.</span>
<br /><br />
<span class="tour_update_tags">
Tags:
<a href="https://www.glosstightsglamour.com/categories/Tights-Grey.html">Tights - Grey</a>
<a href="https://www.glosstightsglamour.com/categories/10-29-Denier-Sheer.html">10-29 Denier Sheer</a>
</span>
</div>
</div>
<div class="update_table_right">
<div class="update_image index-page">
<div class="index-preview-block" data-vr="0" data-vrformat="" onclick="tload('/trailers/tillie_purple_dress.mp4', this)">
<img id="set-target-207" alt="model" class="large_update_thumb index-page left thumbs stdimage" src0_1x="/content//contentthumbs/35/14/3514-tour-1x.jpg" src0_1x_width="405" src0_webp_1x="/content//contentthumbs/35/14/3514-tour-1x.webp" cnt="1" v="0" />
</div>
</div>
</div>
<div class="update_block_footer"><div class="update_counts_preview_table">50&nbsp;Photos</div></div>
</div>
<div class="between_update_join_links"></div>
<div class="update_block">
<div class="update_table_left">
<div class="update_block_info index-page">
<span class="update_title">Annabelle in copper dress &amp; light tan glossy tights</span>
<br />
<span class="tour_update_models">
<a href="https://www.glosstightsglamour.com/models/annabelle.html">Annabelle</a>
</span>
<br />
<span class="update_date">03/08/2024</span>
<hr class="update_hr" />
<span class="latest_update_description">Annabelle models a copper dress with light tan glossy tights.</span>
</div>
</div>
<div class="update_table_right">
<div class="update_image index-page">
<img id="set-target-195" alt="model" class="large_update_thumb index-page left thumbs stdimage" src0_1x="/content//contentthumbs/59/21/5921-tour-1x.jpg" src0_1x_width="405" cnt="1" v="0" />
</div>
</div>
<div class="update_block_footer"><div class="update_counts_preview_table">40&nbsp;Photos</div></div>
</div>
</html>`

func TestParseListingPage(t *testing.T) {
	scenes := parseListingPage([]byte(fixtureHTML), "https://www.glosstightsglamour.com/")

	if len(scenes) != 2 {
		t.Fatalf("got %d scenes, want 2", len(scenes))
	}

	sc := scenes[0]
	if sc.ID != "207" {
		t.Errorf("ID = %q, want 207", sc.ID)
	}
	if sc.Title != "Tillie in purple dress with grey sheer glossy tights" {
		t.Errorf("Title = %q", sc.Title)
	}
	if len(sc.Performers) != 1 || sc.Performers[0] != "Tillie" {
		t.Errorf("Performers = %v", sc.Performers)
	}
	if sc.Date.Format("2006-01-02") != "2025-05-08" {
		t.Errorf("Date = %v", sc.Date)
	}
	if sc.Description != "Tillie wears a purple dress with grey sheer glossy tights." {
		t.Errorf("Description = %q", sc.Description)
	}
	if sc.Thumbnail != siteBase+"/content//contentthumbs/35/14/3514-tour-1x.jpg" {
		t.Errorf("Thumbnail = %q", sc.Thumbnail)
	}
	if sc.Preview != siteBase+"/trailers/tillie_purple_dress.mp4" {
		t.Errorf("Preview = %q", sc.Preview)
	}
	if len(sc.Tags) != 2 || sc.Tags[0] != "Tights - Grey" {
		t.Errorf("Tags = %v", sc.Tags)
	}

	sc2 := scenes[1]
	if sc2.ID != "195" {
		t.Errorf("ID = %q, want 195", sc2.ID)
	}
	if sc2.Title != "Annabelle in copper dress & light tan glossy tights" {
		t.Errorf("Title = %q (should unescape HTML entities)", sc2.Title)
	}
	if sc2.Preview != "" {
		t.Errorf("Preview = %q, want empty (no trailer)", sc2.Preview)
	}
}

func TestParseMaxPage(t *testing.T) {
	html := []byte(`<a href="/updates/page_2.html">2</a><a href="/updates/page_3.html">3</a>`)
	if got := parseMaxPage(html); got != 3 {
		t.Errorf("parseMaxPage = %d, want 3", got)
	}
}

func TestHasNextPage(t *testing.T) {
	html := []byte(`<a href="/updates/page_5.html">5</a><a href="/updates/page_6.html">6</a>`)
	if !hasNextPage(html, 5) {
		t.Error("hasNextPage(5) should be true when page_6 link exists")
	}
	if hasNextPage(html, 6) {
		t.Error("hasNextPage(6) should be false when no page_7 link exists")
	}
}
