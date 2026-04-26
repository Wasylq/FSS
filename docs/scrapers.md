# Supported Scrapers

| Site | URL Pattern | Platform | Pricing | Notes |
|------|-------------|----------|---------|-------|
| [A POV Story](https://apovstory.com) | `apovstory.com` | PHP tour site | No | HTML listing + detail pages, category extraction |
| [Babes](https://www.babes.com) | `babes.com`, `babes.com/pornstar/{id}/{slug}`, `babes.com/category/{id}/{slug}`, `babes.com/site/{id}/{slug}`, `babes.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Brazzers](https://www.brazzers.com) | `brazzers.com`, `brazzers.com/pornstar/{id}/{slug}`, `brazzers.com/category/{id}/{slug}`, `brazzers.com/site/{id}/{slug}`, `brazzers.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Clips4Sale](https://www.clips4sale.com) | `clips4sale.com/studio/{id}/{slug}` | Clips4Sale | Yes | Multi-page JSON, categories, all-page enumeration |
| [Anal Therapy](https://analtherapyxxx.com) | `analtherapyxxx.com` | WordPress | No | Sitemap-driven, JSON-LD VideoObject fallback, uses `wputil` |
| [Digital Playground](https://www.digitalplayground.com) | `digitalplayground.com`, `digitalplayground.com/pornstar/{id}/{slug}`, `digitalplayground.com/category/{id}/{slug}`, `digitalplayground.com/site/{id}/{slug}`, `digitalplayground.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Fakings](https://fakings.com) | `fakings.com`, `fakings.com/serie/{slug}`, `fakings.com/actrices-porno/{slug}`, `fakings.com/categoria/{slug}` | Next.js RSC | No | React Server Component flight payload parsing, 5 sub-brands (fakings/pepeporn/nigged/morenolust/pornermates), pagination via `/f/pag:{N}`, actress pages load all videos at once |
| [Family Therapy](https://familytherapyxxx.com) | `familytherapyxxx.com` | WordPress | No | Sitemap-driven, Rank Math SEO, performer extraction from title, uses `wputil` |
| [Gloryhole Secrets](https://www.gloryholesecrets.com) | `gloryholesecrets.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [BrasilVR](https://www.brasilvr.com) | `brasilvr.com` | POVR/WankzVR | No | Export JSON + listing page dates, uses `povrutil` |
| [IWantClips](https://www.iwantclips.com) | `iwantclips.com/store/{id}/{username}` | IWantClips | Yes | JSON API, double HTML-unescaping |
| [Kink](https://www.kink.com) | `kink.com`, `kink.com/channel/{slug}`, `kink.com/model/{id}/{slug}`, `kink.com/tag/{slug}`, `kink.com/series/{slug}` | Kink | No | HTML scraping, 51 channels, filter by channel/performer/tag/series, detail page worker pool for tags/description/duration, age gate cookie bypass, JSON-LD + data-setup parsing |
| [ManyVids](https://www.manyvids.com) | `manyvids.com/Profile/{id}/{slug}/Store/Videos` | ManyVids | Yes | JSON API, detail-page worker pool |
| [MilfVR](https://www.milfvr.com) | `milfvr.com` | POVR/WankzVR | No | Export JSON + listing page dates, uses `povrutil` |
| [MissaX](https://www.missax.com) | `missax.com` | Custom | No | HTML scraping, listing + detail page worker pool |
| [Mofos](https://www.mofos.com) | `mofos.com`, `mofos.com/pornstar/{id}/{slug}`, `mofos.com/category/{id}/{slug}`, `mofos.com/site/{id}/{slug}`, `mofos.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Mom Comes First](https://momcomesfirst.com) | `momcomesfirst.com` | WordPress | No | Sitemap-driven, JSON-LD VideoObject, uses `wputil` |
| [OopsFamily](https://oopsfamily.com) | `oopsfamily.com`, `oopsfamily.com/model/{slug}`, `oopsfamily.com/tag/{slug}` | FapHouse/Custom | No | HTML listing + detail page JSON-LD worker pool for dates/tags, model/tag filtering, 4K |
| [Naughty America](https://www.naughtyamerica.com) | `naughtyamerica.com`, `naughtyamericavr.com`, `myfriendshotmom.com`, `mysistershotfriend.com`, `tonightsgirlfriend.com`, `thundercock.com` | Naughty America | No | Open JSON API at api.naughtyapi.com, ~15k scenes, 50+ sub-sites, VR support |
| [Nubiles Network](https://nubiles-porn.com) | `nubiles-porn.com`, `nubiles.net`, `momsteachsex.com`, `stepsiblingscaught.com`, `myfamilypies.com`, `princesscum.com`, + 14 more | EdgeCms | No | HTML scraping, 20+ network sites, detail page worker pool for tags/description, filter by model or category |
| [MyDirtyHobby](https://www.mydirtyhobby.com) | `mydirtyhobby.com/profil/{id}-{username}` | MyDirtyHobby | No | JSON API with auth headers |
| [Perfect Girlfriend](https://perfectgirlfriend.com) | `perfectgirlfriend.com` | WordPress | No | Sitemap-driven, JSON-LD VideoObject fallback, uses `wputil` |
| [Pornhub](https://www.pornhub.com) | `pornhub.com/pornstar/{slug}`, `pornhub.com/channels/{slug}` | Pornhub | Free | HTML scraping, minimal fields |
| [Pure Taboo](https://www.puretaboo.com) | `puretaboo.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Rachel Steele](https://rachel-steele.com) | `rachel-steele.com` | MyMember.site | Yes | JSON list API + HTML detail pages, JSON-LD keywords |
| [Reality Kings](https://www.realitykings.com) | `realitykings.com`, `realitykings.com/pornstar/{id}/{slug}`, `realitykings.com/category/{id}/{slug}`, `realitykings.com/site/{id}/{slug}`, `realitykings.com/series/{id}/{slug}` | Aylo/Juan | No | REST API, filter by performer/category/sub-site/series, uses `ayloutil` |
| [Taboo Heat](https://www.tabooheat.com) | `tabooheat.com` | Gamma/Algolia | No | Thin wrapper around `gammautil` |
| [Tara Tainton](https://taratainton.com) | `taratainton.com` | WordPress | Yes | Sitemap-driven, HTML meta parsing, uses `wputil` |
| [Exposed Latinas](https://exposedlatinas.com) | `exposedlatinas.com/tour/updates`, `exposedlatinas.com/tour/models/{slug}.html`, `exposedlatinas.com/tour/categories/{slug}.html` | SexMex Pro CMS | No | Thin wrapper around `sexmexutil` |
| [SexMex](https://sexmex.xxx) | `sexmex.xxx/tour/updates`, `sexmex.xxx/tour/models/{slug}.html`, `sexmex.xxx/tour/categories/{slug}.html` | SexMex Pro CMS | No | HTML scraping with regex, pagination, model/category/full-studio modes, uses `sexmexutil` |
| [Trans Queens](https://transqueens.com) | `transqueens.com/tour/updates`, `transqueens.com/tour/models/{slug}.html`, `transqueens.com/tour/categories/{slug}.html` | SexMex Pro CMS | No | Thin wrapper around `sexmexutil` |
| [TranzVR](https://www.tranzvr.com) | `tranzvr.com` | POVR/WankzVR | No | Thin wrapper around `povrutil` |
| [WankzVR](https://www.wankzvr.com) | `wankzvr.com` | POVR/WankzVR | No | Export JSON + listing page dates, uses `povrutil` |
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
| `ayloutil` | Aylo/Juan (REST API, instance token auth) | Babes, Brazzers, Digital Playground, Mofos, Reality Kings |
| `gammautil` | Gamma Entertainment (Algolia search API) | Gloryhole Secrets, Pure Taboo, Taboo Heat |
| `povrutil` | POVR/WankzVR (export JSON + HTML listing pages) | BrasilVR, MilfVR, TranzVR, WankzVR |
| `sexmexutil` | SexMex Pro CMS (HTML scraping, pagination). **Quirk:** their CMS returns HTTP 500 with valid HTML on some pages (e.g. model pages), so `fetchPage` accepts 500 responses instead of using `httpx.Do`. | Exposed Latinas, SexMex, Trans Queens |
| `veutil` | WordPress video-elements theme (WP REST API for posts + tags, poster extraction from content) | BoyfriendSharing, BrattyFamily, GoStuckYourself, HugeCockBreak, LittleFromAsia, MommysBoy, MomXXX, MyBadMILFs, DaughterSwap, PervMom, SisLovesMe, YoungerLoverOfMine |
| `wputil` | WordPress (sitemap + HTML meta parsing) | Anal Therapy, Family Therapy, Mom Comes First, Perfect Girlfriend, Tara Tainton |

## Adding a new scraper

See the [contributing guide](../CONTRIBUTING.md) for step-by-step instructions and reference implementations.
