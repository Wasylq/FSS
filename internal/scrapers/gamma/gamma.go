package gamma

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Wasylq/FSS/internal/scrapers/gammautil"
	"github.com/Wasylq/FSS/scraper"
)

type siteConfig struct {
	SiteID      string
	Domain      string
	StudioName  string
	SiteName    string // Algolia availableOnSite filter; defaults to SiteID if empty
	RefererBase string // override for API key bootstrap (network scrapers)
	MatchRe     string // optional: override the default domain-based match regex
}

var sites = []siteConfig{
	// Adult Time segment — originals (more specific match, must be before adulttime)
	{"adulttimeoriginals", "adulttime.com", "Adult Time Originals", "adulttime", "", `^https?://(?:www\.)?adulttime\.com/en/(?:studio|channel)/adult-time(?:-originals)?(?:/|$)`},

	// Adult Time segment — full catalog (all content in the segment)
	{"adulttime", "adulttime.com", "", "", "", ""},

	// Adult Time segment — individual sites
	{"burningangel", "burningangel.com", "Burning Angel", "", "", ""},
	{"evilangel", "evilangel.com", "Evil Angel", "", "", ""},
	{"tsfactor", "tsfactor.com", "TS Factor", "tsfactor", "", ""},
	{"pansexualx", "pansexualx.com", "PansexualX", "pansexualx", "", ""},
	{"transgressivexxx", "transgressivexxx.com", "TransgressiveXXX", "transgressivexxx", "", ""},

	// Adult Time segment — Adult Time Originals sub-sites
	{"accidentalgangbang", "accidentalgangbang.com", "Accidental Gangbang", "", "", ""},
	{"ageandbeauty", "ageandbeauty.com", "Age and Beauty", "", "", ""},
	{"asmrfantasy", "asmrfantasy.com", "ASMR Fantasy", "", "", ""},
	{"bethecuck", "bethecuck.com", "Be the Cuck", "", "", ""},
	{"femalesubmission", "femalesubmission.com", "Female Submission", "", "", ""},
	{"femboyish", "femboyish.com", "Femboyish", "", "", ""},
	{"futasentaisquad", "futasentaisquad.com", "F.U.T.A Sentai Squad", "", "", ""},
	{"futaworld", "futaworld.com", "Futa World", "", "", ""},
	{"futuredarkly", "futuredarkly.com", "Future Darkly", "", "", ""},
	{"getupclose", "getupclose.com", "UP CLOSE", "", "", ""},
	{"girlcore", "girlcore.com", "Girlcore", "", "", ""},
	{"girlsunderarrest", "girlsunderarrest.com", "Girls Under Arrest", "", "", ""},
	{"hentaisexschool", "hentaisexschool.com", "Hentai Sex School", "", "", ""},
	{"heteroflexible", "heteroflexible.com", "HeteroFlexible", "", "", ""},
	{"isthisreal", "isthisreal.com", "Is This Real?!", "", "", ""},
	{"jerkbuddies", "jerk-buddies.com", "Jerk Buddies", "jerk-buddies", "", ""},
	{"ladygonzo", "ladygonzo.com", "Lady Gonzo", "", "", ""},
	{"lezbebad", "lezbebad.com", "Lez Be Bad", "", "", ""},
	{"moderndaysins", "moderndaysins.com", "Modern-Day Sins", "", "", ""},
	{"oopsie", "oopsie.com", "Oopsie!", "", "", ""},
	{"oopsieanimated", "oopsieanimated.com", "Oopsie! Animated", "", "", ""},
	{"prettydirty", "prettydirty.com", "Pretty Dirty", "", "", ""},
	{"sistertrick", "sistertrick.com", "Sister Trick", "", "", ""},
	{"transfixed", "transfixed.com", "Transfixed", "", "", ""},
	{"truelesbian", "truelesbian.com", "True Lesbian", "", "", ""},
	{"upclosevr", "upclosevr.com", "Up Close VR", "", "", ""},

	// Adult Time segment — standalone-domain sites
	{"joymii", "joymii.com", "JoyMii", "", "", ""},
	{"mixedx", "mixedx.com", "MixedX", "", "", ""},
	{"modeltime", "modeltime.com", "Model Time", "", "", ""},

	// Adult Time segment — Vivid network. All sub-domains are reskinned
	// vivid.com fronts that ship the same Algolia API key bootstrap
	// (segment:adulttime), but the keys are Referer-signed for vivid.com
	// — querying Algolia from the sub-domain's own Referer returns HTTP
	// 403. Set `RefererBase` to vivid.com so both the API-key bootstrap
	// fetch and the Algolia query use the parent's Referer header.
	// Each per-site `availableOnSite:{siteid}` filter narrows the
	// segment-wide pool to that brand. Vivid Celeb redirects to
	// vivid.com/en/videos/sites/vividclassic so the `vividclassic`
	// entry below covers it. TS Divas (`members.adulttime.com/en/channel/tsdivas`)
	// requires Adult Time member login. Vivid Alt (`vividalt.com`) is an
	// abandoned WordPress 2.8.5 blog (pre-REST-API, 2009) and is out of
	// scope.
	{"vivid", "vivid.com", "Vivid", "", "", ""},
	{"65inchhugeasses", "65inchhugeasses.com", "65 Inch Huge Asses", "", "https://www.vivid.com", ""},
	{"blackwhitefuckfest", "blackwhitefuckfest.com", "Black White Fuckfest", "", "https://www.vivid.com", ""},
	{"brandnewfaces", "brandnewfaces.com", "Brand New Faces", "", "https://www.vivid.com", ""},
	{"girlswhofuckgirls", "girlswhofuckgirls.com", "Girls Who Fuck Girls", "", "https://www.vivid.com", ""},
	{"momisamilf", "momisamilf.com", "Mom Is a Milf", "", "https://www.vivid.com", ""},
	{"nastystepfamily", "nastystepfamily.com", "Nasty Step Family", "", "https://www.vivid.com", ""},
	{"nineteen", "nineteen.com", "Nineteen", "", "https://www.vivid.com", ""},
	{"orgytrain", "orgytrain.com", "Orgy Train", "", "https://www.vivid.com", ""},
	{"petited", "petited.com", "Petited", "", "https://www.vivid.com", ""},
	{"thebrats", "thebrats.com", "The Brats", "", "https://www.vivid.com", ""},
	{"vividclassic", "vividclassic.com", "Vivid Classic", "", "https://www.vivid.com", ""},
	{"wheretheboysarent", "wheretheboysarent.com", "Where The Boys Aren't", "", "https://www.vivid.com", ""},

	// Adult Time segment — Devil's Film (Network) sub-sites
	{"devilsfilm", "devilsfilm.com", "Devil's Film", "", "", ""},
	{"devilsfilmparodies", "devilsfilmparodies.com", "Devil's Film Parodies", "", "", ""},
	{"devilsgangbangs", "devilsgangbangs.com", "Devil's Gangbangs", "", "", ""},
	{"devilstgirls", "devilstgirls.com", "Devil's Tgirls", "", "", ""},
	{"givemeteens", "givemeteens.com", "Give Me Teens", "", "", ""},
	{"hairyundies", "hairyundies.com", "Hairy Undies", "", "", ""},
	{"lesbianfactor", "lesbianfactor.com", "Lesbian Factor", "", "", ""},
	{"outofthefamily", "outofthefamily.com", "Out of the Family", "", "", ""},
	{"squirtalicious", "squirtalicious.com", "Squirtalicious", "", "", ""},

	// Adult Time segment — Fame Digital sub-sites
	{"famedigital", "famedigital.com", "Fame Digital", "", "", ""},
	{"bigfatcreampie", "bigfatcreampie.com", "Big Fat Creampie", "", "", ""},
	{"bushybushy", "bushybushy.com", "Bushy Bushy", "", "", ""},
	{"cumshotoasis", "cumshotoasis.com", "Cumshot Oasis", "", "", ""},
	{"currycreampie", "currycreampie.com", "Curry Creampie", "", "", ""},
	{"grannyghetto", "grannyghetto.com", "Granny Ghetto", "", "", ""},
	{"lowartfilms", "lowartfilms.com", "Low Art Films", "", "", ""},
	{"motherfuckerxxx", "motherfuckerxxx.com", "Motherfucker XXX", "", "", ""},
	{"myteenoasis", "myteenoasis.com", "My Teen Oasis", "", "", ""},
	{"povthis", "povthis.com", "POV This", "", "", ""},
	{"silverstonedvd", "silverstonedvd.com", "Silverstone DVD", "", "", ""},
	{"silviasaint", "silviasaint.com", "Silvia Saint", "", "", ""},
	{"terapatrick", "terapatrick.com", "Tera Patrick", "terapatrick", "https://www.famedigital.com", ""},
	{"transsexualroadtrip", "transsexualroadtrip.com", "Transsexual Roadtrip", "", "", ""},
	{"whiteghetto", "whiteghetto.com", "White Ghetto", "", "", ""},

	// Adult Time segment — Girlsway (Network) sub-sites
	{"girlsway", "girlsway.com", "Girlsway", "", "", ""},
	{"girlstryanal", "girlstryanal.com", "Girls Try Anal", "", "", ""},
	{"mommysgirl", "mommysgirl.com", "Mommy's Girl", "", "", ""},
	{"sextapelesbians", "sextapelesbians.com", "Sex Tape Lesbians", "", "", ""},
	{"squirtinglesbian", "squirtinglesbian.com", "Squirting Lesbian", "", "", ""},
	{"webyoung", "webyoung.com", "Web Young", "", "", ""},

	// Evil Angel Network segment (evilangelnetwork) — director-branded sub-sites
	{"buttman", "buttman.com", "Buttman", "buttman", "", ""},
	{"analtrixxx", "analtrixxx.com", "AnalTriXXX", "analtrixxx", "", ""},
	{"jonnidarkkoxxx", "jonnidarkkoxxx.com", "Jonni Darkko XXX", "jonnidarkkoxxx", "", ""},
	{"latexplaytime", "latexplaytime.com", "Latex Playtime", "latexplaytime", "", ""},
	{"transsexualangel", "transsexualangel.com", "Transsexual Angel", "transsexualangel", "", ""},
	{"filthykings", "filthykings.com", "Filthy Kings", "", "", ""},
	{"gangbangcreampie", "gangbangcreampie.com", "Gangbang Creampie", "", "", ""},
	{"girlfriendsfilms", "girlfriendsfilms.com", "Girlfriends Films", "", "", ""},
	{"gloryholesecrets", "gloryholesecrets.com", "Gloryhole Secrets", "", "", ""},
	{"lethalhardcore", "lethalhardcore.com", "Lethal Hardcore", "", "", ""},
	{"mommyblowsbest", "mommyblowsbest.com", "Mommy Blows Best", "", "", ""},
	{"puretaboo", "puretaboo.com", "Pure Taboo", "", "", ""},
	{"roccosiffredi", "roccosiffredi.com", "Rocco Siffredi", "", "", ""},
	{"tabooheat", "tabooheat.com", "Taboo Heat", "", "", ""},
	{"wicked", "wicked.com", "Wicked", "", "", ""},

	// Dogfart segment (dfxtra) — 17 subsites under dogfartnetwork.com
	{"dogfartnetwork", "dogfartnetwork.com", "", "", "", ""},

	// OpenLife segment — 12 subsites under openlife.com
	{"openlife", "openlife.com", "", "", "", ""},

	// Zero Tolerance Films segment — network hub (all content in the segment)
	{"zerotolerancefilms", "zerotolerancefilms.com", "", "", "", ""},

	// Zero Tolerance Films segment — individual sites with own domains
	{"3rddegreefilms", "3rddegreefilms.com", "3rd Degree Films", "3rddegreefilms", "", ""},
	{"diabolic", "diabolic.com", "Diabolic", "diabolic", "", ""},

	// Addicted 2 Girls segment (own segment, under Zero Tolerance Films tree)
	{"addicted2girls", "addicted2girls.com", "Addicted 2 Girls", "", "", ""},

	// BiPhoria segment (own segment, under Zero Tolerance Films tree)
	{"biphoria", "biphoria.com", "BiPhoria", "", "", ""},

	// Blowpass segment — network hub (all content in the segment)
	{"blowpass", "blowpass.com", "", "", "", ""},

	// Blowpass segment — individual sites with own domains
	{"throated", "throated.com", "Throated", "", "", ""},
	{"1000facials", "1000facials.com", "1000 Facials", "", "", ""},
	{"onlyteenblowjobs", "onlyteenblowjobs.com", "Only Teen Blowjobs", "", "", ""},
	{"immorallive", "immorallive.com", "Immoral Live", "", "", ""},

	// ---- ASGMAX segment (Next Door Studios family — gay all-access network) ----
	//
	// Three groups, ordered so the more specific MatchRe sites are evaluated
	// before the bare nextdoorstudios.com hub catches the rest:
	//   1. Own-domain sites — /en/videos served by their own host.
	//   2. Own-domain sites that redirect to nextdoorstudios.com/en/videos/sites/{slug}.
	//      RefererBase pins the Algolia bootstrap to the hub so the apiKey's
	//      Referer restriction matches when we make the search request.
	//   3. Sub-sites that only live under nextdoorstudios.com/en/videos/sites/{slug}.
	//   4. The bare nextdoorstudios.com network hub — last so it's a fallback
	//      when nothing more specific matched.

	// ASGMAX — own-domain sub-sites (apiKey extracted from each site's /en/videos).
	{"nextdoorbuddies", "nextdoorbuddies.com", "Next Door Buddies", "nextdoorbuddies", "", ""},
	{"nextdoorraw", "nextdoorraw.com", "Next Door Raw", "nextdoorraw", "", ""},
	{"nextdoortwink", "nextdoortwink.com", "Next Door Twink", "nextdoortwink", "", ""},
	{"nextdoortaboo", "nextdoortaboo.com", "Next Door Taboo", "nextdoortaboo", "", ""},
	{"nextdoormale", "nextdoormale.com", "Next Door Male", "nextdoormale", "", ""},
	{"nextdoorfilms", "nextdoorfilms.com", "Next Door Films", "nextdoorfilms", "", ""},
	{"nextdoorhookups", "nextdoorhookups.com", "Next Door Hookups", "nextdoorhookups", "", ""},
	{"nextdoorcasting", "nextdoorcasting.com", "Next Door Casting", "nextdoorcasting", "", ""},
	{"codycummings", "codycummings.com", "Cody Cummings", "codycummings", "", ""},
	{"rodsroom", "rodsroom.com", "Rod's Room", "rodsroom", "", ""},
	{"stagcollective", "stagcollective.com", "Stag Collective", "stagcollective", "", ""},

	// ASGMAX — own-domain sites that redirect to nextdoorstudios.com.
	// RefererBase makes the apiKey bootstrap + Algolia Referer header consistent.
	{"marcusmojo", "marcusmojo.com", "Marcus Mojo", "marcusmojo", "https://www.nextdoorstudios.com", ""},
	{"masonwyler", "masonwyler.com", "Mason Wyler", "masonwyler", "https://www.nextdoorstudios.com", ""},
	{"roddaily", "roddaily.com", "Rod Daily", "roddaily", "https://www.nextdoorstudios.com", ""},
	{"samuelotoole", "samuelotoole.com", "Samuel O'Toole", "samuelotoole", "https://www.nextdoorstudios.com", ""},
	{"trystanbull", "trystanbull.com", "Trystan Bull", "trystanbull", "https://www.nextdoorstudios.com", ""},
	{"nextdoorebony", "nextdoorebony.com", "Next Door Ebony", "nextdoorebony", "https://www.nextdoorstudios.com", ""},

	// ASGMAX — hub-only sub-sites with no own domain. Domain stays nextdoorstudios.com
	// so the apiKey bootstrap works; specific MatchRe scopes URL matching.
	{"austinwilde", "nextdoorstudios.com", "Austin Wilde", "austinwilde", "", `^https?://(?:www\.)?nextdoorstudios\.com/en/videos/sites/austinwilde(?:/|$)`},
	{"tommydxxx", "nextdoorstudios.com", "Tommy D XXX", "tommydxxx", "", `^https?://(?:www\.)?nextdoorstudios\.com/en/videos/sites/tommydxxx(?:/|$)`},
	{"nextdoororiginals", "nextdoorstudios.com", "Next Door Originals", "nextdoororiginals", "", `^https?://(?:www\.)?nextdoorstudios\.com/en/videos/sites/nextdoororiginals(?:/|$)`},
	{"nextdoorhomemade", "nextdoorstudios.com", "Next Door Homemade", "nextdoorhomemade", "", `^https?://(?:www\.)?nextdoorstudios\.com/en/videos/sites/nextdoorhomemade(?:/|$)`},
	// Sub-sites listed in nextdoorstudios.com's contentSource that don't have
	// their own scraper entry above — included so users who paste these URLs
	// get coverage too.
	{"strokethatdick", "nextdoorstudios.com", "Stroke That Dick", "strokethatdick", "", `^https?://(?:www\.)?nextdoorstudios\.com/en/videos/sites/strokethatdick(?:/|$)`},
	{"guysdoingporn", "nextdoorstudios.com", "Guys Doing Porn", "guysdoingporn", "", `^https?://(?:www\.)?nextdoorstudios\.com/en/videos/sites/guysdoingporn(?:/|$)`},

	// ASGMAX — network hub (catches the bare nextdoorstudios.com URL).
	// SiteName empty → no availableOnSite filter → whole asgmax catalog.
	{"nextdoorstudios", "nextdoorstudios.com", "Next Door Studios", "", "", ""},
}

type siteScraper struct {
	gamma   *gammautil.Scraper
	config  siteConfig
	matchRe *regexp.Regexp
}

var _ scraper.StudioScraper = (*siteScraper)(nil)

func (s *siteScraper) ID() string               { return s.config.SiteID }
func (s *siteScraper) Patterns() []string       { return []string{s.config.Domain} }
func (s *siteScraper) MatchesURL(u string) bool { return s.matchRe.MatchString(u) }

func (s *siteScraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.gamma.Run(ctx, studioURL, opts, out)
	return out, nil
}

func init() {
	for _, cfg := range sites {
		var re *regexp.Regexp
		if cfg.MatchRe != "" {
			re = regexp.MustCompile(cfg.MatchRe)
		} else {
			escaped := strings.ReplaceAll(cfg.Domain, ".", `\.`)
			re = regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s(?:/|$)`, escaped))
		}

		siteName := cfg.SiteName
		if siteName == "" && cfg.StudioName != "" {
			siteName = cfg.SiteID
		}

		gammaCfg := gammautil.SiteConfig{
			SiteID:      cfg.SiteID,
			SiteBase:    "https://www." + cfg.Domain,
			StudioName:  cfg.StudioName,
			SiteName:    siteName,
			RefererBase: cfg.RefererBase,
		}

		s := &siteScraper{
			gamma:   gammautil.New(gammaCfg),
			config:  cfg,
			matchRe: re,
		}
		scraper.Register(s)
	}
}
