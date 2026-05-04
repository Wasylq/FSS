# Contributing

## Adding a New Scraper

Each site scraper is a self-contained package under `internal/scrapers/<site>/`. The system uses a plugin-based registry — no central config to edit beyond a one-line import.

### 1. Create the package

Create `internal/scrapers/<site>/<site>.go`. Your scraper must implement the `scraper.StudioScraper` interface:

```go
type StudioScraper interface {
    ID() string
    Patterns() []string
    MatchesURL(url string) bool
    ListScenes(ctx context.Context, studioURL string, opts ListOpts) (<-chan SceneResult, error)
}
```

- **`ID()`** — stable lowercase identifier, e.g. `"pornhub"`
- **`Patterns()`** — human-readable URL patterns shown by `fss list-scrapers`, e.g. `"pornhub.com/pornstar/{slug}"`
- **`MatchesURL()`** — returns true if this scraper handles the given URL (use a compiled regex)
- **`ListScenes()`** — starts a goroutine that sends results on a channel, returns the channel immediately

### 2. Register via init()

```go
func init() {
    scraper.Register(New())
}
```

This is called automatically at startup when the package is imported.

### 3. Add blank import in main.go

```go
_ "github.com/Wasylq/FSS/internal/scrapers/<site>"
```

This triggers `init()` and registers the scraper. Without this line, the scraper won't be available.

### 4. Implement the run() goroutine

The `ListScenes` method launches a goroutine that sends `SceneResult` values on a channel. See `internal/scrapers/pornhub/pornhub.go` for the simplest complete example.

Required pattern:

```go
func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
    out := make(chan scraper.SceneResult)
    go s.run(ctx, studioURL, opts, out)
    return out, nil
}

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
    defer close(out) // MUST be the first line

    for page := 1; ; page++ {
        if ctx.Err() != nil {
            return
        }

        // Respect delay between pages
        if page > 1 && opts.Delay > 0 {
            select {
            case <-time.After(opts.Delay):
            case <-ctx.Done():
                return
            }
        }

        items, err := s.fetchPage(ctx, studioURL, page)
        if err != nil {
            select {
            case out <- scraper.SceneResult{Err: err}:
            case <-ctx.Done():
            }
            return
        }

        if len(items) == 0 {
            return // no more pages
        }

        // Send total hint once (first page only) for progress display
        if page == 1 && totalCount > 0 {
            select {
            case out <- scraper.SceneResult{Total: totalCount}:
            case <-ctx.Done():
                return
            }
        }

        now := time.Now().UTC()
        for _, item := range items {
            // Incremental mode: stop when we hit a known ID
            if len(opts.KnownIDs) > 0 && opts.KnownIDs[item.id] {
                select {
                case out <- scraper.SceneResult{StoppedEarly: true}:
                case <-ctx.Done():
                }
                return
            }

            scene := toScene(studioURL, item, now)
            select {
            case out <- scraper.SceneResult{Scene: scene}:
            case <-ctx.Done():
                return
            }
        }
    }
}
```

**Critical rules:**

- `defer close(out)` must be the first line in `run()` — the consumer blocks on this channel
- Every channel send must be wrapped in `select` with `case <-ctx.Done(): return` to prevent goroutine leaks on cancellation
- Send `SceneResult{Total: n}` once after the first page so the CLI can show progress
- Send `SceneResult{StoppedEarly: true}` when hitting a known ID in incremental mode

### 5. Build the Scene

Populate `models.Scene` with as many fields as the site provides. Required fields:

| Field | Description |
|-------|-------------|
| `ID` | Unique identifier from the site |
| `SiteID` | Your scraper's `ID()` value |
| `StudioURL` | The input studio URL |
| `Title` | Scene title |
| `URL` | Direct link to the scene |
| `ScrapedAt` | `time.Now().UTC()` |

Optional but recommended: `Date`, `Description`, `Thumbnail`, `Preview`, `Performers`, `Tags`, `Categories`, `Duration`, `Width`, `Height`, `Resolution`, `Format`, `Studio`.

For sites with pricing, call `scene.AddPrice()`:

```go
scene.AddPrice(models.PriceSnapshot{
    Date:            now,
    Regular:         19.99,
    Discounted:      9.99,
    IsFree:          false,
    IsOnSale:        true,
    DiscountPercent: 50,
})
```

For free sites (e.g. Pornhub):

```go
scene.AddPrice(models.PriceSnapshot{Date: now, IsFree: true})
```

### 6. WordPress sites — use wputil

For WordPress-based sites, the `internal/scrapers/wputil` package provides shared helpers:

- `wputil.BrowserHeaders()` — common browser headers to avoid WAF blocks
- `wputil.FetchSitemap()` / `wputil.FetchAllSitemaps()` — XML sitemap parsing
- `wputil.FetchPage()` — fetch a single page
- `wputil.ParseMeta(body, titleSuffix)` — extract OpenGraph, `article:tag`, `article:published_time`, shortlink post ID, JSON-LD `VideoObject` width/height, and `articleSection` categories
- `wputil.RunWorkerPool()` — sitemap discovery + parallel page fetching with a `PageParser` callback
- `wputil.SlugFromURL()`, `wputil.ParseDuration()`, `wputil.VideoWidth()` — utility helpers

See `taratainton` and `momcomesfirst` for examples. Your scraper only needs to implement the site-specific `parsePage` callback and registration.

### 7. Use the shared HTTP layer

All scrapers should use the shared HTTP client:

```go
import "github.com/Wasylq/FSS/internal/httpx"

// In New():
client: httpx.NewClient(30 * time.Second)

// In fetch methods:
resp, err := httpx.Do(ctx, s.client, httpx.Request{
    URL: pageURL,
    Headers: map[string]string{
        "User-Agent": httpx.UserAgentFirefox,
    },
})
```

This gives you connection pooling, automatic retries with backoff (0s/2s/4s), and fail-fast on non-retryable 4xx errors.

### 8. Write tests

Create `internal/scrapers/<site>/<site>_test.go`. Tests should be offline — use `httptest.NewServer` to serve fixture HTML/JSON responses:

```go
func TestParsing(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write(fixtureHTML)
    }))
    defer ts.Close()
    // test against ts.URL
}
```

What to test:

- URL matching (`MatchesURL` with valid and invalid URLs)
- HTML/JSON parsing (page parsing, edge cases, missing fields)
- Pagination (multi-page responses, empty last page)
- `KnownIDs` early stopping

For live integration smoke tests that hit the real site, use the shared `testutil` helper. Each scraper has an `integration_test.go` like:

```go
//go:build integration

package <site>

import (
    "testing"
    "github.com/Wasylq/FSS/internal/scrapers/testutil"
)

// liveStudioURL — pick a stable studio. Update if it 404s.
const liveStudioURL = "https://example.com/profile/123/some-name"

func TestLive<Site>(t *testing.T) {
    testutil.SkipIfPlaceholder(t, liveStudioURL)
    testutil.RunLiveScrape(t, New(), liveStudioURL, 2)
}
```

`testutil.RunLiveScrape` fetches the first 2 scenes, validates each via `testutil.ValidateScene` (non-empty `ID`/`Title`/`URL`/`Date`, plausible `Duration`, etc.), and logs the first scene's full struct so you can eyeball field mappings on `-v`. `SkipIfPlaceholder` skips cleanly when `liveStudioURL` still contains `REPLACE-ME` — use it for new scrapers until you find a stable URL.

Run all of them:

```bash
make smoke              # all scrapers + Stash
make smoke-one SCRAPER=<site>   # one scraper
make smoke-stash        # Stash integration only
```

These are **never run in CI** (Cloudflare blocks shared GitHub-runner IP ranges, and they hit live sites / local services). They're a manual pre-release check.

#### Stash integration tests

The Stash integration tests (`stash/integration_test.go`) verify that the GraphQL client works against your real Stash instance. They are **read-only** — no tags, performers, studios, or scenes are created or modified.

```bash
# Default: connects to http://localhost:9999, no auth
make smoke-stash

# Custom URL and/or API key
FSS_STASH_URL=http://192.168.1.50:9999 make smoke-stash
FSS_STASH_URL=http://192.168.1.50:9999 FSS_STASH_API_KEY=yourkey make smoke-stash
```

If Stash isn't reachable, all tests skip gracefully — no failures.

### 9. Update docs

Add a row to [docs/scrapers.md](docs/scrapers.md) with the site name, URL pattern, platform, and notes.

### 10. Verify

```bash
go build ./...                           # compiles
go vet ./...                             # static analysis
go test -race -count=1 ./...             # all tests pass
go build -o fss . && ./fss list-scrapers # new scraper appears
```

### Reference implementations

| Scraper | Complexity | Good example of |
|---------|-----------|-----------------|
| `pornhub` | Simple | HTML scraping, minimal fields, free content |
| `momcomesfirst` | Simple | WordPress site using `wputil` shared package, JSON-LD VideoObject |
| `babes` | Simple | Thin wrapper around `ayloutil` for an Aylo/Juan site |
| `digitalplayground` | Simple | Thin wrapper around `ayloutil` for an Aylo/Juan site |
| `mofos` | Simple | Thin wrapper around `ayloutil` for an Aylo/Juan site |
| `realitykings` | Simple | Thin wrapper around `ayloutil` for an Aylo/Juan site |
| `tabooheat` | Simple | Thin wrapper around `gammautil` for a Gamma Entertainment site |
| `naughtyamerica` | Medium | Open JSON API, paginated, multi-domain (6 sister sites), VR support, thumbnail URL construction from trailer paths |
| `nubiles` | Medium | EdgeCms HTML scraping, 20+ network domains, detail page worker pool, model/category URL filtering |
| `bangbros` | Medium | Aylo/Juan REST API with slug-to-ID resolution for `/websites/` and `/category/` URLs, uses `ayloutil` |
| `brazzers` | Medium | Aylo/Juan REST API, instance token auth, multi-filter URL parsing, series support, uses `ayloutil` |
| `loyalfans` | Medium | POST-based JSON API, cursor pagination (`page_token`), session init, owner filtering |
| `apclips` | Medium | HTML scraping, listing + detail pages for dates/tags, price tracking |
| `faphouse` | Medium | HTML listing + detail pages with embedded JSON (`view-state-data`), model/studio URL types, price tracking |
| `apovstory` | Medium | PHP tour site, HTML listing + detail pages, category extraction |
| `manyvids` | Medium | JSON API, pricing, detail-page worker pool |
| `clips4sale` | Medium | Multi-page HTML, categories, pricing |
| `iwantclips` | Medium | JSON API, double HTML-unescaping |
| `mydirtyhobby` | Medium | JSON API with auth headers |
| `taratainton` | Medium | WordPress/sitemap-driven discovery, HTML meta parsing, worker pool, uses `wputil` |
| `missax` | Medium | HTML scraping, listing + detail page worker pool, no API |
| `puretaboo` | Medium | Algolia search API, session API key extraction, rich structured JSON, uses `gammautil` |
| `rachelsteele` | Medium | MyMember.site SaaS platform, JSON list API + HTML detail pages, JSON-LD keywords parsing |

---

## Cutting a release

Releases are tagged with `vMAJOR.MINOR.PATCH`. Pushing the tag triggers `.github/workflows/release.yml`, which builds the cross-platform binaries and `.deb`/`.rpm` packages automatically and then **pauses for manual approval** before publishing.

### Steps

```bash
git tag -a v1.7.0 -m "v1.7.0"
git push origin v1.7.0
```

Then go to the **Actions → Release** run on GitHub, click *Review deployments*, tick `manual-smoke-gate`, and approve. The GitHub Release is published (with tarballs, zips, `.deb`, and `.rpm` packages), the AUR `fss` package is updated, and the Docker image (with semantic version tags) is built and pushed — all in the same run. Everything happens behind the single approval gate.

### What the release produces

| Artifact | Platforms |
|----------|-----------|
| `.tar.gz` binaries | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 |
| `.zip` binary | windows/amd64 |
| `.deb` package | linux/amd64, linux/arm64 |
| `.rpm` package | linux/amd64, linux/arm64 |
| AUR `fss` package | auto-published after release |
| Docker image (`ghcr.io/wasylq/fss`) | linux/amd64, linux/arm64 |

The `.deb`/`.rpm` packages are built by [nfpm](https://nfpm.goreleaser.com/) using `nfpm.yaml`. To test locally:

```bash
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
GOOS=linux GOARCH=amd64 go build -o dist/fss .
GOARCH=amd64 VERSION=1.7.0 nfpm package --packager deb --target dist/
```

### Approver checklist

Before clicking approve, confirm:

- [ ] `make smoke` (scrapers + Stash integration) passed locally — CI cannot run these because Cloudflare blocks shared GitHub runner IP ranges and Stash is a local service.
- [ ] `CHANGELOG`/release notes accurately describe user-visible changes.
- [ ] No known regressions in any of the high-severity checks you track.
- [ ] The new binary's `fss version` shows the expected tag when run locally.

The gate is a **trust-me** check — nothing verifies that you actually ran the smoke tests. Its only job is to force a pause-and-think before a release goes public.

### AUR and Homebrew

Packaging files live in `packaging/`:

- `packaging/aur/PKGBUILD` — Arch Linux AUR package. **Automatically published** to the AUR after the GitHub Release is created (via `KSXGitHub/github-actions-deploy-aur` action). Requires `AUR_SSH_PRIVATE_KEY` secret in the repository.
- `packaging/homebrew/fss.rb` — Reference Homebrew formula. For a proper tap, create a `homebrew-fss` repository and publish the formula there after each release.

Both AUR and Homebrew support system-level updates (`yay -Syu` / `brew upgrade`). For `.deb`/`.rpm` auto-updates via `apt upgrade`/`dnf upgrade`, a hosted package repository (e.g. Packagecloud, Cloudsmith, or Gemfury) is needed — see `docs/enhancements.md`.

### One-time setup (per maintainer / per fork)

The `manual-smoke-gate` environment must exist in the GitHub repository before the workflow can pause on it. To create it:

1. Repository → **Settings → Environments → New environment**, name it `manual-smoke-gate`.
2. Under **Deployment protection rules**, tick *Required reviewers* and add yourself (and any co-maintainers).
3. Save. No environment secrets are needed.

Without this, the release workflow will fail with `Environment "manual-smoke-gate" not found` on the first tag push. Environment protection rules with required reviewers are free for public repositories.
