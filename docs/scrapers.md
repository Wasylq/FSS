# Supported Scrapers

| Site | URL Pattern | Platform | Pricing | Notes |
|------|-------------|----------|---------|-------|
| [50 Plus MILFs](https://www.50plusmilfs.com) | `50plusmilfs.com` | Score Group | No | HTML listing + detail page worker pool for dates/tags/description, uses `scoregrouputil` |
| [A POV Story](https://apovstory.com) | `apovstory.com` | PHP tour site | No | HTML listing + detail pages, category extraction |
| [Aunt Judy's](https://www.auntjudysxxx.com) | `auntjudysxxx.com`, `auntjudysxxx.com/tour/categories/movies.html`, `auntjudysxxx.com/tour/models/{slug}.html` | JEBN CMS | No | HTML listing (24/page, `movies_N_d.html`) + detail page worker pool for title/description/tags/duration, model page support, ~1000+ scenes |
| [APClips](https://apclips.com) | `apclips.com/{creator_slug}` | Custom HTML | Yes | HTML listing (60/page, `sort=date-new`) + detail pages for dates/tags, price tracking |
| [Babes](https://www.babes.com) | `babes.com`, `babes.com/pornstar/{id}/{slug}`, `babes.com/category/{id}/{slug}`, `babes.com/site/{id}/{slug}`, `babes.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [BangBros](https://www.bangbros.com) | `bangbros.com`, `bangbros.com/model/{id}/{slug}`, `bangbros.com/category/{slug}`, `bangbros.com/websites/{slug}`, `bangbros.com/site/{id}/{slug}`, `bangbros.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, slug-to-ID resolution for `/websites/` and `/category/` URLs, uses `ayloutil` |
| [Brazzers](https://www.brazzers.com) | `brazzers.com`, `brazzers.com/pornstar/{id}/{slug}`, `brazzers.com/category/{id}/{slug}`, `brazzers.com/site/{id}/{slug}`, `brazzers.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Clips4Sale](https://www.clips4sale.com) | `clips4sale.com/studio/{id}/{slug}` | Clips4Sale | Yes | Multi-page JSON, categories, all-page enumeration |
| [Anal Therapy](https://analtherapyxxx.com) | `analtherapyxxx.com` | WordPress | No | Sitemap-driven, JSON-LD VideoObject fallback, uses `wputil` |
| [Digital Playground](https://www.digitalplayground.com) | `digitalplayground.com`, `digitalplayground.com/pornstar/{id}/{slug}`, `digitalplayground.com/category/{id}/{slug}`, `digitalplayground.com/site/{id}/{slug}`, `digitalplayground.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Fakings](https://fakings.com) | `fakings.com`, `fakings.com/serie/{slug}`, `fakings.com/actrices-porno/{slug}`, `fakings.com/categoria/{slug}` | Next.js RSC | No | React Server Component flight payload parsing, 5 sub-brands (fakings/pepeporn/nigged/morenolust/pornermates), pagination via `/f/pag:{N}`, actress pages load all videos at once |
| [Family Therapy](https://familytherapyxxx.com) | `familytherapyxxx.com` | WordPress | No | Sitemap-driven, Rank Math SEO, performer extraction from title, uses `wputil` |
| [FanCentro](https://fancentro.com) | `fancentro.com/{model}` | FanCentro | Yes (USD) | REST API (`/api/content/content`), no auth, page-based pagination (24/page), price tracking with discount detection |
| [FapHouse](https://faphouse.com) | `faphouse.com/models/{slug}`, `faphouse.com/studios/{slug}` | Custom HTML | Yes | HTML listing (60/page, `sort=new`) + detail pages with embedded JSON (`view-state-data`) for dates/performers/categories, price tracking for VOD content, xHamster ecosystem |
| [Gloryhole Secrets](https://www.gloryholesecrets.com) | `gloryholesecrets.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [House of Fyre](https://www.houseofyre.com) | `houseofyre.com`, `houseofyre.com/models/{name}.html` | ElevatedX | Yes | HTML listing + detail page worker pool for description/tags, price tracking, ~450 scenes |
| [BrasilVR](https://www.brasilvr.com) | `brasilvr.com` | POVR/WankzVR | No | Export JSON + listing page dates, uses `povrutil` |
| [IWantClips](https://www.iwantclips.com) | `iwantclips.com/store/{id}/{username}` | IWantClips | Yes | JSON API, double HTML-unescaping |
| [Lady Sonia](https://tour.lady-sonia.com) | `lady-sonia.com` | KB Productions/Next.js | No | `__NEXT_DATA__` JSON parsing, 1500+ scenes, listing-only (no detail pages needed) |
| [LoyalFans](https://www.loyalfans.com) | `loyalfans.com/{creator_slug}` | LoyalFans API | No | POST `/api/v2/advanced-search`, cursor-based `page_token` pagination (20/page), session via `/api/v2/system-status`, filters results by owner slug |
| [Kink](https://www.kink.com) | `kink.com`, `kink.com/channel/{slug}`, `kink.com/model/{id}/{slug}`, `kink.com/tag/{slug}`, `kink.com/series/{slug}` | Kink | No | HTML scraping, 51 channels, filter by channel/performer/tag/series, detail page worker pool for tags/description/duration, age gate cookie bypass, JSON-LD + data-setup parsing |
| [ManyVids](https://www.manyvids.com) | `manyvids.com/Profile/{id}/{slug}/Store/Videos` | ManyVids | Yes | JSON API, detail-page worker pool |
| [Mature.nl](https://www.mature.nl) | `mature.nl/en/updates`, `mature.nl/en/model/{id}`, `mature.nl/en/niche/{id}/{page}/{slug}` | Custom | No | HTML scraping, paginated listing + detail page worker pool for model URLs |
| [MYLF](https://www.mylf.com) | `mylf.com`, `mylf.com/models/{slug}`, `mylf.com/series/{slug}`, `mylf.com/categories/{name}` | TeamSkeet/PSM | No | Public Elasticsearch API, filter by model/series/category |
| [Pure Mature](https://puremature.com) | `puremature.com`, `puremature.com/models/{slug}` | AMA Multimedia | No | JSON REST API, filter by model, resolution from download options |
| [MilfVR](https://www.milfvr.com) | `milfvr.com` | POVR/WankzVR | No | Export JSON + listing page dates, uses `povrutil` |
| [MissaX](https://www.missax.com) | `missax.com` | Custom | No | HTML scraping, listing + detail page worker pool |
| [Mommy Blows Best](https://www.mommyblowsbest.com) | `mommyblowsbest.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Mofos](https://www.mofos.com) | `mofos.com`, `mofos.com/pornstar/{id}/{slug}`, `mofos.com/category/{id}/{slug}`, `mofos.com/site/{id}/{slug}`, `mofos.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Mom Comes First](https://momcomesfirst.com) | `momcomesfirst.com` | WordPress | No | Sitemap-driven, JSON-LD VideoObject, uses `wputil` |
| [OopsFamily](https://oopsfamily.com) | `oopsfamily.com`, `oopsfamily.com/model/{slug}`, `oopsfamily.com/tag/{slug}` | FapHouse/Custom | No | HTML listing + detail page JSON-LD worker pool for dates/tags, model/tag filtering, 4K |
| [Naughty America](https://www.naughtyamerica.com) | `naughtyamerica.com`, `naughtyamericavr.com`, `myfriendshotmom.com`, `mysistershotfriend.com`, `tonightsgirlfriend.com`, `thundercock.com` | Naughty America | No | Open JSON API at api.naughtyapi.com, ~15k scenes, 50+ sub-sites, VR support |
| [Nubiles Network](https://nubiles-porn.com) | `nubiles-porn.com`, `nubiles.net`, `momsteachsex.com`, `stepsiblingscaught.com`, `myfamilypies.com`, `princesscum.com`, + 14 more | EdgeCms | No | HTML scraping, 20+ network sites, detail page worker pool for tags/description, filter by model or category |
| [MyDirtyHobby](https://www.mydirtyhobby.com) | `mydirtyhobby.com/profil/{id}-{username}` | MyDirtyHobby | No | JSON API with auth headers |
| [Penny Barber](https://pennybarber.com) | `pennybarber.com/videos` | ModelCentro | No | JSON API (`/api/content.load`), listing + per-scene detail for tags/description, no thumbnails (CDN-encrypted) |
| [Perfect Girlfriend](https://perfectgirlfriend.com) | `perfectgirlfriend.com` | WordPress | No | Sitemap-driven, JSON-LD VideoObject fallback, uses `wputil` |
| [Pornhub](https://www.pornhub.com) | `pornhub.com/pornstar/{slug}`, `pornhub.com/channels/{slug}` | Pornhub | Free | HTML scraping, minimal fields |
| [Pure Taboo](https://www.puretaboo.com) | `puretaboo.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Queensnake](https://queensnake.com) | `queensnake.com` | Custom | No | HTML scraping, 0-indexed paginated listing, `cLegalAge` cookie for age gate, performer extraction from title/tags, listing-only (no detail pages needed) |
| [Rachel Steele](https://rachel-steele.com) | `rachel-steele.com` | MyMember.site | Yes | JSON list API + HTML detail pages, JSON-LD keywords |
| [See Mom Suck](https://www.seemomsuck.com) | `seemomsuck.com`, `seemomsuck.com/models/{name}.html` | 3rdShiftVideo/ThickCash | No | HTML scraping, listing-only (no detail pages), model page support, ~1500 scenes, no dates available on tour pages |
| [Reagan Foxx](https://www.reaganfoxx.com) | `reaganfoxx.com`, `reaganfoxx.com/scenes/{id}/{slug}.html` | Adult Empire Stores (Ravana LLC) | Yes (USD) | HTML listing (52/page, `?page=N`) + detail page worker pool for dates/tags/price, `AgeConfirmed` cookie, ~174 scenes |
| [Reality Kings](https://www.realitykings.com) | `realitykings.com`, `realitykings.com/pornstar/{id}/{slug}`, `realitykings.com/category/{id}/{slug}`, `realitykings.com/site/{id}/{slug}`, `realitykings.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Taboo Heat](https://www.tabooheat.com) | `tabooheat.com` | Gamma/Algolia | No | Thin wrapper around `gammautil` |
| [Tara Tainton](https://taratainton.com) | `taratainton.com` | WordPress | Yes | Sitemap-driven, HTML meta parsing, uses `wputil` |
| [Evil Angel](https://www.evilangel.com) | `evilangel.com` | Gamma/Algolia | No | Algolia search API, ~20k scenes, uses `gammautil` |
| [Exposed Latinas](https://exposedlatinas.com) | `exposedlatinas.com/tour/updates`, `exposedlatinas.com/tour/models/{slug}.html`, `exposedlatinas.com/tour/categories/{slug}.html` | SexMex Pro CMS | No | Thin wrapper around `sexmexutil` |
| [SexMex](https://sexmex.xxx) | `sexmex.xxx/tour/updates`, `sexmex.xxx/tour/models/{slug}.html`, `sexmex.xxx/tour/categories/{slug}.html` | SexMex Pro CMS | No | HTML scraping with regex, pagination, model/category/full-studio modes, uses `sexmexutil` |
| [Sofie Marie](https://sofiemariexxx.com) | `sofiemariexxx.com`, `sofiemariexxx.com/models/{slug}.html`, `sofiemariexxx.com/dvds/{slug}.html` | ELXComplete/Andomark | No | HTML scraping, paginated listing (movies category), model pages via `sets.php` pagination, DVD pages single-fetch |
| [Trans Queens](https://transqueens.com) | `transqueens.com/tour/updates`, `transqueens.com/tour/models/{slug}.html`, `transqueens.com/tour/categories/{slug}.html` | SexMex Pro CMS | No | Thin wrapper around `sexmexutil` |
| [TranzVR](https://www.tranzvr.com) | `tranzvr.com` | POVR/WankzVR | No | Thin wrapper around `povrutil` |
| [Visit-X](https://www.visit-x.net) | `visit-x.net/{lang}/amateur/{model}/videos/` | VXOne (custom) | Yes (VXC) | GraphQL API (`/vxql`), JWT token from page HTML (no login), all data from listing query, price tracking in VXC coins |
| [Xev Bellringer](https://www.xevunleashed.com) | `xevunleashed.com`, `xevunleashed.com/categories/movies.html` | JEBN CMS | Yes (USD) | HTML listing (10/page, `movies_{N}.html`) + detail page worker pool for description/tags, price tracking, ~343 scenes, slug-based IDs |
| [WankzVR](https://www.wankzvr.com) | `wankzvr.com` | POVR/WankzVR | No | Export JSON + listing page dates, uses `povrutil` |
| [Burning Angel](https://www.burningangel.com) | `burningangel.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Filthy Kings](https://www.filthykings.com) | `filthykings.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Gangbang Creampie](https://www.gangbangcreampie.com) | `gangbangcreampie.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Girlfriends Films](https://www.girlfriendsfilms.com) | `girlfriendsfilms.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Lethal Hardcore](https://www.lethalhardcore.com) | `lethalhardcore.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Rocco Siffredi](https://www.roccosiffredi.com) | `roccosiffredi.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Wicked](https://www.wicked.com) | `wicked.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [PropertySex](https://www.propertysex.com) | `propertysex.com`, `propertysex.com/pornstar/{id}/{slug}`, `propertysex.com/category/{id}/{slug}`, `propertysex.com/site/{id}/{slug}`, `propertysex.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [TransAngels](https://www.transangels.com) | `transangels.com`, `transangels.com/pornstar/{id}/{slug}`, `transangels.com/category/{id}/{slug}`, `transangels.com/site/{id}/{slug}`, `transangels.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Twistys](https://www.twistys.com) | `twistys.com`, `twistys.com/pornstar/{id}/{slug}`, `twistys.com/category/{id}/{slug}`, `twistys.com/site/{id}/{slug}`, `twistys.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Jerk Off Instructions](https://jerkoffinstructions.com) | `jerkoffinstructions.com` | Custom PHP | No | HTML scraping, 302 redirect with body, listing-only (no detail pages needed), `free_adult` cookie for thumbnails |
| [YourVids](https://yourvids.com) | `yourvids.com/creators/{slug}` | YourVids | Yes | JSON API, detail-page worker pool for tags/description, price tracking with sale detection |
| [BoyfriendSharing](https://boyfriendsharing.com) | `boyfriendsharing.com` | WP video-elements | No | WP REST API, uses `veutil` |
| [BrattyFamily](https://brattyfamily.com) | `brattyfamily.com` | WP video-elements | No | WP REST API, uses `veutil` |
| [GoStuckYourself](https://gostuckyourself.net) | `gostuckyourself.net` | WP video-elements | No | WP REST API, uses `veutil` |
| [HugeCockBreak](https://hugecockbreak.com) | `hugecockbreak.com` | WP video-elements | No | WP REST API, uses `veutil` |
| [LittleFromAsia](https://littlefromasia.com) | `littlefromasia.com` | WP video-elements | No | WP REST API, uses `veutil` |
| [MommysBoy](https://mommysboy.net) | `mommysboy.net` | WP video-elements | No | WP REST API, uses `veutil` |
| [MomXXX](https://momxxx.org) | `momxxx.org` | WP video-elements | No | WP REST API, category 10, uses `veutil` |
| [MyBadMILFs](https://mybadmilfs.com) | `mybadmilfs.com` | WP video-elements | No | WP REST API, uses `veutil` |
| [DaughterSwap](https://mydaughterswap.com) | `mydaughterswap.com` | WP video-elements | No | WP REST API, uses `veutil` |
| [PervMom](https://mypervmom.com) | `mypervmom.com` | WP video-elements | No | WP REST API, uses `veutil` |
| [SisLovesMe](https://mysislovesme.com) | `mysislovesme.com` | WP video-elements | No | WP REST API, uses `veutil` |
| [YoungerLoverOfMine](https://youngerloverofmine.com) | `youngerloverofmine.com` | WP video-elements | No | WP REST API, uses `veutil` |

## Shared scraper packages

Scrapers that share a platform use common utility packages to avoid duplication:

| Package | Platform | Used by |
|---------|----------|---------|
| `ayloutil` | Aylo/Juan (REST API, instance token auth) | Babes, BangBros, Brazzers, Digital Playground, Mofos, PropertySex, Reality Kings, TransAngels, Twistys |
| `gammautil` | Gamma Entertainment (Algolia search API) | Burning Angel, Evil Angel, Filthy Kings, Gangbang Creampie, Girlfriends Films, Gloryhole Secrets, Lethal Hardcore, Mommy Blows Best, Pure Taboo, Rocco Siffredi, Taboo Heat, Wicked |
| `scoregrouputil` | Score Group (HTML listing + detail pages, `meta name="Date"` for dates, `updates-tag` links for tags) | 50 Plus MILFs |
| `povrutil` | POVR/WankzVR (export JSON + HTML listing pages) | BrasilVR, MilfVR, TranzVR, WankzVR |
| `sexmexutil` | SexMex Pro CMS (HTML scraping, pagination). **Quirk:** their CMS returns HTTP 500 with valid HTML on some pages (e.g. model pages), so `fetchPage` accepts 500 responses instead of using `httpx.Do`. | Exposed Latinas, SexMex, Trans Queens |
| `veutil` | WordPress video-elements theme (WP REST API for posts + tags, poster extraction from content) | BoyfriendSharing, BrattyFamily, GoStuckYourself, HugeCockBreak, LittleFromAsia, MommysBoy, MomXXX, MyBadMILFs, DaughterSwap, PervMom, SisLovesMe, YoungerLoverOfMine |
| `wputil` | WordPress (sitemap + HTML meta parsing) | Anal Therapy, Family Therapy, Mom Comes First, Perfect Girlfriend, Tara Tainton |

## Adding a new scraper

See the [contributing guide](../CONTRIBUTING.md) for step-by-step instructions and reference implementations.
