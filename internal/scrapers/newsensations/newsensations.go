package newsensations

import (
	"github.com/Wasylq/FSS/internal/scrapers/newsensationsutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []newsensationsutil.SiteConfig{
	{SiteID: "newsensations", Domain: "newsensations.com", SiteBase: "https://www.newsensations.com", TourPrefix: "tour_ns", StudioName: "New Sensations"},
	{SiteID: "familyxxx", Domain: "familyxxx.com", SiteBase: "https://familyxxx.com", TourPrefix: "tour_famxxx", StudioName: "FamilyXXX"},
	{SiteID: "hotwifexxx", Domain: "hotwifexxx.com", SiteBase: "https://www.hotwifexxx.com", TourPrefix: "tour_hwxxx", StudioName: "Hot Wife XXX"},
	{SiteID: "girlgirlxxx", Domain: "girlgirlxxx.com", SiteBase: "https://www.girlgirlxxx.com", TourPrefix: "tour_girlgirlxxx", StudioName: "Girl Girl XXX"},
	{SiteID: "freshoutofhighschool", Domain: "freshouttahighschool.com", SiteBase: "https://freshouttahighschool.com", TourPrefix: "tour_fohs", StudioName: "Fresh Outta High School", AltDomains: []string{"freshoutofhighschool.com"}},
	{SiteID: "thelesbianexperience", Domain: "thelesbianexperience.com", SiteBase: "https://thelesbianexperience.com", TourPrefix: "tour_tle", StudioName: "The Lesbian Experience"},
	{SiteID: "shanedieselxxx", Domain: "shanedieselxxx.com", SiteBase: "https://www.shanedieselxxx.com", TourPrefix: "tour_sdxxx", StudioName: "Shane Diesel XXX"},
	{SiteID: "thetalesfromtheedge", Domain: "thetalesfromtheedge.com", SiteBase: "https://www.thetalesfromtheedge.com", TourPrefix: "tour_ttfte", StudioName: "Tales From The Edge"},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newsensationsutil.New(cfg))
	}
}
