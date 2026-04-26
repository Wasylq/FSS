//go:build integration

package stash

import (
	"context"
	"os"
	"testing"
)

func stashClient(t *testing.T) *Client {
	t.Helper()
	url := os.Getenv("FSS_STASH_URL")
	if url == "" {
		url = "http://localhost:9999"
	}
	apiKey := os.Getenv("FSS_STASH_API_KEY")
	c := NewClient(url, apiKey)
	if err := c.Ping(context.Background()); err != nil {
		t.Skipf("Stash not reachable at %s: %v", url, err)
	}
	return c
}

func TestLivePing(t *testing.T) {
	c := stashClient(t)
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestLiveFindScenes(t *testing.T) {
	c := stashClient(t)
	scenes, count, err := c.FindScenes(context.Background(), FindScenesFilter{}, 1, 5)
	if err != nil {
		t.Fatalf("FindScenes: %v", err)
	}
	t.Logf("Found %d scenes (total %d)", len(scenes), count)
	if count == 0 {
		t.Skip("No scenes in Stash, nothing to validate")
	}
	s := scenes[0]
	if s.ID == "" {
		t.Error("scene ID is empty")
	}
	t.Logf("First scene: id=%s title=%q files=%d performers=%d tags=%d",
		s.ID, s.Title, len(s.Files), len(s.Performers), len(s.Tags))
	if len(s.Files) > 0 {
		t.Logf("  file: %s", s.Files[0].Path)
	}
}

func TestLiveFindScenesUnmatched(t *testing.T) {
	c := stashClient(t)
	zero := 0
	filter := FindScenesFilter{StashIDCount: &zero}
	scenes, count, err := c.FindScenes(context.Background(), filter, 1, 5)
	if err != nil {
		t.Fatalf("FindScenes(unmatched): %v", err)
	}
	t.Logf("Unmatched: %d scenes (total %d)", len(scenes), count)
}

func TestLiveFindSceneByID(t *testing.T) {
	c := stashClient(t)
	scenes, _, err := c.FindScenes(context.Background(), FindScenesFilter{}, 1, 1)
	if err != nil {
		t.Fatalf("FindScenes: %v", err)
	}
	if len(scenes) == 0 {
		t.Skip("No scenes in Stash")
	}

	scene, found, err := c.FindSceneByID(context.Background(), scenes[0].ID)
	if err != nil {
		t.Fatalf("FindSceneByID: %v", err)
	}
	if !found {
		t.Fatal("scene not found by ID")
	}
	if scene.ID != scenes[0].ID {
		t.Errorf("ID mismatch: got %s, want %s", scene.ID, scenes[0].ID)
	}
	t.Logf("FindSceneByID(%s): title=%q", scene.ID, scene.Title)
}

func TestLiveFindAllScenes(t *testing.T) {
	c := stashClient(t)
	ctx := context.Background()

	var pages int
	scenes, err := c.FindAllScenes(ctx, FindScenesFilter{}, func(fetched, total int) {
		pages++
		t.Logf("  page %d: fetched %d / %d", pages, fetched, total)
	})
	if err != nil {
		t.Fatalf("FindAllScenes: %v", err)
	}
	t.Logf("FindAllScenes: %d scenes in %d pages", len(scenes), pages)
}

func TestLiveFilterByPerformer(t *testing.T) {
	c := stashClient(t)
	ctx := context.Background()

	scenes, _, err := c.FindScenes(ctx, FindScenesFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("FindScenes: %v", err)
	}

	var perfName string
	for _, s := range scenes {
		if len(s.Performers) > 0 {
			perfName = s.Performers[0].Name
			break
		}
	}
	if perfName == "" {
		t.Skip("No scenes with performers found")
	}

	filtered, count, err := c.FindScenes(ctx, FindScenesFilter{PerformerName: perfName}, 1, 5)
	if err != nil {
		t.Fatalf("FindScenes(performer=%q): %v", perfName, err)
	}
	t.Logf("Filter by performer %q: %d scenes (total %d)", perfName, len(filtered), count)
	if count == 0 {
		t.Errorf("expected at least 1 scene for performer %q", perfName)
	}
}

func TestLiveFilterByStudio(t *testing.T) {
	c := stashClient(t)
	ctx := context.Background()

	scenes, _, err := c.FindScenes(ctx, FindScenesFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("FindScenes: %v", err)
	}

	var studioName string
	for _, s := range scenes {
		if s.Studio != nil {
			studioName = s.Studio.Name
			break
		}
	}
	if studioName == "" {
		t.Skip("No scenes with studios found")
	}

	filtered, count, err := c.FindScenes(ctx, FindScenesFilter{StudioName: studioName}, 1, 5)
	if err != nil {
		t.Fatalf("FindScenes(studio=%q): %v", studioName, err)
	}
	t.Logf("Filter by studio %q: %d scenes (total %d)", studioName, len(filtered), count)
	if count == 0 {
		t.Errorf("expected at least 1 scene for studio %q", studioName)
	}
}

func TestLiveFindTagByName(t *testing.T) {
	c := stashClient(t)
	ctx := context.Background()

	// Find a tag that exists on any scene
	scenes, _, err := c.FindScenes(ctx, FindScenesFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("FindScenes: %v", err)
	}

	var tagName string
	for _, s := range scenes {
		if len(s.Tags) > 0 {
			tagName = s.Tags[0].Name
			break
		}
	}
	if tagName == "" {
		t.Skip("No scenes with tags found")
	}

	id, found, err := c.FindTagByName(ctx, tagName)
	if err != nil {
		t.Fatalf("FindTagByName(%q): %v", tagName, err)
	}
	if !found {
		t.Errorf("tag %q not found", tagName)
	}
	t.Logf("FindTagByName(%q) = %s", tagName, id)
}

func TestLiveFindPerformerByName(t *testing.T) {
	c := stashClient(t)
	ctx := context.Background()

	scenes, _, err := c.FindScenes(ctx, FindScenesFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("FindScenes: %v", err)
	}

	var perfName string
	for _, s := range scenes {
		if len(s.Performers) > 0 {
			perfName = s.Performers[0].Name
			break
		}
	}
	if perfName == "" {
		t.Skip("No scenes with performers found")
	}

	id, found, err := c.FindPerformerByName(ctx, perfName)
	if err != nil {
		t.Fatalf("FindPerformerByName(%q): %v", perfName, err)
	}
	if !found {
		t.Errorf("performer %q not found", perfName)
	}
	t.Logf("FindPerformerByName(%q) = %s", perfName, id)
}

func TestLiveFindStudioByName(t *testing.T) {
	c := stashClient(t)
	ctx := context.Background()

	scenes, _, err := c.FindScenes(ctx, FindScenesFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("FindScenes: %v", err)
	}

	var studioName string
	for _, s := range scenes {
		if s.Studio != nil {
			studioName = s.Studio.Name
			break
		}
	}
	if studioName == "" {
		t.Skip("No scenes with studios found")
	}

	id, found, err := c.FindStudioByName(ctx, studioName)
	if err != nil {
		t.Fatalf("FindStudioByName(%q): %v", studioName, err)
	}
	if !found {
		t.Errorf("studio %q not found", studioName)
	}
	t.Logf("FindStudioByName(%q) = %s", studioName, id)
}

func TestLiveFindTagByName_notFound(t *testing.T) {
	c := stashClient(t)
	_, found, err := c.FindTagByName(context.Background(), "fss_nonexistent_tag_42xyz")
	if err != nil {
		t.Fatalf("FindTagByName: %v", err)
	}
	if found {
		t.Error("expected tag not to be found")
	}
}

func TestLiveFindPerformerByName_notFound(t *testing.T) {
	c := stashClient(t)
	_, found, err := c.FindPerformerByName(context.Background(), "FSS Nonexistent Performer 42xyz")
	if err != nil {
		t.Fatalf("FindPerformerByName: %v", err)
	}
	if found {
		t.Error("expected performer not to be found")
	}
}

func TestLiveFindStudioByName_notFound(t *testing.T) {
	c := stashClient(t)
	_, found, err := c.FindStudioByName(context.Background(), "FSS Nonexistent Studio 42xyz")
	if err != nil {
		t.Fatalf("FindStudioByName: %v", err)
	}
	if found {
		t.Error("expected studio not to be found")
	}
}
