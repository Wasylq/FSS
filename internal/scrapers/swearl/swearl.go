// Package swearl registers the five sites of the Swearl / VR Bangers network,
// all backed by the shared swearlutil JSON content API scraper.
package swearl

import (
	"regexp"

	"github.com/Wasylq/FSS/internal/scrapers/swearlutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []swearlutil.SiteConfig{
	{
		ID:          "vrbangers",
		ContentHost: "content.vrbangers.com",
		SiteBase:    "https://vrbangers.com",
		Studio:      "VR Bangers",
		MatchRe:     regexp.MustCompile(`^https?://(?:www\.)?vrbangers\.com`),
	},
	{
		ID:          "vrbtrans",
		ContentHost: "content.vrbtrans.com",
		SiteBase:    "https://vrbtrans.com",
		Studio:      "VRB Trans",
		MatchRe:     regexp.MustCompile(`^https?://(?:www\.)?vrbtrans\.com`),
	},
	{
		ID:          "blowvr",
		ContentHost: "content.blowvr.com",
		SiteBase:    "https://blowvr.com",
		Studio:      "Blow VR",
		MatchRe:     regexp.MustCompile(`^https?://(?:www\.)?blowvr\.com`),
	},
	{
		ID:          "arporn",
		ContentHost: "content.arporn.com",
		SiteBase:    "https://arporn.com",
		Studio:      "AR Porn",
		MatchRe:     regexp.MustCompile(`^https?://(?:www\.)?arporn\.com`),
	},
	{
		ID:          "vrbgay",
		ContentHost: "content.vrbgay.com",
		SiteBase:    "https://vrbgay.com",
		Studio:      "VRB Gay",
		MatchRe:     regexp.MustCompile(`^https?://(?:www\.)?vrbgay\.com`),
	},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(swearlutil.New(cfg))
	}
}
