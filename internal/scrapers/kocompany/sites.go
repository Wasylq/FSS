package kocompany

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites enumerates every KO Company sub-label per stashdb. Each
// `MatchRe` accepts both the canonical `ko-video.com/products/list.php?{filter}={N}`
// URL and the stashdb-alternate `ko-tube.com/ranking/label/01-XX/NAME`
// form so users can paste either. All scrapes route through the
// scraper's own `listingURL()` (which always hits ko-video.com).
var sites = []SiteConfig{
	{
		ID:       "kobeast",
		SiteName: "KO BEAST",
		LabelID:  3,
		Patterns: []string{
			"ko-video.com/products/list.php?label=3",
			"ko-tube.com/ranking/label/01-06/BEAST",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=3\b|ko-tube\.com/ranking/label/01-06/BEAST)`),
	},
	{
		ID:       "kobump",
		SiteName: "KO BUMP",
		LabelID:  21,
		Patterns: []string{
			"ko-video.com/products/list.php?label=21",
			"ko-tube.com/ranking/label/01-18/BUMP",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=21\b|ko-tube\.com/ranking/label/01-18/BUMP)`),
	},
	{
		ID:       "kodeep",
		SiteName: "KO DEEP",
		LabelID:  6,
		Patterns: []string{
			"ko-video.com/products/list.php?label=6",
			"ko-tube.com/ranking/label/01-08/DEEP",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=6\b|ko-tube\.com/ranking/label/01-08/DEEP)`),
	},
	{
		ID:       "koeast",
		SiteName: "KO EAST",
		MakerID:  10,
		Patterns: []string{
			"ko-video.com/products/list.php?maker=10",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?ko-video\.com/products/list\.php\?maker=10\b`),
	},
	{
		ID:       "koeros",
		SiteName: "KO EROS",
		LabelID:  5,
		Patterns: []string{
			"ko-video.com/products/list.php?label=5",
			"ko-tube.com/ranking/label/01-10/EROS",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=5\b|ko-tube\.com/ranking/label/01-10/EROS)`),
	},
	{
		ID:       "koindies",
		SiteName: "KO INDIES",
		LabelID:  8,
		Patterns: []string{
			"ko-video.com/products/list.php?label=8",
			"ko-tube.com/ranking/label/01-13/INDIES",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=8\b|ko-tube\.com/ranking/label/01-13/INDIES)`),
	},
	{
		ID:       "kojoker",
		SiteName: "KO JOKER",
		LabelID:  48,
		Patterns: []string{
			"ko-video.com/products/list.php?label=48",
			"ko-tube.com/ranking/label/01-26/JOKER",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=48\b|ko-tube\.com/ranking/label/01-26/JOKER)`),
	},
	{
		ID:       "kokuruu",
		SiteName: "KO KURUU",
		LabelID:  9,
		Patterns: []string{
			"ko-video.com/products/list.php?label=9",
			"ko-tube.com/ranking/label/01-11/KURUU",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=9\b|ko-tube\.com/ranking/label/01-11/KURUU)`),
	},
	{
		ID:       "koline",
		SiteName: "KO LINE",
		LabelID:  16,
		Patterns: []string{
			"ko-video.com/products/list.php?label=16",
			"ko-tube.com/ranking/label/01-15/LINE",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=16\b|ko-tube\.com/ranking/label/01-15/LINE)`),
	},
	{
		ID:       "kosuits",
		SiteName: "KO SUITS",
		LabelID:  43,
		Patterns: []string{
			"ko-video.com/products/list.php?label=43",
			"ko-tube.com/ranking/label/01-25/SUITS",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=43\b|ko-tube\.com/ranking/label/01-25/SUITS)`),
	},
	{
		// KO TUBE オリジナル Original — only the ko-tube URL is in
		// stashdb; no ko-video.com label number was provided, so this
		// entry is gated behind the ko-tube domain only and uses the
		// `tube` query argument (label=1 by convention on the ko-tube
		// hub).
		ID:       "kotuboriginal",
		SiteName: "KO TUBE オリジナル",
		LabelID:  1,
		Patterns: []string{
			"ko-tube.com/ranking/label/01-01/TUBE+オリジナル",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?ko-tube\.com/ranking/label/01-01/TUBE`),
	},
	{
		// KO どぴゅノンケ (dopyu-nonke).
		ID:       "kodopyunonke",
		SiteName: "KO どぴゅノンケ",
		LabelID:  56,
		Patterns: []string{
			"ko-video.com/products/list.php?label=56",
			"ko-tube.com/ranking/label/01-27/どぴゅノンケ",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=56\b|ko-tube\.com/ranking/label/01-27/)`),
	},
	{
		// KO ガチ撮り (gachi-dori).
		ID:       "kogachidori",
		SiteName: "KO ガチ撮り",
		LabelID:  38,
		Patterns: []string{
			"ko-video.com/products/list.php?label=38",
			"ko-tube.com/ranking/label/01-24/ガチ撮り",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=38\b|ko-tube\.com/ranking/label/01-24/)`),
	},
	{
		// KO 裸王-Raoh-.
		ID:       "koraoh",
		SiteName: "KO 裸王-Raoh-",
		LabelID:  36,
		Patterns: []string{
			"ko-video.com/products/list.php?label=36",
			"ko-tube.com/ranking/label/01-23/裸王-Raoh-",
		},
		MatchRe: regexp.MustCompile(`^https?://(?:www\.)?(?:ko-video\.com/products/list\.php\?label=36\b|ko-tube\.com/ranking/label/01-23/)`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(New(cfg))
	}
}
