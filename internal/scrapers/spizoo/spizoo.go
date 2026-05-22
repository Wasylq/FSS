package spizoo

import (
	"github.com/Wasylq/FSS/internal/scrapers/spizooutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []spizooutil.SiteConfig{
	{SiteID: "spizoo", Domain: "spizoo.com", StudioName: "Spizoo"},
	{SiteID: "creamher", Domain: "creamher.com", StudioName: "Cream Her"},
	{SiteID: "drdaddypov", Domain: "drdaddypov.com", StudioName: "Dr. Daddy POV"},
	{SiteID: "firstclasspov", Domain: "firstclasspov.com", StudioName: "First Class POV"},
	{SiteID: "gothgirlfriends", Domain: "gothgirlfriends.com", StudioName: "Goth Girlfriends"},
	{SiteID: "gothgirlfriendsvip", Domain: "gothgirlfriendsvip.com", StudioName: "Goth GirlfriendsVIP"},
	{SiteID: "intimatelesbians", Domain: "intimatelesbians.com", StudioName: "Intimate Lesbians"},
	{SiteID: "mrluckylife", Domain: "mrluckylife.com", StudioName: "Mr. LuckyLIFE"},
	{SiteID: "mrluckypov", Domain: "mrluckypov.com", StudioName: "Mr. LuckyPOV"},
	{SiteID: "mrluckyraw", Domain: "mrluckyraw.com", StudioName: "Mr. LuckyRaw"},
	{SiteID: "mrluckyvip", Domain: "mrluckyvip.com", StudioName: "Mr. LuckyVIP"},
	{SiteID: "pervertcollege", Domain: "pervertcollege.com", StudioName: "Pervert College"},
	{SiteID: "porngoespro", Domain: "porngoespro.com", StudioName: "Porn Goes Pro"},
	{SiteID: "rawattack", Domain: "rawattack.com", StudioName: "RawAttack"},
	{SiteID: "realsensual", Domain: "realsensual.com", StudioName: "Real Sensual"},
	{SiteID: "tagteampov", Domain: "tagteampov.com", StudioName: "TagTeamPOV"},
	{SiteID: "thestripperexperience", Domain: "thestripperexperience.com", StudioName: "The Stripper Experience"},
	{SiteID: "tiffanybrookesxxx", Domain: "tiffanybrookesxxx.com", StudioName: "TiffanyBrookesXXX"},
	{SiteID: "vlogxxx", Domain: "vlogxxx.com", StudioName: "Vlog XXX"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(spizooutil.New(cfg))
	}
}
