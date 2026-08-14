package creators

import (
	"strings"
	"testing"
)

func taraTainton() Creator {
	return Creator{
		Name:    "Tara Tainton",
		Aliases: []string{"Tara Tainton TV"},
		Stores: []Store{
			{URL: "https://www.clips4sale.com/studio/21571/tara-tainton"},
			{URL: "https://www.loyalfans.com/tarataintontv"},
		},
	}
}

func TestCanonRewritesAnAliasToTheCreatorName(t *testing.T) {
	c := NewCanon([]Creator{taraTainton()})
	sc := c.For("https://www.loyalfans.com/tarataintontv")
	if !sc.Active() {
		t.Fatal("store listed in the creator file got no rewrite")
	}
	got := sc.Apply([]string{"Tara Tainton TV"})
	if strings.Join(got, ",") != "Tara Tainton" {
		t.Errorf("Apply = %v, want [Tara Tainton]", got)
	}
}

// The whole point of scoping to the creator's own stores: a name that means the
// creator on their storefront must not be rewritten anywhere else, because on
// another site it may well be someone else's.
func TestCanonIgnoresStoresTheCreatorDoesNotList(t *testing.T) {
	c := NewCanon([]Creator{taraTainton()})
	if sc := c.For("https://example.com/somebody-else"); sc.Active() {
		t.Fatal("an unlisted store got a rewrite")
	}
	got := c.For("https://example.com/somebody-else").Apply([]string{"Tara Tainton TV"})
	if strings.Join(got, ",") != "Tara Tainton TV" {
		t.Errorf("Apply = %v, want the name untouched", got)
	}
}

// Co-stars are the thing this must never damage. Only strings a human wrote
// into the file are eligible, so an unrelated name on a creator's own store is
// passed through however much it resembles theirs.
func TestCanonLeavesCoStarsAlone(t *testing.T) {
	c := NewCanon([]Creator{taraTainton()})
	sc := c.For("https://www.loyalfans.com/tarataintontv")
	in := []string{"Tara Tainton TV", "Laz Fyre", "Tara", "Tainton Jones"}
	got := sc.Apply(in)
	want := "Tara Tainton,Laz Fyre,Tara,Tainton Jones"
	if strings.Join(got, ",") != want {
		t.Errorf("Apply = %v, want %s", got, want)
	}
	if strings.Join(in, ",") != "Tara Tainton TV,Laz Fyre,Tara,Tainton Jones" {
		t.Errorf("Apply mutated its input: %v", in)
	}
}

// A scene crediting both spellings is one person, and storing two names would
// recreate exactly the duplicate this exists to remove.
func TestCanonCollapsesBothSpellingsOnOneScene(t *testing.T) {
	c := NewCanon([]Creator{taraTainton()})
	sc := c.For("https://www.clips4sale.com/studio/21571/tara-tainton")
	got := sc.Apply([]string{"Tara Tainton", "Tara Tainton TV", "Amy Solo"})
	if strings.Join(got, ",") != "Tara Tainton,Amy Solo" {
		t.Errorf("Apply = %v, want [Tara Tainton Amy Solo]", got)
	}
}

func TestCanonMatchesStoreURLsCanonically(t *testing.T) {
	c := NewCanon([]Creator{taraTainton()})
	// Scheme, host case and a trailing slash must not decide whether a store
	// is recognised — the store layer keys on the canonical form, so this has
	// to agree with it.
	for _, u := range []string{
		"http://www.LoyalFans.com/tarataintontv",
		"https://www.loyalfans.com/tarataintontv/",
		"https://WWW.LOYALFANS.COM/tarataintontv",
	} {
		if !c.For(u).Active() {
			t.Errorf("%s was not recognised as a listed store", u)
		}
	}
}

// A creator with no aliases has nothing to rewrite; building a table entry for
// it would only add lookups that can never fire.
func TestCanonSkipsCreatorsWithoutAliases(t *testing.T) {
	c := NewCanon([]Creator{{
		Name:   "Kyla Keys",
		Stores: []Store{{URL: "https://www.clips4sale.com/studio/213129/kyla-keys"}},
	}})
	if c.For("https://www.clips4sale.com/studio/213129/kyla-keys").Active() {
		t.Error("a creator with no aliases produced a rewrite")
	}
}

// A storefront claimed by two creator files cannot be relabelled to either name
// without guessing. Load already warns; the rewrite simply declines.
func TestCanonDeclinesAStoreClaimedByTwoCreators(t *testing.T) {
	shared := "https://www.loyalfans.com/shared-shop"
	c := NewCanon([]Creator{
		{Name: "Ada Stone", Aliases: []string{"AdaStone TV"}, Stores: []Store{{URL: shared}}},
		{Name: "Mara Vance", Aliases: []string{"MaraVance TV"}, Stores: []Store{{URL: shared}}},
	})
	if c.For(shared).Active() {
		t.Error("a doubly-claimed store got a rewrite; that is a guess, not a fact")
	}
}

// The zero Canon is what every operator with no creators.d has.
func TestZeroCanonRewritesNothing(t *testing.T) {
	var c Canon
	sc := c.For("https://example.com")
	if sc.Active() {
		t.Fatal("zero Canon reported an active rewrite")
	}
	in := []string{"Someone", ""}
	got := sc.Apply(in)
	if len(got) != 2 || got[0] != "Someone" || got[1] != "" {
		t.Errorf("Apply = %v, want the input unchanged", got)
	}
	if got := NewCanon(nil).For("https://example.com"); got.Active() {
		t.Error("NewCanon(nil) produced an active rewrite")
	}
}

// A credit with nothing to key on is kept verbatim rather than dropped — the
// site did publish it, and discarding data is not this function's job.
func TestCanonKeepsUnkeyableNames(t *testing.T) {
	c := NewCanon([]Creator{taraTainton()})
	sc := c.For("https://www.loyalfans.com/tarataintontv")
	got := sc.Apply([]string{"???", "Tara Tainton TV"})
	if strings.Join(got, "|") != "???|Tara Tainton" {
		t.Errorf("Apply = %v, want [??? Tara Tainton]", got)
	}
}
