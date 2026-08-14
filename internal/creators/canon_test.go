package creators

import (
	"strings"
	"testing"
)

// veraQuill has one store whose branding differs from her name only in the
// studio field, and one that spells it differently again — the two cases the
// two rules cover.
func veraQuill() Creator {
	return Creator{
		Name: "Vera Quill",
		Stores: []Store{
			{URL: "https://clipmarket.example/studio/4021/vera-quill"},
			{URL: "https://fanhub.example/veraquillfilms"},
		},
	}
}

const (
	clipStore = "https://clipmarket.example/studio/4021/vera-quill"
	fanStore  = "https://fanhub.example/veraquillfilms"
)

// The rule that needs nothing configured: a storefront crediting itself as the
// performer says so in the studio field too.
func TestCanonRewritesACreditMatchingTheScenesOwnStudio(t *testing.T) {
	c := NewCanon([]Creator{veraQuill()})
	sc := c.For(fanStore)
	if !sc.Active() {
		t.Fatal("a listed store got no rewrite")
	}
	got := sc.Apply("Vera Quill Films", []string{"Vera Quill Films"})
	if strings.Join(got, ",") != "Vera Quill" {
		t.Errorf("Apply = %v, want [Vera Quill]", got)
	}
}

// No aliases anywhere in this creator file. The studio rule has to carry it,
// or every operator is back to hand-writing what FSS already knows.
func TestCanonNeedsNoAliases(t *testing.T) {
	c := NewCanon([]Creator{veraQuill()})
	if len(veraQuill().Aliases) != 0 {
		t.Fatal("fixture is supposed to declare no aliases")
	}
	got := c.For(clipStore).Apply("VeraQuill", []string{"VeraQuill", "Ada Stone"})
	if strings.Join(got, ",") != "Vera Quill,Ada Stone" {
		t.Errorf("Apply = %v, want [Vera Quill Ada Stone]", got)
	}
}

// The residue the studio rule cannot reach: the store spells its own name one
// way in the studio field and another in the credit. That is what an alias is
// for, and all an alias is for.
func TestCanonAliasCoversAStudioMismatch(t *testing.T) {
	c := NewCanon([]Creator{{
		Name:    "Mara Vance",
		Aliases: []string{"MaraVance"},
		Stores:  []Store{{URL: "https://clipmarket.example/studio/77/mara-vance-1"}},
	}})
	sc := c.For("https://clipmarket.example/studio/77/mara-vance-1")
	// The studio field says "Mara Vance 1"; the credit says "MaraVance".
	// Neither matches the other, so only the alias resolves it.
	got := sc.Apply("Mara Vance 1", []string{"MaraVance"})
	if strings.Join(got, ",") != "Mara Vance" {
		t.Errorf("Apply = %v, want [Mara Vance]", got)
	}
}

// The limit that makes this safe to leave on: a name that is neither the shop
// nor a declared spelling of it is somebody else, and stays as published.
func TestCanonLeavesCoStarsAlone(t *testing.T) {
	c := NewCanon([]Creator{veraQuill()})
	in := []string{"Vera Quill Films", "Ada Stone", "Vera", "Quill Bishop"}
	got := c.For(fanStore).Apply("Vera Quill Films", in)
	want := "Vera Quill,Ada Stone,Vera,Quill Bishop"
	if strings.Join(got, ",") != want {
		t.Errorf("Apply = %v, want %s", got, want)
	}
	if strings.Join(in, ",") != "Vera Quill Films,Ada Stone,Vera,Quill Bishop" {
		t.Errorf("Apply mutated its input: %v", in)
	}
}

// A name that means the creator on their own store means nothing on anyone
// else's, where it may well be a different person.
func TestCanonIgnoresStoresTheCreatorDoesNotList(t *testing.T) {
	c := NewCanon([]Creator{veraQuill()})
	other := "https://fanhub.example/somebody-else"
	if c.For(other).Active() {
		t.Fatal("an unlisted store got a rewrite")
	}
	got := c.For(other).Apply("Vera Quill Films", []string{"Vera Quill Films"})
	if strings.Join(got, ",") != "Vera Quill Films" {
		t.Errorf("Apply = %v, want the credit untouched", got)
	}
}

// A scene crediting the shop and the person separately names one performer.
func TestCanonCollapsesTheShopAndThePerson(t *testing.T) {
	c := NewCanon([]Creator{veraQuill()})
	got := c.For(fanStore).Apply("Vera Quill Films", []string{"Vera Quill", "Vera Quill Films", "Ada Stone"})
	if strings.Join(got, ",") != "Vera Quill,Ada Stone" {
		t.Errorf("Apply = %v, want [Vera Quill Ada Stone]", got)
	}
}

// A scraper that leaves Studio empty must not turn the studio rule into a
// match-anything: the empty key would otherwise catch every unkeyable credit.
func TestCanonWithNoStudioRewritesOnlyDeclaredNames(t *testing.T) {
	c := NewCanon([]Creator{{
		Name:    "Vera Quill",
		Aliases: []string{"VeraQuill"},
		Stores:  []Store{{URL: fanStore}},
	}})
	got := c.For(fanStore).Apply("", []string{"???", "VeraQuill", "Ada Stone"})
	if strings.Join(got, "|") != "???|Vera Quill|Ada Stone" {
		t.Errorf("Apply = %v, want the unkeyable credit kept and only the alias rewritten", got)
	}
}

func TestCanonMatchesStoreURLsCanonically(t *testing.T) {
	c := NewCanon([]Creator{veraQuill()})
	// The store layer keys on the canonical form, so this has to agree with it
	// or a store is recognised in one place and not the other.
	for _, u := range []string{
		"http://fanhub.example/veraquillfilms",
		"https://fanhub.example/veraquillfilms/",
		"https://FanHub.EXAMPLE/veraquillfilms",
	} {
		if !c.For(u).Active() {
			t.Errorf("%s was not recognised as a listed store", u)
		}
	}
}

// A storefront claimed by two creator files cannot be relabelled to either name
// without guessing. Load already warns; the rewrite simply declines.
func TestCanonDeclinesAStoreClaimedByTwoCreators(t *testing.T) {
	shared := "https://fanhub.example/shared-shop"
	c := NewCanon([]Creator{
		{Name: "Ada Stone", Stores: []Store{{URL: shared}}},
		{Name: "Mara Vance", Stores: []Store{{URL: shared}}},
	})
	if c.For(shared).Active() {
		t.Error("a doubly-claimed store got a rewrite; that is a guess, not a fact")
	}
}

// The zero Canon is what every operator with no creators.d has.
func TestZeroCanonRewritesNothing(t *testing.T) {
	var c Canon
	sc := c.For("https://fanhub.example/anything")
	if sc.Active() {
		t.Fatal("zero Canon reported an active rewrite")
	}
	in := []string{"Ada Stone", ""}
	got := sc.Apply("Ada Stone", in)
	if len(got) != 2 || got[0] != "Ada Stone" || got[1] != "" {
		t.Errorf("Apply = %v, want the input unchanged", got)
	}
	if NewCanon(nil).For("https://fanhub.example/anything").Active() {
		t.Error("NewCanon(nil) produced an active rewrite")
	}
}
