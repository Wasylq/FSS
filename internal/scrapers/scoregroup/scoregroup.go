package scoregroup

import (
	"context"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/scoregrouputil"
	"github.com/Wasylq/FSS/scraper"
)

var sites = []scoregrouputil.SiteConfig{
	{SiteID: "18eighteen", SiteBase: "https://www.18eighteen.com", StudioName: "18Eighteen", VideosPath: "/xxx-teen-videos/", ModelsPath: "/teen-babes/"},
	{SiteID: "40somethingmag", SiteBase: "https://www.40somethingmag.com", StudioName: "40 Something Mag", VideosPath: "/xxx-mature-videos/", ModelsPath: "/xxx-mature-models/"},
	{SiteID: "50plusmilfs", SiteBase: "https://www.50plusmilfs.com", StudioName: "50 Plus MILFs", VideosPath: "/xxx-milf-videos/", ModelsPath: "/xxx-milf-models/"},
	{SiteID: "60plusmilfs", SiteBase: "https://www.60plusmilfs.com", StudioName: "60 Plus MILFs", VideosPath: "/xxx-granny-videos/", ModelsPath: "/xxx-granny-models/"},
	{SiteID: "analqts", SiteBase: "https://www.analqts.com", StudioName: "Anal QTs", VideosPath: "/anal-sex-videos/", ModelsPath: "/anal-porn-stars/"},
	{SiteID: "ashleysageellison", SiteBase: "https://www.ashleysageellison.com", StudioName: "Ashley Sage Ellison", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "asiancoochies", SiteBase: "https://www.asiancoochies.com", StudioName: "Asian Coochies", VideosPath: "/asian-porn-videos/", ModelsPath: "/asian-porn-stars/"},
	{SiteID: "autumn-jade", SiteBase: "https://www.autumn-jade.com", StudioName: "Autumn Jade", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bigboobalexya", SiteBase: "https://www.bigboobalexya.com", StudioName: "Big Boob Alexya", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bigboobbundle", SiteBase: "https://www.bigboobbundle.com", StudioName: "Big Boob Bundle", VideosPath: "/videos/", ModelsPath: "/big-tit-babes/"},
	{SiteID: "bigboobdaria", SiteBase: "https://www.bigboobdaria.com", StudioName: "Big Boob Daria", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bigboobspov", SiteBase: "https://www.bigboobspov.com", StudioName: "Big Boobs POV", VideosPath: "/big-boob-videos/", ModelsPath: "/big-boob-models/"},
	{SiteID: "bigboobvanessay", SiteBase: "https://www.bigboobvanessay.com", StudioName: "Big Boob Vanessa Y", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bigtitangelawhite", SiteBase: "https://www.bigtitangelawhite.com", StudioName: "Big Tit Angela White", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bigtithitomi", SiteBase: "https://www.bigtithitomi.com", StudioName: "Big Tit Hitomi", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bigtithooker", SiteBase: "https://www.bigtithooker.com", StudioName: "Big Tit Hooker", VideosPath: "/big-boob-videos/", ModelsPath: "/big-boob-models/"},
	{SiteID: "bigtitkatiethornton", SiteBase: "https://www.bigtitkatiethornton.com", StudioName: "Big Tit Katie Thornton", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bigtitterrynova", SiteBase: "https://www.bigtitterrynova.com", StudioName: "Big Tit Terry Nova", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bigtittymaserati", SiteBase: "https://www.bigtittymaserati.com", StudioName: "Big Titty Maserati", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bigtitvenera", SiteBase: "https://www.bigtitvenera.com", StudioName: "Big Tit Venera", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bonedathome", SiteBase: "https://www.bonedathome.com", StudioName: "Boned At Home", VideosPath: "/amateur-videos/", ModelsPath: "/amateur-girls/"},
	{SiteID: "bootyliciousmag", SiteBase: "https://www.bootyliciousmag.com", StudioName: "Bootylicious Mag", VideosPath: "/big-booty-videos/", ModelsPath: "/big-booty-girls/"},
	{SiteID: "bustyangelique", SiteBase: "https://www.bustyangelique.com", StudioName: "Busty Angelique", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bustyarianna", SiteBase: "https://www.bustyarianna.com", StudioName: "Busty Arianna", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bustydustystash", SiteBase: "https://www.bustydustystash.com", StudioName: "Busty Dusty Stash", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bustyinescudna", SiteBase: "https://www.bustyinescudna.com", StudioName: "Busty Ines Cudna", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bustykellykay", SiteBase: "https://www.bustykellykay.com", StudioName: "Busty Kelly Kay", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bustykerrymarie", SiteBase: "https://www.bustykerrymarie.com", StudioName: "Busty Kerry Marie", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bustylezzies", SiteBase: "https://www.bustylezzies.com", StudioName: "Busty Lezzies", VideosPath: "/lesbian-videos/", ModelsPath: "/big-tit-models/"},
	{SiteID: "bustymerilyn", SiteBase: "https://www.bustymerilyn.com", StudioName: "Busty Merilyn", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "bustyoldsluts", SiteBase: "https://www.bustyoldsluts.com", StudioName: "Busty Old Sluts", VideosPath: "/big-tit-scenes/", ModelsPath: ""},
	{SiteID: "chicksonblackdicks", SiteBase: "https://www.chicksonblackdicks.com", StudioName: "Chicks On Black Dicks", VideosPath: "/interracial-porn-videos/", ModelsPath: "/interracial-porn-stars/"},
	{SiteID: "chloesworld", SiteBase: "https://www.chloesworld.com", StudioName: "Chloe's World", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "christymarks", SiteBase: "https://www.christymarks.com", StudioName: "Christy Marks", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "cock4stepmom", SiteBase: "https://www.cock4stepmom.com", StudioName: "Cock 4 Stepmom", VideosPath: "/stepmom-xxx-scenes/", ModelsPath: ""},
	{SiteID: "codivorexxx", SiteBase: "https://www.codivorexxx.com", StudioName: "Codi Vore XXX", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "creampieforgranny", SiteBase: "https://www.creampieforgranny.com", StudioName: "Creampie For Granny", VideosPath: "/milf-creampie-scenes/", ModelsPath: ""},
	{SiteID: "daylenerio", SiteBase: "https://www.daylenerio.com", StudioName: "Daylene Rio", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "desiraesworld", SiteBase: "https://www.desiraesworld.com", StudioName: "Desirae's World", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "ebonythots", SiteBase: "https://www.ebonythots.com", StudioName: "Ebony Thots", VideosPath: "/black-girl-videos/", ModelsPath: "/black-porn-stars/"},
	{SiteID: "evanottyvideos", SiteBase: "https://www.evanottyvideos.com", StudioName: "Eva Notty Videos", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "feedherfuckher", SiteBase: "https://www.feedherfuckher.com", StudioName: "Feed Her Fuck Her", VideosPath: "/bbw-videos/", ModelsPath: "/bbw-models/"},
	{SiteID: "flatandfuckedmilfs", SiteBase: "https://www.flatandfuckedmilfs.com", StudioName: "Flat And Fucked MILFs", VideosPath: "/xxx-milf-scenes/", ModelsPath: ""},
	{SiteID: "grannygetsafacial", SiteBase: "https://www.grannygetsafacial.com", StudioName: "Granny Gets A Facial", VideosPath: "/granny-facial-scenes/", ModelsPath: ""},
	{SiteID: "grannylovesbbc", SiteBase: "https://www.grannylovesbbc.com", StudioName: "Granny Loves BBC", VideosPath: "/bbc-granny-scenes/", ModelsPath: ""},
	{SiteID: "grannylovesyoungcock", SiteBase: "https://www.grannylovesyoungcock.com", StudioName: "Granny Loves Young Cock", VideosPath: "/xxx-granny-scenes/", ModelsPath: ""},
	{SiteID: "hairycoochies", SiteBase: "https://www.hairycoochies.com", StudioName: "Hairy Coochies", VideosPath: "/hairy-pussy-videos/", ModelsPath: "/hirsute-girls/"},
	{SiteID: "homealonemilfs", SiteBase: "https://www.homealonemilfs.com", StudioName: "Home Alone MILFs", VideosPath: "/milf-scenes/", ModelsPath: ""},
	{SiteID: "hornyasianmilfs", SiteBase: "https://www.hornyasianmilfs.com", StudioName: "Horny Asian MILFs", VideosPath: "/asian-milf-scenes/", ModelsPath: ""},
	{SiteID: "ibonedyourmom", SiteBase: "https://www.ibonedyourmom.com", StudioName: "I Boned Your Mom", VideosPath: "/mom-fucking-scenes/", ModelsPath: ""},
	{SiteID: "ifuckedtheboss", SiteBase: "https://www.ifuckedtheboss.com", StudioName: "I Fucked The Boss", VideosPath: "/working-girl-scenes/", ModelsPath: ""},
	{SiteID: "joanabliss", SiteBase: "https://www.joanabliss.com", StudioName: "Joana Bliss", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "karinahart", SiteBase: "https://www.karinahart.com", StudioName: "Karina Hart", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "latinacoochies", SiteBase: "https://www.latinacoochies.com", StudioName: "Latina Coochies", VideosPath: "/latina-porn-videos/", ModelsPath: "/latina-porn-stars/"},
	{SiteID: "latinmommas", SiteBase: "https://www.latinmommas.com", StudioName: "Latin Mommas", VideosPath: "/latina-milf-scenes/", ModelsPath: ""},
	{SiteID: "leannecrowvideos", SiteBase: "https://www.leannecrowvideos.com", StudioName: "Leanne Crow Videos", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "legsex", SiteBase: "https://www.legsex.com", StudioName: "Leg Sex", VideosPath: "/foot-fetish-videos/", ModelsPath: "/foot-models/"},
	{SiteID: "linseysworld", SiteBase: "https://www.linseysworld.com", StudioName: "Linsey's World", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "megatitsminka", SiteBase: "https://www.megatitsminka.com", StudioName: "Mega Tits Minka", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "mickybells", SiteBase: "https://www.mickybells.com", StudioName: "Micky Bells", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "milfbundle", SiteBase: "https://www.milfbundle.com", StudioName: "MILF Bundle", VideosPath: "/milf-scenes/", ModelsPath: "/milf-models/"},
	{SiteID: "milfthreesomes", SiteBase: "https://www.milfthreesomes.com", StudioName: "MILF Threesomes", VideosPath: "/milf-group-scenes/", ModelsPath: ""},
	{SiteID: "milftugs", SiteBase: "https://www.milftugs.com", StudioName: "MILF Tugs", VideosPath: "/milf-handjob-scenes/", ModelsPath: ""},
	{SiteID: "millymarks", SiteBase: "https://www.millymarks.com", StudioName: "Milly Marks", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "mommystoytime", SiteBase: "https://www.mommystoytime.com", StudioName: "Mommy's Toy Time", VideosPath: "/milf-sex-toy-scenes/", ModelsPath: ""},
	{SiteID: "nataliefiore", SiteBase: "https://www.nataliefiore.com", StudioName: "Natalie Fiore", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "naughtyfootjobs", SiteBase: "https://www.naughtyfootjobs.com", StudioName: "Naughty Footjobs", VideosPath: "/foot-job-videos/", ModelsPath: "/foot-models/"},
	{SiteID: "naughtymag", SiteBase: "https://www.naughtymag.com", StudioName: "Naughty Mag", VideosPath: "/amateur-videos/", ModelsPath: "/amateur-girls/"},
	{SiteID: "naughtytugs", SiteBase: "https://www.naughtytugs.com", StudioName: "Naughty Tugs", VideosPath: "/hand-job-videos/", ModelsPath: "/hand-job-models/"},
	{SiteID: "nicolepeters", SiteBase: "https://www.nicolepeters.com", StudioName: "Nicole Peters", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "oldhornymilfs", SiteBase: "https://www.oldhornymilfs.com", StudioName: "Old Horny MILFs", VideosPath: "/milf-scenes/", ModelsPath: ""},
	{SiteID: "pickinguppussy", SiteBase: "https://www.pickinguppussy.com", StudioName: "Picking Up Pussy", VideosPath: "/xxx-teen-videos/", ModelsPath: "/teen-babes/"},
	{SiteID: "pornloser", SiteBase: "https://www.pornloser.com", StudioName: "Porn Loser", VideosPath: "/amateur-videos/", ModelsPath: "/amateur-girls/"},
	{SiteID: "pornmegaload", SiteBase: "https://www.pornmegaload.com", StudioName: "Porn Mega Load", VideosPath: "/hd-porn-scenes/", ModelsPath: "/porn-models/"},
	{SiteID: "reneerossvideos", SiteBase: "https://www.reneerossvideos.com", StudioName: "Renee Ross Videos", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "roxired", SiteBase: "https://www.roxired.com", StudioName: "Roxi Red", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "sarennasworld", SiteBase: "https://www.sarennasworld.com", StudioName: "SaRenna's World", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "scoreclassics", SiteBase: "https://www.scoreclassics.com", StudioName: "Score Classics", VideosPath: "/classic-boob-videos/", ModelsPath: "/classic-boob-models/"},
	{SiteID: "scoreland", SiteBase: "https://www.scoreland.com", StudioName: "Scoreland", VideosPath: "/big-boob-videos/", ModelsPath: "/big-boob-models/"},
	{SiteID: "scoreland2", SiteBase: "https://www.scoreland2.com", StudioName: "Scoreland 2", VideosPath: "/big-boob-scenes/", ModelsPath: "/big-boob-models/"},
	{SiteID: "scorevideos", SiteBase: "https://www.scorevideos.com", StudioName: "Score Videos", VideosPath: "/porn-videos/", ModelsPath: "/porn-models/"},
	{SiteID: "sharizelvideos", SiteBase: "https://www.sharizelvideos.com", StudioName: "Sha Rizel Videos", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "silversluts", SiteBase: "https://www.silversluts.com", StudioName: "Silver Sluts", VideosPath: "/granny-scenes/", ModelsPath: ""},
	{SiteID: "stacyvandenbergboobs", SiteBase: "https://www.stacyvandenbergboobs.com", StudioName: "Stacy Vandenberg Boobs", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "tawny-peaks", SiteBase: "https://www.tawny-peaks.com", StudioName: "Tawny Peaks", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "tiffany-towers", SiteBase: "https://www.tiffany-towers.com", StudioName: "Tiffany Towers", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "titsandtugs", SiteBase: "https://www.titsandtugs.com", StudioName: "Tits And Tugs", VideosPath: "/big-boob-videos/", ModelsPath: "/big-boob-models/"},
	{SiteID: "tnatryouts", SiteBase: "https://www.tnatryouts.com", StudioName: "TNA Tryouts", VideosPath: "/xxx-teen-videos/", ModelsPath: "/teen-babes/"},
	{SiteID: "valoryirene", SiteBase: "https://www.valoryirene.com", StudioName: "Valory Irene", VideosPath: "/videos/", ModelsPath: ""},
	{SiteID: "xlgirls", SiteBase: "https://www.xlgirls.com", StudioName: "XL Girls", VideosPath: "/bbw-videos/", ModelsPath: "/bbw-models/"},
	{SiteID: "yourmomlovesanal", SiteBase: "https://www.yourmomlovesanal.com", StudioName: "Your Mom Loves Anal", VideosPath: "/anal-milf-scenes/", ModelsPath: ""},
	{SiteID: "yourmomsgotbigtits", SiteBase: "https://www.yourmomsgotbigtits.com", StudioName: "Your Mom's Got Big Tits", VideosPath: "/big-tit-mom-scenes/", ModelsPath: ""},
	{SiteID: "yourwifemymeat", SiteBase: "https://www.yourwifemymeat.com", StudioName: "Your Wife My Meat", VideosPath: "/wife-fucking-scenes/", ModelsPath: ""},
}

func init() {
	for _, cfg := range sites {
		scraper.Register(newScraper(cfg))
	}
}

type siteScraper struct {
	sg      *scoregrouputil.Scraper
	config  scoregrouputil.SiteConfig
	matchRe *regexp.Regexp
}

func newScraper(cfg scoregrouputil.SiteConfig) *siteScraper {
	domain := strings.TrimPrefix(cfg.SiteBase, "https://www.")
	escaped := regexp.QuoteMeta(domain)
	re := regexp.MustCompile(`^https?://(?:www\.)?` + escaped)
	return &siteScraper{
		sg:      scoregrouputil.NewScraper(cfg),
		config:  cfg,
		matchRe: re,
	}
}

func (s *siteScraper) ID() string { return s.config.SiteID }

func (s *siteScraper) Patterns() []string {
	domain := strings.TrimPrefix(s.config.SiteBase, "https://www.")
	patterns := []string{domain}
	if s.config.ModelsPath != "" {
		patterns = append(patterns, domain+s.config.ModelsPath+"{name}/{id}")
	}
	return patterns
}

func (s *siteScraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.sg.Run(ctx, studioURL, opts, out)
	return out, nil
}
