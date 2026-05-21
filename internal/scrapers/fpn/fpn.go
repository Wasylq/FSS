package fpn

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/fpnutil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []fpnutil.SiteConfig{
	// Hub — aggregates all subsites
	{SiteID: "fullpornnetwork", Domain: "fullpornnetwork.com", SiteBase: "https://fullpornnetwork.com", StudioName: "Full Porn Network"},

	// Standalone subsites (own domain, not redirecting)
	{SiteID: "abbiemaley", Domain: "abbiemaley.com", SiteBase: "https://abbiemaley.com", StudioName: "Abbie Maley"},
	{SiteID: "analamateur", Domain: "analamateur.com", SiteBase: "https://analamateur.com", StudioName: "Anal Amateur"},
	{SiteID: "analbbc", Domain: "analbbc.com", SiteBase: "https://analbbc.com", StudioName: "AnalBBC"},
	{SiteID: "analized", Domain: "analized.com", SiteBase: "https://analized.com", StudioName: "Analized"},
	{SiteID: "badbrotherpov", Domain: "badbrotherpov.com", SiteBase: "https://badbrotherpov.com", StudioName: "Bad Brother POV"},
	{SiteID: "baddaddypov", Domain: "baddaddypov.com", SiteBase: "https://baddaddypov.com", StudioName: "Bad Daddy POV"},
	{SiteID: "badfamilypov", Domain: "badfamilypov.com", SiteBase: "https://badfamilypov.com", StudioName: "Bad Family POV"},
	{SiteID: "badmommypov", Domain: "badmommypov.com", SiteBase: "https://badmommypov.com", StudioName: "Bad Mommy POV"},
	{SiteID: "brokensluts", Domain: "brokensluts.net", SiteBase: "https://brokensluts.net", StudioName: "Broken Sluts"},
	{SiteID: "cumdumpsterteens", Domain: "cumdumpsterteens.com", SiteBase: "https://cumdumpsterteens.com", StudioName: "Cum Dumpster Teens"},
	{SiteID: "dtfsluts", Domain: "dtfsluts.com", SiteBase: "https://dtfsluts.com", StudioName: "DTF Sluts"},
	{SiteID: "jamesdeen", Domain: "jamesdeen.com", SiteBase: "https://jamesdeen.com", StudioName: "James Deen"},
	{SiteID: "pornforce", Domain: "pornforce.com", SiteBase: "https://pornforce.com", StudioName: "Porn Force"},
	{SiteID: "porkvendors", Domain: "porkvendors.com", SiteBase: "https://porkvendors.com", StudioName: "Pork Vendors"},
	{SiteID: "publicsexdate", Domain: "publicsexdate.com", SiteBase: "https://publicsexdate.com", StudioName: "Public Sex Date"},
	{SiteID: "slutinspection", Domain: "slutinspection.com", SiteBase: "https://slutinspection.com", StudioName: "Slut Inspection"},
	{SiteID: "sluttybbws", Domain: "sluttybbws.com", SiteBase: "https://sluttybbws.com", StudioName: "Slutty BBWs"},
	{SiteID: "teasingandpleasing", Domain: "teasingandpleasing.com", SiteBase: "https://teasingandpleasing.com", StudioName: "Teasing and Pleasing"},
	{SiteID: "teenageanalsluts", Domain: "teenageanalsluts.com", SiteBase: "https://teenageanalsluts.com", StudioName: "Teenage Anal Sluts"},
	{SiteID: "teenagetryouts", Domain: "teenagetryouts.com", SiteBase: "https://teenagetryouts.com", StudioName: "Teenage Tryouts"},
	{SiteID: "yourmomdoesporn", Domain: "yourmomdoesporn.com", SiteBase: "https://yourmomdoesporn.com", StudioName: "Your Mom Does Porn"},
}

type siteScraper struct {
	config  fpnutil.SiteConfig
	matchRe *regexp.Regexp
	inner   *fpnutil.Scraper
}

func newSiteScraper(cfg fpnutil.SiteConfig) *siteScraper {
	re := regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(?:/|$)`, strings.ReplaceAll(cfg.Domain, ".", `\.`)))
	return &siteScraper{
		config:  cfg,
		matchRe: re,
		inner:   fpnutil.NewScraper(cfg),
	}
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newSiteScraper(cfg))
	}
}

func (s *siteScraper) ID() string { return s.config.SiteID }

func (s *siteScraper) Patterns() []string {
	return []string{
		s.config.Domain,
		s.config.Domain + "/models/{name}.html",
		s.config.Domain + "/porn-categories/{slug}",
		s.config.Domain + "/channels/{slug}",
	}
}

func (s *siteScraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.inner.Run(ctx, studioURL, opts, out)
	return out, nil
}
