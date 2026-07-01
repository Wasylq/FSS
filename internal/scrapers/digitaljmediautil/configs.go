package digitaljmediautil

import "regexp"

func matchRe(host string) *regexp.Regexp {
	return regexp.MustCompile(`^https?://(?:www\.)?` + regexp.QuoteMeta(host))
}

// Configs returns the SiteConfig for every Digital J Media / JapanHDV network
// site, ready to be passed to New() and registered.
func Configs() []SiteConfig {
	return []SiteConfig{
		{
			SiteID:   "fellatiojapan",
			Studio:   "Fellatio Japan",
			Base:     "https://fellatiojapan.com",
			ListPath: "/en/samples",
			Patterns: []string{"fellatiojapan.com", "fellatiojapan.com/en/samples"},
			MatchRe:  matchRe("fellatiojapan.com"),
			parse:    parseFellatio,
		},
		{
			SiteID:   "cospuri",
			Studio:   "Cospuri",
			Base:     "https://cospuri.com",
			ListPath: "/samples",
			Patterns: []string{"cospuri.com", "cospuri.com/samples"},
			MatchRe:  matchRe("cospuri.com"),
			parse:    parseCospuri,
		},
		{
			SiteID:      "cutebutts",
			Studio:      "Cute Butts",
			Base:        "https://cutebutts.com",
			ListPath:    "/samples",
			Patterns:    []string{"cutebutts.com", "cutebutts.com/samples"},
			MatchRe:     matchRe("cutebutts.com"),
			parse:       parseCuteButts,
			detailParse: cuteButtsDetail,
		},
		{
			SiteID:      "cumbuffet",
			Studio:      "Cum Buffet",
			Base:        "https://cumbuffet.com",
			ListPath:    "/samples",
			Patterns:    []string{"cumbuffet.com", "cumbuffet.com/samples"},
			MatchRe:     matchRe("cumbuffet.com"),
			parse:       parseCumBuffet,
			detailParse: cumBuffetDetail,
		},
		{
			SiteID:   "legsjapan",
			Studio:   "Legs Japan",
			Base:     "https://legsjapan.com",
			ListPath: "/en/samples",
			Patterns: []string{"legsjapan.com", "legsjapan.com/en/samples"},
			MatchRe:  matchRe("legsjapan.com"),
			parse:    parseLegsJapan,
		},
		{
			SiteID:   "tokyofacefuck",
			Studio:   "Tokyo Face Fuck",
			Base:     "https://tokyofacefuck.com",
			ListPath: "/en/samples",
			Patterns: []string{"tokyofacefuck.com", "tokyofacefuck.com/en/samples"},
			MatchRe:  matchRe("tokyofacefuck.com"),
			parse:    parseTokyoFaceFuck,
		},
		{
			SiteID:   "handjobjapan",
			Studio:   "Handjob Japan",
			Base:     "https://handjobjapan.com",
			ListPath: "/en/samples",
			Patterns: []string{"handjobjapan.com", "handjobjapan.com/en/samples"},
			MatchRe:  matchRe("handjobjapan.com"),
			parse:    parseHandjobJapan,
		},
		{
			SiteID:   "spermmania",
			Studio:   "Sperm Mania",
			Base:     "https://spermmania.com",
			ListPath: "/samples",
			Patterns: []string{"spermmania.com", "spermmania.com/samples"},
			MatchRe:  matchRe("spermmania.com"),
			parse:    parseSpermMania,
		},
		{
			SiteID:   "transexjapan",
			Studio:   "Transex Japan",
			Base:     "https://transexjapan.com",
			ListPath: "/samples",
			Patterns: []string{"transexjapan.com", "transexjapan.com/samples"},
			MatchRe:  matchRe("transexjapan.com"),
			parse:    parseTransexJapan,
		},
		{
			SiteID:   "uralesbian",
			Studio:   "Ura Lesbian",
			Base:     "https://uralesbian.com",
			ListPath: "/en/samples",
			Patterns: []string{"uralesbian.com", "uralesbian.com/en/samples"},
			MatchRe:  matchRe("uralesbian.com"),
			parse:    parseUraLesbian,
		},
	}
}
