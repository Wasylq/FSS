package creators

import (
	"github.com/Anastylosis/FSS/output"
)

// Canon rewrites the performer names a storefront publishes into the spelling
// its creator is filed under.
//
// Storefronts routinely credit their own branding as the performer: LoyalFans
// lists "Tara Tainton TV", ManyVids lists "RedMilfRachelSteele". Stored
// verbatim, one person becomes several performers, and every cross-store view
// — `fss compare`, a Stash import, and `suggest`'s own shared-performer signal
// — fragments along with them.
//
// The rewrite is driven entirely by the `aliases:` a creator file already
// declares, and applies only to that creator's own stores. Both limits matter:
// the only strings it touches are ones a human wrote down as meaning this
// creator, on a storefront that same human listed as theirs. A co-star can
// never be caught by it, however their name is spelled.
//
// It deliberately holds no heuristic. Deciding that an unfamiliar name is
// really this creator needs evidence a single scene does not carry — which
// storefronts credit it, and nothing else — so that judgement lives in
// `fss creators suggest`, which has the whole library to reason over and writes
// its conclusion to the file for review.
type Canon struct {
	byStore map[string]StoreCanon
}

// StoreCanon is the rewrite for one storefront. The zero value is valid and
// rewrites nothing, so a caller with no creators defined needs no special case.
type StoreCanon struct {
	name string
	keys map[string]struct{}
}

// NewCanon builds the rewrite table from loaded creator files.
//
// A store listed under two creators is left out entirely rather than assigned
// to whichever file loaded first: Load already warns about the duplicate, and
// silently relabelling a shared storefront's performers to one of the two names
// would be a guess dressed as a fact.
func NewCanon(list []Creator) Canon {
	claims := map[string]int{}
	for _, c := range list {
		for _, u := range c.URLs() {
			claims[output.CanonicalStudioURL(u)]++
		}
	}

	byStore := map[string]StoreCanon{}
	for _, c := range list {
		keys := map[string]struct{}{}
		for _, k := range c.Keys() {
			keys[k] = struct{}{}
		}
		// The name alone is not worth a table entry: rewriting a name to
		// itself is a no-op, and an entry that can never fire only costs
		// lookups.
		if len(keys) < 2 {
			continue
		}
		for _, u := range c.URLs() {
			key := output.CanonicalStudioURL(u)
			if claims[key] > 1 {
				continue
			}
			byStore[key] = StoreCanon{name: c.Name, keys: keys}
		}
	}
	if len(byStore) == 0 {
		return Canon{}
	}
	return Canon{byStore: byStore}
}

// For returns the rewrite that applies to one storefront. The result is safe to
// use whether or not the URL belongs to a creator.
func (c Canon) For(studioURL string) StoreCanon {
	if c.byStore == nil {
		return StoreCanon{}
	}
	return c.byStore[output.CanonicalStudioURL(studioURL)]
}

// Active reports whether this storefront has any rewrite to apply.
func (s StoreCanon) Active() bool { return s.name != "" && len(s.keys) > 0 }

// Name is the creator's canonical spelling.
func (s StoreCanon) Name() string { return s.name }

// Apply rewrites a scene's performer list.
//
// Names that key to one of the creator's declared spellings become the creator
// name; everything else is passed through untouched. The result is deduplicated
// by key, because a scene crediting both "Tara Tainton" and "Tara Tainton TV"
// collapses to one person and must not be stored as two.
//
// The input slice is never modified.
func (s StoreCanon) Apply(names []string) []string {
	if !s.Active() || len(names) == 0 {
		return names
	}

	out := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		k := Key(n)
		if k == "" {
			// No letters or digits to key on — keep it verbatim rather than
			// dropping something the site did publish.
			out = append(out, n)
			continue
		}
		if _, ok := s.keys[k]; ok {
			n, k = s.name, Key(s.name)
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, n)
	}
	return out
}
