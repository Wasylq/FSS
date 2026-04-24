# Supported Scrapers

| Site | URL Pattern | Platform | Pricing | Notes |
|------|-------------|----------|---------|-------|
| [A POV Story](https://apovstory.com) | `apovstory.com` | PHP tour site | No | HTML listing + detail pages, category extraction |
| [Clips4Sale](https://www.clips4sale.com) | `clips4sale.com/studio/{id}/{slug}` | Clips4Sale | Yes | Multi-page JSON, categories, all-page enumeration |
| [IWantClips](https://www.iwantclips.com) | `iwantclips.com/store/{id}/{username}` | IWantClips | Yes | JSON API, double HTML-unescaping |
| [ManyVids](https://www.manyvids.com) | `manyvids.com/Profile/{id}/{slug}/Store/Videos` | ManyVids | Yes | JSON API, detail-page worker pool |
| [MissaX](https://www.missax.com) | `missax.com` | Custom | No | HTML scraping, listing + detail page worker pool |
| [Mom Comes First](https://momcomesfirst.com) | `momcomesfirst.com` | WordPress | No | Sitemap-driven, JSON-LD VideoObject, uses `wputil` |
| [MyDirtyHobby](https://www.mydirtyhobby.com) | `mydirtyhobby.com/profil/{id}-{username}` | MyDirtyHobby | No | JSON API with auth headers |
| [Pornhub](https://www.pornhub.com) | `pornhub.com/pornstar/{slug}`, `pornhub.com/channels/{slug}` | Pornhub | Free | HTML scraping, minimal fields |
| [Pure Taboo](https://www.puretaboo.com) | `puretaboo.com` | Gamma/Algolia | No | Algolia search API, uses `gammautil` |
| [Rachel Steele](https://rachel-steele.com) | `rachel-steele.com` | MyMember.site | Yes | JSON list API + HTML detail pages, JSON-LD keywords |
| [Taboo Heat](https://www.tabooheat.com) | `tabooheat.com` | Gamma/Algolia | No | Thin wrapper around `gammautil` |
| [Tara Tainton](https://taratainton.com) | `taratainton.com` | WordPress | Yes | Sitemap-driven, HTML meta parsing, uses `wputil` |

## Shared scraper packages

Scrapers that share a platform use common utility packages to avoid duplication:

| Package | Platform | Used by |
|---------|----------|---------|
| `gammautil` | Gamma Entertainment (Algolia search API) | Pure Taboo, Taboo Heat |
| `wputil` | WordPress (sitemap + HTML meta parsing) | Tara Tainton, Mom Comes First |

## Adding a new scraper

See the [contributing guide](../CONTRIBUTING.md) for step-by-step instructions and reference implementations.
