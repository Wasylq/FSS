package parseutil

import (
	"testing"
)

func TestExtractVideoObject_basic(t *testing.T) {
	body := []byte(`<html><head>
<script type="application/ld+json">
{
  "@type": "VideoObject",
  "name": "Test Video",
  "description": "A description",
  "thumbnailUrl": "https://example.com/thumb.jpg",
  "contentUrl": "https://example.com/video.mp4",
  "duration": "PT1H30M",
  "uploadDate": "2024-06-15T00:00:00Z",
  "keywords": "tag1, tag2"
}
</script></head></html>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject, got nil")
	}
	if vo.Name != "Test Video" {
		t.Errorf("Name = %q", vo.Name)
	}
	if vo.Description != "A description" {
		t.Errorf("Description = %q", vo.Description)
	}
	if vo.ThumbnailURL != "https://example.com/thumb.jpg" {
		t.Errorf("ThumbnailURL = %q", vo.ThumbnailURL)
	}
	if vo.ContentURL != "https://example.com/video.mp4" {
		t.Errorf("ContentURL = %q", vo.ContentURL)
	}
	if vo.Duration != "PT1H30M" {
		t.Errorf("Duration = %q", vo.Duration)
	}
	if vo.UploadDate != "2024-06-15T00:00:00Z" {
		t.Errorf("UploadDate = %q", vo.UploadDate)
	}
	if vo.Keywords != "tag1, tag2" {
		t.Errorf("Keywords = %q", vo.Keywords)
	}
}

func TestExtractVideoObject_actorObjectArray(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type":"VideoObject","name":"V","actor":[{"name":"Alice"},{"name":"Bob"}]}
</script>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject")
	}
	if len(vo.Actors) != 2 || vo.Actors[0] != "Alice" || vo.Actors[1] != "Bob" {
		t.Errorf("Actors = %v", vo.Actors)
	}
}

func TestExtractVideoObject_actorStringArray(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type":"VideoObject","name":"V","actor":["Alice","Bob"]}
</script>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject")
	}
	if len(vo.Actors) != 2 || vo.Actors[0] != "Alice" || vo.Actors[1] != "Bob" {
		t.Errorf("Actors = %v", vo.Actors)
	}
}

func TestExtractVideoObject_actorSingleString(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type":"VideoObject","name":"V","actor":"Solo Performer"}
</script>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject")
	}
	if len(vo.Actors) != 1 || vo.Actors[0] != "Solo Performer" {
		t.Errorf("Actors = %v", vo.Actors)
	}
}

func TestExtractVideoObject_actorSingleObject(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type":"VideoObject","name":"V","actor":{"name":"Solo"}}
</script>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject")
	}
	if len(vo.Actors) != 1 || vo.Actors[0] != "Solo" {
		t.Errorf("Actors = %v", vo.Actors)
	}
}

func TestExtractVideoObject_director(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type":"VideoObject","name":"V","director":{"name":"John Director"}}
</script>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject")
	}
	if vo.Director != "John Director" {
		t.Errorf("Director = %q", vo.Director)
	}
}

func TestExtractVideoObject_partOfSeries(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type":"VideoObject","name":"V","partOfSeries":{"name":"The Series"}}
</script>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject")
	}
	if vo.PartOfSeries != "The Series" {
		t.Errorf("PartOfSeries = %q", vo.PartOfSeries)
	}
}

func TestExtractVideoObject_skipsNonVideoObject(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type":"WebPage","name":"Not a video"}
</script>
<script type="application/ld+json">
{"@type":"VideoObject","name":"The Video"}
</script>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject")
	}
	if vo.Name != "The Video" {
		t.Errorf("Name = %q, want The Video", vo.Name)
	}
}

func TestExtractVideoObject_noBlocks(t *testing.T) {
	body := []byte(`<html><body>no json-ld here</body></html>`)
	vo := ExtractVideoObject(body)
	if vo != nil {
		t.Errorf("expected nil, got %+v", vo)
	}
}

func TestExtractVideoObjects_itemList(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{
  "@type": "ItemList",
  "itemListElement": [
    {"item": {"@type": "VideoObject", "name": "V1", "actor": [{"name": "A"}]}},
    {"item": {"@type": "VideoObject", "name": "V2", "actor": [{"name": "B"}]}}
  ]
}
</script>`)

	vos := ExtractVideoObjects(body)
	if len(vos) != 2 {
		t.Fatalf("got %d VideoObjects, want 2", len(vos))
	}
	if vos[0].Name != "V1" {
		t.Errorf("[0].Name = %q", vos[0].Name)
	}
	if vos[1].Name != "V2" {
		t.Errorf("[1].Name = %q", vos[1].Name)
	}
	if len(vos[0].Actors) != 1 || vos[0].Actors[0] != "A" {
		t.Errorf("[0].Actors = %v", vos[0].Actors)
	}
}

func TestExtractVideoObject_datePublished(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type":"VideoObject","name":"V","datePublished":"2024-01-15"}
</script>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject")
	}
	if vo.DatePublished != "2024-01-15" {
		t.Errorf("DatePublished = %q", vo.DatePublished)
	}
}

func TestExtractVideoObject_noActors(t *testing.T) {
	body := []byte(`<script type="application/ld+json">
{"@type":"VideoObject","name":"V"}
</script>`)

	vo := ExtractVideoObject(body)
	if vo == nil {
		t.Fatal("expected VideoObject")
	}
	if len(vo.Actors) != 0 {
		t.Errorf("Actors = %v, want empty", vo.Actors)
	}
}

func TestExtractVideoObjects_mixedBlocks(t *testing.T) {
	body := []byte(`
<script type="application/ld+json">{"@type":"WebPage","name":"WP"}</script>
<script type="application/ld+json">{"@type":"VideoObject","name":"Bare"}</script>
<script type="application/ld+json">
{"@type":"ItemList","itemListElement":[
  {"item":{"@type":"VideoObject","name":"Listed"}}
]}
</script>`)

	vos := ExtractVideoObjects(body)
	if len(vos) != 2 {
		t.Fatalf("got %d, want 2", len(vos))
	}
	if vos[0].Name != "Bare" {
		t.Errorf("[0].Name = %q", vos[0].Name)
	}
	if vos[1].Name != "Listed" {
		t.Errorf("[1].Name = %q", vos[1].Name)
	}
}
