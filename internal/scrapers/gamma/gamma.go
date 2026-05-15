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
}

type siteScraper struct {
	gamma   *gammautil.Scraper
	config  siteConfig
	matchRe *regexp.Regexp
}

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
			re = regexp.MustCompile(fmt.Sprintf(`^https?://(?:www\.)?%s`, escaped))
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
			gamma:   gammautil.NewScraper(gammaCfg),
			config:  cfg,
			matchRe: re,
		}
		scraper.Register(s)
	}
}
