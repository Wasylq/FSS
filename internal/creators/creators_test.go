package creators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const maraFile = `name: Mara Vance
aliases: [MaraVanceVIP]
stores:
  - url: https://clipmarket.example/studio/4021/mara-vance
  - url: https://maravance.example
    delay: 2000
  - url: https://fanhub.example/mara-vance
    enabled: false
    note: needs a session cookie
`

func TestKeyIgnoresSpacingAndPunctuation(t *testing.T) {
	// The point of the tighter key: one person spelled three ways across three
	// storefronts has to resolve to one lookup value.
	cases := map[string]string{
		"Vera Quill":       "veraquill",
		"VeraQuill":        "veraquill",
		"vera-quill":       "veraquill",
		"Vera  Quill!":     "veraquill",
		"thegirlupstairs7": "thegirlupstairs7",
		"":                 "",
		"---":              "",
	}
	for in, want := range cases {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFilename(t *testing.T) {
	cases := map[string]string{
		"Mara Vance": "mara-vance.yaml",
		"VeraQuill":  "veraquill.yaml",
		"Ines  Dahl": "ines-dahl.yaml",
		"!!!":        "creator.yaml",
	}
	for in, want := range cases {
		if got := Filename(in); got != want {
			t.Errorf("Filename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadReadsStoresAndPerStoreSettings(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "mara-vance.yaml", maraFile)

	list, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("loaded %d creators, want 1", len(list))
	}
	c := list[0]
	if c.Name != "Mara Vance" {
		t.Errorf("Name = %q", c.Name)
	}
	if len(c.Stores) != 3 {
		t.Fatalf("Stores = %d, want 3", len(c.Stores))
	}
	if c.Stores[1].Delay == nil || *c.Stores[1].Delay != 2000 {
		t.Errorf("per-store delay not read: %v", c.Stores[1].Delay)
	}
	if c.Stores[2].On() {
		t.Error("store with enabled:false reported as on")
	}
	if got := len(c.EnabledStores()); got != 2 {
		t.Errorf("EnabledStores = %d, want 2", got)
	}
	if got := len(c.URLs()); got != 3 {
		t.Errorf("URLs = %d, want 3 (disabled stores still listed)", got)
	}
	// The file name is cosmetic; the name field is authoritative.
	if c.Path != filepath.Join(dir, "mara-vance.yaml") {
		t.Errorf("Path = %q", c.Path)
	}
}

// An absent directory is the normal state before the first `creators suggest
// --write`, and every command consults creators, so it must not be an error.
func TestLoadMissingDirectoryIsNotAnError(t *testing.T) {
	list, err := Load(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("Load of a missing directory: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("got %d creators from a missing directory", len(list))
	}
}

// A shared creators repository carries a README, a LICENSE and a .git — none of
// which are creators.
func TestLoadSkipsNonYAMLAndDotfiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "mara-vance.yaml", maraFile)
	write(t, dir, "README.md", "# creators")
	write(t, dir, "LICENSE", "MIT")
	write(t, dir, ".hidden.yaml", "name: nope\nstores: []\n")
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	list, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("loaded %d creators, want only the YAML one", len(list))
	}
}

func TestLoadRejectsInvalidFiles(t *testing.T) {
	cases := map[string]string{
		"no name":        "stores:\n  - url: https://example.com\n",
		"no stores":      "name: Someone\n",
		"empty stores":   "name: Someone\nstores: []\n",
		"schemeless":     "name: Someone\nstores:\n  - url: example.com\n",
		"duplicate url":  "name: Someone\nstores:\n  - url: https://example.com\n  - url: https://example.com\n",
		"negative delay": "name: Someone\nstores:\n  - url: https://example.com\n    delay: -1\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "c.yaml", body)
			if _, err := Load(dir); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

// Writing `urls:` instead of `stores:` is the copy-paste mistake this format
// invites. It must fail loudly rather than yield a creator with no storefronts.
func TestLoadRejectsWrongStoresKey(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "c.yaml", "name: Someone\nurls:\n  - https://example.com\n")
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected an error for a file with no stores")
	}
	if !strings.Contains(err.Error(), "stores") {
		t.Errorf("error should name the missing key, got: %v", err)
	}
}

func TestLoadRejectsDuplicateNamesAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.yaml", "name: Mara Vance\nstores:\n  - url: https://a.example.com\n")
	// Different spelling, same key — the collision this is meant to catch.
	write(t, dir, "b.yaml", "name: MaraVance\nstores:\n  - url: https://b.example.com\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected a duplicate-name error")
	}
}

func TestLoadRejectsAliasCollidingWithAnotherName(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.yaml", "name: Alice\nstores:\n  - url: https://a.example.com\n")
	write(t, dir, "b.yaml", "name: Bob\naliases: [alice]\nstores:\n  - url: https://b.example.com\n")
	if _, err := Load(dir); err == nil {
		t.Fatal("expected an alias collision error")
	}
}

func TestLoadSortsByName(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "z.yaml", "name: Zoe\nstores:\n  - url: https://z.example.com\n")
	write(t, dir, "a.yaml", "name: Amy\nstores:\n  - url: https://a.example.com\n")
	list, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "Amy" {
		t.Fatalf("not sorted by name: %+v", list)
	}
}

func TestFind(t *testing.T) {
	list := []Creator{
		{Name: "Mara Vance", Aliases: []string{"MaraVanceVIP"}},
		{Name: "Mara Quinn"},
		{Name: "Mona Reeve"},
	}

	t.Run("exact name", func(t *testing.T) {
		c, err := Find(list, "mona reeve")
		if err != nil || c.Name != "Mona Reeve" {
			t.Fatalf("got %q, %v", c.Name, err)
		}
	})
	t.Run("spacing ignored", func(t *testing.T) {
		c, err := Find(list, "MonaReeve")
		if err != nil || c.Name != "Mona Reeve" {
			t.Fatalf("got %q, %v", c.Name, err)
		}
	})
	t.Run("alias", func(t *testing.T) {
		c, err := Find(list, "maravancevip")
		if err != nil || c.Name != "Mara Vance" {
			t.Fatalf("got %q, %v", c.Name, err)
		}
	})
	t.Run("unique prefix", func(t *testing.T) {
		c, err := Find(list, "mona")
		if err != nil || c.Name != "Mona Reeve" {
			t.Fatalf("got %q, %v", c.Name, err)
		}
	})
	// "mara" prefixes two creators: picking one silently would scrape the wrong
	// person's catalogue.
	t.Run("ambiguous prefix is an error", func(t *testing.T) {
		_, err := Find(list, "mara")
		if err == nil {
			t.Fatal("expected an ambiguity error")
		}
		if !strings.Contains(err.Error(), "Mara Vance") || !strings.Contains(err.Error(), "Mara Quinn") {
			t.Errorf("error should name both candidates, got: %v", err)
		}
	})
	t.Run("no match lists what exists", func(t *testing.T) {
		_, err := Find(list, "nobody")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "Mona Reeve") {
			t.Errorf("error should list available creators, got: %v", err)
		}
	})
	// An exact match must beat a prefix match, or a creator whose name prefixes
	// another's becomes unreachable.
	t.Run("exact beats prefix", func(t *testing.T) {
		two := []Creator{{Name: "Ivy"}, {Name: "Ivy Chamberlain"}}
		c, err := Find(two, "Ivy")
		if err != nil || c.Name != "Ivy" {
			t.Fatalf("got %q, %v", c.Name, err)
		}
	})
}

func TestMarshalRoundTrips(t *testing.T) {
	dir := t.TempDir()
	on := false
	delay := 1500
	orig := Creator{
		Name:    "Mona Reeve",
		Aliases: []string{"MonaReeveVIP"},
		Stores: []Store{
			{URL: "https://clipmarket.example/studio/3207/mona-reeve"},
			{URL: "https://clipstore.example/creators/mona-reeve-1", Delay: &delay, Enabled: &on, Note: "slow"},
		},
	}
	body, err := orig.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, Filename(orig.Name), string(body))

	list, err := Load(dir)
	if err != nil {
		t.Fatalf("re-loading marshalled creator: %v\n%s", err, body)
	}
	if len(list) != 1 {
		t.Fatalf("loaded %d", len(list))
	}
	got := list[0]
	if got.Name != orig.Name || len(got.Stores) != 2 || len(got.Aliases) != 1 {
		t.Fatalf("round trip lost data: %+v", got)
	}
	if got.Stores[1].Delay == nil || *got.Stores[1].Delay != 1500 || got.Stores[1].On() || got.Stores[1].Note != "slow" {
		t.Errorf("per-store settings lost: %+v", got.Stores[1])
	}
	// A creator with no optional fields must not emit empty keys that then
	// read back as meaningful.
	plain, err := Creator{Name: "A", Stores: []Store{{URL: "https://a.example.com"}}}.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"aliases", "delay", "enabled", "note", "path"} {
		if strings.Contains(string(plain), key+":") {
			t.Errorf("marshalled a bare creator with a %q key:\n%s", key, plain)
		}
	}
}
