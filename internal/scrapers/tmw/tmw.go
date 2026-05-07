package tmw

import (
	"github.com/Wasylq/FSS/internal/scrapers/tmwutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []tmwutil.SiteConfig{
	{SiteID: "tmw-18firstsex", Slug: "18firstsex", Domain: "18firstsex.com", StudioName: "18 First Sex"},
	{SiteID: "tmw-aboutgirlslove", Slug: "aboutgirlslove", Domain: "aboutgirlslove.com", StudioName: "About Girls Love"},
	{SiteID: "tmw-anal-angels", Slug: "anal-angels", Domain: "anal-angels.com", StudioName: "Anal Angels"},
	{SiteID: "tmw-anal-beauty", Slug: "anal-beauty", Domain: "anal-beauty.com", StudioName: "Anal-Beauty"},
	{SiteID: "tmw-atmovs", Slug: "atmovs", Domain: "atmovs.com", StudioName: "ATMovs"},
	{SiteID: "tmw-beauty4k", Slug: "beauty4k", Domain: "beauty4k.com", StudioName: "Beauty 4K"},
	{SiteID: "tmw-beauty-angels", Slug: "beauty-angels", Domain: "beauty-angels.com", StudioName: "Beauty Angels"},
	{SiteID: "tmw-creampie-angels", Slug: "creampie-angels", Domain: "creampie-angels.com", StudioName: "Creampie Angels"},
	{SiteID: "tmw-dirty-coach", Slug: "dirty-coach", Domain: "dirty-coach.com", StudioName: "Dirty Coach"},
	{SiteID: "tmw-dirty-doctor", Slug: "dirty-doctor", Domain: "dirty-doctor.com", StudioName: "Dirty Doctor"},
	{SiteID: "tmw-exgfbox", Slug: "exgfbox", Domain: "exgfbox.com", StudioName: "Ex GF Box"},
	{SiteID: "tmw-firstbgg", Slug: "firstbgg", Domain: "firstbgg.com", StudioName: "First BGG"},
	{SiteID: "tmw-fuckstudies", Slug: "fuckstudies", Domain: "fuckstudies.com", StudioName: "Fuck Studies"},
	{SiteID: "tmw-gag-n-gape", Slug: "gag-n-gape", Domain: "gag-n-gape.com", StudioName: "Gag-n-Gape"},
	{SiteID: "tmw-hometeenvids", Slug: "hometeenvids", Domain: "hometeenvids.com", StudioName: "Home Teen Vids"},
	{SiteID: "tmw-hometoyteens", Slug: "hometoyteens", Domain: "hometoyteens.com", StudioName: "Home Toy Teens"},
	{SiteID: "tmw-lollyhardcore", Slug: "lollyhardcore", Domain: "lollyhardcore.com", StudioName: "Lolly Hardcore"},
	{SiteID: "tmw-nubilegirlshd", Slug: "nubilegirlshd", Domain: "nubilegirlshd.com", StudioName: "Nubile Girls HD"},
	{SiteID: "tmw-nylonsx", Slug: "nylonsx", Domain: "nylonsx.com", StudioName: "NylonsX"},
	{SiteID: "tmw-ohmyholes", Slug: "ohmyholes", Domain: "ohmyholes.com", StudioName: "Oh! My Holes"},
	{SiteID: "tmw-old-n-young", Slug: "old-n-young", Domain: "old-n-young.com", StudioName: "Old-n-Young"},
	{SiteID: "tmw-privateteenvideo", Slug: "privateteenvideo", Domain: "privateteenvideo.com", StudioName: "Private Teen Video"},
	{SiteID: "tmw-rawcouples", Slug: "rawcouples", Domain: "rawcouples.com", StudioName: "Raw Couples"},
	{SiteID: "tmw-soloteengirls", Slug: "soloteengirls", Domain: "soloteengirls.net", StudioName: "Solo Teen Girls"},
	{SiteID: "tmw-squirtingvirgin", Slug: "squirtingvirgin", Domain: "squirtingvirgin.com", StudioName: "Squirting Virgin"},
	{SiteID: "tmw-teensexmania", Slug: "teensexmania", Domain: "teensexmania.com", StudioName: "Teen Sex Mania"},
	{SiteID: "tmw-teensexmovs", Slug: "teensexmovs", Domain: "teensexmovs.com", StudioName: "Teen Sex Movs"},
	{SiteID: "tmw-teenstarsonly", Slug: "teenstarsonly", Domain: "teenstarsonly.com", StudioName: "Teen Stars Only"},
	{SiteID: "tmw-teens3some", Slug: "teens3some", Domain: "teens3some.com", StudioName: "Teens3Some"},
	{SiteID: "tmw-tmwpov", Slug: "tmwpov", Domain: "tmwpov.com", StudioName: "TmwPOV"},
	{SiteID: "tmw-tmwvrnet", Slug: "tmwvrnet", Domain: "tmwvrnet.com", StudioName: "TmwVRnet"},
	{SiteID: "tmw-trickymasseur", Slug: "trickymasseur", Domain: "trickymasseur.com", StudioName: "Tricky Masseur"},
	{SiteID: "tmw-vogov", Slug: "vogov", Domain: "vogov.com", StudioName: "VogoV"},
	{SiteID: "tmw-watchmefucked", Slug: "watchmefucked", Domain: "watchmefucked.com", StudioName: "Watch Me Fucked"},
	{SiteID: "tmw-wow-orgasms", Slug: "wow-orgasms", Domain: "wow-orgasms.com", StudioName: "WOW Orgasms"},
	{SiteID: "tmw-x-angels", Slug: "x-angels", Domain: "x-angels.com", StudioName: "X-Angels"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(tmwutil.NewScraper(cfg))
	}
}
