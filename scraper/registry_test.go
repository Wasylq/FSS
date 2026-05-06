package scraper

import (
	"context"
	"testing"
)

type fakeScraper struct {
	id       string
	patterns []string
	matchFn  func(string) bool
}

func (f *fakeScraper) ID() string         { return f.id }
func (f *fakeScraper) Patterns() []string  { return f.patterns }
func (f *fakeScraper) MatchesURL(u string) bool { return f.matchFn(u) }
func (f *fakeScraper) ListScenes(_ context.Context, _ string, _ ListOpts) (<-chan SceneResult, error) {
	return nil, nil
}

func withCleanRegistry(t *testing.T, scrapers ...*fakeScraper) {
	t.Helper()
	old := registered
	registered = nil
	for _, s := range scrapers {
		Register(s)
	}
	t.Cleanup(func() { registered = old })
}

func TestForID(t *testing.T) {
	a := &fakeScraper{id: "alpha"}
	b := &fakeScraper{id: "beta"}
	withCleanRegistry(t, a, b)

	got, err := ForID("beta")
	if err != nil {
		t.Fatalf("ForID(beta): %v", err)
	}
	if got.ID() != "beta" {
		t.Errorf("got %q, want beta", got.ID())
	}
}

func TestForIDNotFound(t *testing.T) {
	withCleanRegistry(t)

	_, err := ForID("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
}

func TestForURL(t *testing.T) {
	a := &fakeScraper{
		id:      "alpha",
		matchFn: func(u string) bool { return u == "https://alpha.com/videos" },
	}
	b := &fakeScraper{
		id:      "beta",
		matchFn: func(u string) bool { return u == "https://beta.com/videos" },
	}
	withCleanRegistry(t, a, b)

	got, err := ForURL("https://beta.com/videos")
	if err != nil {
		t.Fatalf("ForURL: %v", err)
	}
	if got.ID() != "beta" {
		t.Errorf("got %q, want beta", got.ID())
	}
}

func TestForURLNotFound(t *testing.T) {
	withCleanRegistry(t)

	_, err := ForURL("https://unknown.com")
	if err == nil {
		t.Fatal("expected error for unknown URL")
	}
}

func TestForURLReturnsFirst(t *testing.T) {
	a := &fakeScraper{id: "first", matchFn: func(string) bool { return true }}
	b := &fakeScraper{id: "second", matchFn: func(string) bool { return true }}
	withCleanRegistry(t, a, b)

	got, err := ForURL("https://anything.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID() != "first" {
		t.Errorf("ForURL should return first match, got %q", got.ID())
	}
}

func TestAll(t *testing.T) {
	a := &fakeScraper{id: "a"}
	b := &fakeScraper{id: "b"}
	c := &fakeScraper{id: "c"}
	withCleanRegistry(t, a, b, c)

	all := All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d, want 3", len(all))
	}
	ids := make(map[string]bool)
	for _, s := range all {
		ids[s.ID()] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !ids[want] {
			t.Errorf("All() missing %q", want)
		}
	}
}

func TestAllEmpty(t *testing.T) {
	withCleanRegistry(t)

	all := All()
	if len(all) != 0 {
		t.Errorf("All() on empty registry returned %d", len(all))
	}
}

func TestResultKindString(t *testing.T) {
	tests := []struct {
		kind ResultKind
		want string
	}{
		{KindScene, "Scene"},
		{KindError, "Error"},
		{KindTotal, "Total"},
		{KindStoppedEarly, "StoppedEarly"},
		{ResultKind(99), "ResultKind(99)"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}
