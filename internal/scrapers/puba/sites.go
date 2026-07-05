package puba

import (
	"regexp"

	"github.com/Wasylq/FSS/scraper"
)

// sites lists every Puba scraper. Each per-pornstar entry maps to the
// `&group={N}` filter on the network's pornstarnetwork JSON API; group
// IDs come from the `index.php?section=539` site listing where each
// site card is preceded by an HTML comment naming the site.
//
// The parent `puba` scraper uses Group=0 to walk the whole 2800+
// catalogue without a group filter (the API switches to `view=v` to
// expose the full latest-videos listing in that case).
var sites = []SiteConfig{
	// Parent network — walks the entire catalogue (~2846 scenes).
	{
		ID:       "puba",
		SiteName: "",
		Group:    0,
		Patterns: []string{"https://www.puba.com/pornstarnetwork/index.php?section=538"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?puba\.com/pornstarnetwork/?(?:$|\?|index\.php)`),
	},
	// Per-pornstar sub-sites. Each filters the catalogue by group ID.
	{
		ID:       "puba1girl1camera",
		SiteName: "1 Girl 1 Camera",
		Group:    3,
		Patterns: []string{"https://www.1girl1camera.com", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=3"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?(?:1girl1camera\.com|puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=3(?:&|$))`),
	},
	{
		ID:       "pubaabigailmac",
		SiteName: "Abigail Mac",
		Group:    79,
		Patterns: []string{"https://abigailmac.puba.com/tour/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=79"},
		MatchRe:  regexp.MustCompile(`^https?://(?:abigailmac\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=79(?:&|$))`),
	},
	{
		ID:       "pubaalixlynx",
		SiteName: "Alix Lynx",
		Group:    104,
		Patterns: []string{"https://alixlynx.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=104"},
		MatchRe:  regexp.MustCompile(`^https?://(?:alixlynx\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=104(?:&|$))`),
	},
	{
		ID:       "pubaasaakira",
		SiteName: "Asa Akira",
		Group:    14,
		Patterns: []string{"https://asaakira.puba.com/", "https://asafucks.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=14"},
		MatchRe:  regexp.MustCompile(`^https?://(?:asaakira\.puba\.com|(?:www\.)?asafucks\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=14(?:&|$))`),
	},
	{
		ID:       "pubabouncypictures",
		SiteName: "Bouncy Pictures",
		Group:    2,
		Patterns: []string{"https://www.bouncypicturesonline.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=2"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?(?:bouncypicturesonline\.com|puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=2(?:&|$))`),
	},
	{
		ID:       "pubabritneyamber",
		SiteName: "Britney Amber",
		Group:    55,
		Patterns: []string{"https://britneyamber.puba.com/tour/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=55"},
		MatchRe:  regexp.MustCompile(`^https?://(?:britneyamber\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=55(?:&|$))`),
	},
	{
		ID:       "pubalolafoxx",
		SiteName: "Lola Foxx",
		Group:    76,
		Patterns: []string{"https://lolafoxx.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=76"},
		MatchRe:  regexp.MustCompile(`^https?://(?:lolafoxx\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=76(?:&|$))`),
	},
	{
		ID:       "pubamydollparts",
		SiteName: "My Doll Parts",
		Group:    125,
		Patterns: []string{"https://mydollparts.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=125"},
		MatchRe:  regexp.MustCompile(`^https?://(?:mydollparts\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=125(?:&|$))`),
	},
	{
		ID:       "pubanicoleaniston",
		SiteName: "Nicole Aniston",
		Group:    87,
		Patterns: []string{"https://nicoleaniston.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=87"},
		MatchRe:  regexp.MustCompile(`^https?://(?:nicoleaniston\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=87(?:&|$))`),
	},
	{
		ID:       "pubapriyarai",
		SiteName: "Priya Rai",
		Group:    128,
		Patterns: []string{"https://www.priyaraiofficial.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=128"},
		MatchRe:  regexp.MustCompile(`^https?://(?:www\.)?(?:priyaraiofficial\.com|puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=128(?:&|$))`),
	},
	{
		ID:       "pubasamanthasaint",
		SiteName: "Samantha Saint",
		Group:    46,
		Patterns: []string{"https://samanthasaint.puba.com/", "https://samanthafucks.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=46"},
		MatchRe:  regexp.MustCompile(`^https?://(?:samanthasaint\.puba\.com|(?:www\.)?samanthafucks\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=46(?:&|$))`),
	},
	{
		ID:       "pubashylastylez",
		SiteName: "Shyla Stylez",
		Group:    13,
		Patterns: []string{"https://shyla.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=13"},
		MatchRe:  regexp.MustCompile(`^https?://(?:shyla\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=13(?:&|$))`),
	},
	{
		ID:       "pubavanessacage",
		SiteName: "Vanessa Cage",
		Group:    49,
		Patterns: []string{"https://vanessacage.puba.com/tour/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=49"},
		MatchRe:  regexp.MustCompile(`^https?://(?:vanessacage\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=49(?:&|$))`),
	},
	// Additional Puba network sub-sites (not in stashdb, discovered via
	// the network's `?section=539` site index).
	{
		ID:       "pubaadventuresxxx",
		SiteName: "Adventures XXX",
		Group:    66,
		Patterns: []string{"https://adventuresxxx.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=66"},
		MatchRe:  regexp.MustCompile(`^https?://(?:adventuresxxx\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=66(?:&|$))`),
	},
	{
		ID:       "pubaalisontyler",
		SiteName: "Alison Tyler",
		Group:    57,
		Patterns: []string{"https://alisontyler.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=57"},
		MatchRe:  regexp.MustCompile(`^https?://(?:alisontyler\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=57(?:&|$))`),
	},
	{
		ID:       "pubaashleegraham",
		SiteName: "Ashlee Graham",
		Group:    127,
		Patterns: []string{"https://ashleegraham.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=127"},
		MatchRe:  regexp.MustCompile(`^https?://(?:ashleegraham\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=127(?:&|$))`),
	},
	{
		ID:       "pubaavyscott",
		SiteName: "Avy Scott",
		Group:    10,
		Patterns: []string{"https://avyscott.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=10"},
		MatchRe:  regexp.MustCompile(`^https?://(?:avyscott\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=10(?:&|$))`),
	},
	{
		ID:       "pubabangingpornstars",
		SiteName: "Banging Pornstars",
		Group:    77,
		Patterns: []string{"https://bangingpornstars.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=77"},
		MatchRe:  regexp.MustCompile(`^https?://(?:bangingpornstars\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=77(?:&|$))`),
	},
	{
		ID:       "pubabonuscontent",
		SiteName: "Bonus Content",
		Group:    16,
		Patterns: []string{"https://bonuscontent.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=16"},
		MatchRe:  regexp.MustCompile(`^https?://(?:bonuscontent\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=16(?:&|$))`),
	},
	{
		ID:       "pubabreeolson",
		SiteName: "Bree Olson",
		Group:    131,
		Patterns: []string{"https://breeolson.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=131"},
		MatchRe:  regexp.MustCompile(`^https?://(?:breeolson\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=131(?:&|$))`),
	},
	{
		ID:       "pubabrettrossi",
		SiteName: "Brett Rossi",
		Group:    72,
		Patterns: []string{"https://brettrossi.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=72"},
		MatchRe:  regexp.MustCompile(`^https?://(?:brettrossi\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=72(?:&|$))`),
	},
	{
		ID:       "pubabrookebrand",
		SiteName: "Brooke Brand",
		Group:    18,
		Patterns: []string{"https://brookebrand.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=18"},
		MatchRe:  regexp.MustCompile(`^https?://(?:brookebrand\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=18(?:&|$))`),
	},
	{
		ID:       "pubabrooklynchase",
		SiteName: "Brooklyn Chase",
		Group:    114,
		Patterns: []string{"https://brooklynchase.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=114"},
		MatchRe:  regexp.MustCompile(`^https?://(?:brooklynchase\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=114(?:&|$))`),
	},
	{
		ID:       "pubacapricavanni",
		SiteName: "Capri Cavanni",
		Group:    51,
		Patterns: []string{"https://capricavanni.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=51"},
		MatchRe:  regexp.MustCompile(`^https?://(?:capricavanni\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=51(?:&|$))`),
	},
	{
		ID:       "pubacharleychase",
		SiteName: "Charley Chase",
		Group:    11,
		Patterns: []string{"https://charleychase.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=11"},
		MatchRe:  regexp.MustCompile(`^https?://(?:charleychase\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=11(?:&|$))`),
	},
	{
		ID:       "pubachristianacinn",
		SiteName: "Christiana Cinn",
		Group:    130,
		Patterns: []string{"https://christianacinn.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=130"},
		MatchRe:  regexp.MustCompile(`^https?://(?:christianacinn\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=130(?:&|$))`),
	},
	{
		ID:       "pubachristymack",
		SiteName: "Christy Mack",
		Group:    59,
		Patterns: []string{"https://christymack.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=59"},
		MatchRe:  regexp.MustCompile(`^https?://(?:christymack\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=59(?:&|$))`),
	},
	{
		ID:       "pubaczechhotties",
		SiteName: "Czech Hotties",
		Group:    8,
		Patterns: []string{"https://czechhotties.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=8"},
		MatchRe:  regexp.MustCompile(`^https?://(?:czechhotties\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=8(?:&|$))`),
	},
	{
		ID:       "pubadahliasky",
		SiteName: "Dahlia Sky",
		Group:    60,
		Patterns: []string{"https://dahliasky.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=60"},
		MatchRe:  regexp.MustCompile(`^https?://(?:dahliasky\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=60(?:&|$))`),
	},
	{
		ID:       "pubadaisymonroe",
		SiteName: "Daisy Monroe",
		Group:    105,
		Patterns: []string{"https://daisymonroe.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=105"},
		MatchRe:  regexp.MustCompile(`^https?://(?:daisymonroe\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=105(?:&|$))`),
	},
	{
		ID:       "pubadanadearmond",
		SiteName: "Dana DeArmond",
		Group:    73,
		Patterns: []string{"https://danadearmond.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=73"},
		MatchRe:  regexp.MustCompile(`^https?://(?:danadearmond\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=73(?:&|$))`),
	},
	{
		ID:       "pubadanidaniels",
		SiteName: "Dani Daniels",
		Group:    56,
		Patterns: []string{"https://danidaniels.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=56"},
		MatchRe:  regexp.MustCompile(`^https?://(?:danidaniels\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=56(?:&|$))`),
	},
	{
		ID:       "pubadavafoxx",
		SiteName: "Dava Foxx",
		Group:    90,
		Patterns: []string{"https://davafoxx.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=90"},
		MatchRe:  regexp.MustCompile(`^https?://(?:davafoxx\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=90(?:&|$))`),
	},
	{
		ID:       "pubadiamondkitty",
		SiteName: "Diamond Kitty",
		Group:    52,
		Patterns: []string{"https://diamondkitty.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=52"},
		MatchRe:  regexp.MustCompile(`^https?://(?:diamondkitty\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=52(?:&|$))`),
	},
	{
		ID:       "pubaelsajean",
		SiteName: "Elsa Jean",
		Group:    106,
		Patterns: []string{"https://elsajean.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=106"},
		MatchRe:  regexp.MustCompile(`^https?://(?:elsajean\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=106(?:&|$))`),
	},
	{
		ID:       "pubafacepounders",
		SiteName: "Face Pounders",
		Group:    7,
		Patterns: []string{"https://facepounders.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=7"},
		MatchRe:  regexp.MustCompile(`^https?://(?:facepounders\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=7(?:&|$))`),
	},
	{
		ID:       "pubaforbiddenhookups",
		SiteName: "Forbidden Hookups",
		Group:    137,
		Patterns: []string{"https://forbiddenhookups.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=137"},
		MatchRe:  regexp.MustCompile(`^https?://(?:forbiddenhookups\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=137(?:&|$))`),
	},
	{
		ID:       "pubagiannamichaels",
		SiteName: "Gianna Michaels",
		Group:    132,
		Patterns: []string{"https://giannamichaels.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=132"},
		MatchRe:  regexp.MustCompile(`^https?://(?:giannamichaels\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=132(?:&|$))`),
	},
	{
		ID:       "pubahardgonzo",
		SiteName: "Hard Gonzo",
		Group:    23,
		Patterns: []string{"https://hardgonzo.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=23"},
		MatchRe:  regexp.MustCompile(`^https?://(?:hardgonzo\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=23(?:&|$))`),
	},
	{
		ID:       "pubajaydencole",
		SiteName: "Jayden Cole",
		Group:    123,
		Patterns: []string{"https://jaydencole.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=123"},
		MatchRe:  regexp.MustCompile(`^https?://(?:jaydencole\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=123(?:&|$))`),
	},
	{
		ID:       "pubajaydenjaymes",
		SiteName: "Jayden Jaymes",
		Group:    4,
		Patterns: []string{"https://jaydenjaymes.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=4"},
		MatchRe:  regexp.MustCompile(`^https?://(?:jaydenjaymes\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=4(?:&|$))`),
	},
	{
		ID:       "pubajenhexxx",
		SiteName: "Jen Hexxx",
		Group:    126,
		Patterns: []string{"https://jenhexxx.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=126"},
		MatchRe:  regexp.MustCompile(`^https?://(?:jenhexxx\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=126(?:&|$))`),
	},
	{
		ID:       "pubajezebellebond",
		SiteName: "Jezebelle Bond",
		Group:    118,
		Patterns: []string{"https://jezebellebond.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=118"},
		MatchRe:  regexp.MustCompile(`^https?://(?:jezebellebond\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=118(?:&|$))`),
	},
	{
		ID:       "pubakendallkarson",
		SiteName: "Kendall Karson",
		Group:    58,
		Patterns: []string{"https://kendallkarson.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=58"},
		MatchRe:  regexp.MustCompile(`^https?://(?:kendallkarson\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=58(?:&|$))`),
	},
	{
		ID:       "pubakendracole",
		SiteName: "Kendra Cole",
		Group:    124,
		Patterns: []string{"https://kendracole.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=124"},
		MatchRe:  regexp.MustCompile(`^https?://(?:kendracole\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=124(?:&|$))`),
	},
	{
		ID:       "pubakikidaire",
		SiteName: "Kiki D'Aire",
		Group:    129,
		Patterns: []string{"https://kikidaire.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=129"},
		MatchRe:  regexp.MustCompile(`^https?://(?:kikidaire\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=129(?:&|$))`),
	},
	{
		ID:       "pubakirstenprice",
		SiteName: "Kirsten Price",
		Group:    65,
		Patterns: []string{"https://kirstenprice.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=65"},
		MatchRe:  regexp.MustCompile(`^https?://(?:kirstenprice\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=65(?:&|$))`),
	},
	{
		ID:       "pubaleyafalcon",
		SiteName: "Leya Falcon",
		Group:    67,
		Patterns: []string{"https://leyafalcon.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=67"},
		MatchRe:  regexp.MustCompile(`^https?://(?:leyafalcon\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=67(?:&|$))`),
	},
	{
		ID:       "pubalilycarter",
		SiteName: "Lily Carter",
		Group:    47,
		Patterns: []string{"https://lilycarter.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=47"},
		MatchRe:  regexp.MustCompile(`^https?://(?:lilycarter\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=47(?:&|$))`),
	},
	{
		ID:       "pubalollyink",
		SiteName: "Lolly Ink",
		Group:    122,
		Patterns: []string{"https://lollyink.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=122"},
		MatchRe:  regexp.MustCompile(`^https?://(?:lollyink\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=122(?:&|$))`),
	},
	{
		ID:       "pubalondonkeyes",
		SiteName: "London Keyes",
		Group:    5,
		Patterns: []string{"https://londonkeyes.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=5"},
		MatchRe:  regexp.MustCompile(`^https?://(?:londonkeyes\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=5(?:&|$))`),
	},
	{
		ID:       "pubamaricahase",
		SiteName: "Marica Hase",
		Group:    108,
		Patterns: []string{"https://maricahase.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=108"},
		MatchRe:  regexp.MustCompile(`^https?://(?:maricahase\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=108(?:&|$))`),
	},
	{
		ID:       "pubamasonmoore",
		SiteName: "Mason Moore",
		Group:    6,
		Patterns: []string{"https://masonmoore.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=6"},
		MatchRe:  regexp.MustCompile(`^https?://(?:masonmoore\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=6(?:&|$))`),
	},
	{
		ID:       "pubamialelani",
		SiteName: "Mia Lelani",
		Group:    80,
		Patterns: []string{"https://mialelani.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=80"},
		MatchRe:  regexp.MustCompile(`^https?://(?:mialelani\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=80(?:&|$))`),
	},
	{
		ID:       "pubamishamontana",
		SiteName: "Misha Montana",
		Group:    141,
		Patterns: []string{"https://mishamontana.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=141"},
		MatchRe:  regexp.MustCompile(`^https?://(?:mishamontana\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=141(?:&|$))`),
	},
	{
		ID:       "pubamrfacial",
		SiteName: "Mr. Facial",
		Group:    17,
		Patterns: []string{"https://mrfacial.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=17"},
		MatchRe:  regexp.MustCompile(`^https?://(?:mrfacial\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=17(?:&|$))`),
	},
	{
		ID:       "pubanadiawhite",
		SiteName: "Nadia White",
		Group:    121,
		Patterns: []string{"https://nadiawhite.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=121"},
		MatchRe:  regexp.MustCompile(`^https?://(?:nadiawhite\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=121(?:&|$))`),
	},
	{
		ID:       "pubanatashanice",
		SiteName: "Natasha Nice",
		Group:    12,
		Patterns: []string{"https://natashanice.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=12"},
		MatchRe:  regexp.MustCompile(`^https?://(?:natashanice\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=12(?:&|$))`),
	},
	{
		ID:       "pubanickmanning",
		SiteName: "Nick Manning",
		Group:    19,
		Patterns: []string{"https://nickmanning.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=19"},
		MatchRe:  regexp.MustCompile(`^https?://(?:nickmanning\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=19(?:&|$))`),
	},
	{
		ID:       "pubanikitavonjames",
		SiteName: "Nikita Von James",
		Group:    78,
		Patterns: []string{"https://nikitavonjames.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=78"},
		MatchRe:  regexp.MustCompile(`^https?://(?:nikitavonjames\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=78(?:&|$))`),
	},
	{
		ID:       "pubaoliviaaustin",
		SiteName: "Olivia Austin",
		Group:    117,
		Patterns: []string{"https://oliviaaustin.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=117"},
		MatchRe:  regexp.MustCompile(`^https?://(?:oliviaaustin\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=117(?:&|$))`),
	},
	{
		ID:       "pubarachelroxxx",
		SiteName: "Rachel Roxxx",
		Group:    53,
		Patterns: []string{"https://rachelroxxx.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=53"},
		MatchRe:  regexp.MustCompile(`^https?://(?:rachelroxxx\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=53(?:&|$))`),
	},
	{
		ID:       "pubaromirain",
		SiteName: "Romi Rain",
		Group:    75,
		Patterns: []string{"https://romirain.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=75"},
		MatchRe:  regexp.MustCompile(`^https?://(?:romirain\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=75(?:&|$))`),
	},
	{
		ID:       "pubasarahjessie",
		SiteName: "Sarah Jessie",
		Group:    102,
		Patterns: []string{"https://sarahjessie.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=102"},
		MatchRe:  regexp.MustCompile(`^https?://(?:sarahjessie\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=102(?:&|$))`),
	},
	{
		ID:       "pubasarahvandella",
		SiteName: "Sarah Vandella",
		Group:    119,
		Patterns: []string{"https://sarahvandella.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=119"},
		MatchRe:  regexp.MustCompile(`^https?://(?:sarahvandella\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=119(?:&|$))`),
	},
	{
		ID:       "pubasashagrey",
		SiteName: "Sasha Grey",
		Group:    133,
		Patterns: []string{"https://sashagrey.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=133"},
		MatchRe:  regexp.MustCompile(`^https?://(?:sashagrey\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=133(?:&|$))`),
	},
	{
		ID:       "pubaskindiamond",
		SiteName: "Skin Diamond",
		Group:    61,
		Patterns: []string{"https://skindiamond.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=61"},
		MatchRe:  regexp.MustCompile(`^https?://(?:skindiamond\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=61(?:&|$))`),
	},
	{
		ID:       "pubasummerbrielle",
		SiteName: "Summer Brielle",
		Group:    107,
		Patterns: []string{"https://summerbrielle.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=107"},
		MatchRe:  regexp.MustCompile(`^https?://(?:summerbrielle\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=107(?:&|$))`),
	},
	{
		ID:       "pubataylorvixen",
		SiteName: "Taylor Vixen",
		Group:    45,
		Patterns: []string{"https://taylorvixen.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=45"},
		MatchRe:  regexp.MustCompile(`^https?://(?:taylorvixen\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=45(?:&|$))`),
	},
	{
		ID:       "pubatyendicott",
		SiteName: "Ty Endicott",
		Group:    1,
		Patterns: []string{"https://tyendicott.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=1"},
		MatchRe:  regexp.MustCompile(`^https?://(?:tyendicott\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=1(?:&|$))`),
	},
	{
		ID:       "pubavictoriawhite",
		SiteName: "Victoria White",
		Group:    48,
		Patterns: []string{"https://victoriawhite.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=48"},
		MatchRe:  regexp.MustCompile(`^https?://(?:victoriawhite\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=48(?:&|$))`),
	},
	{
		ID:       "pubavyxensteel",
		SiteName: "Vyxen Steel",
		Group:    98,
		Patterns: []string{"https://vyxensteel.puba.com/", "https://www.puba.com/pornstarnetwork/index.php?section=538&group=98"},
		MatchRe:  regexp.MustCompile(`^https?://(?:vyxensteel\.puba\.com|(?:www\.)?puba\.com/pornstarnetwork/[^?]*\?[^#]*[?&]group=98(?:&|$))`),
	},
}

func init() {
	// Register the per-pornstar sub-sites (Group != 0) BEFORE the parent
	// network (Group == 0). The parent's MatchRe also matches the sub-sites'
	// `…/index.php?…&group=N` URLs, and RE2 has no negative lookahead to
	// exclude them, so first-match-wins resolution must encounter the specific
	// sub-site first. Otherwise a `group=N` URL would resolve to the
	// whole-catalogue parent (2846 scenes) instead of the one pornstar. See
	// AUDIT_PLAN B5.
	for _, cfg := range sites {
		if cfg.Group != 0 {
			scraper.Register(New(cfg))
		}
	}
	for _, cfg := range sites {
		if cfg.Group == 0 {
			scraper.Register(New(cfg))
		}
	}
}
