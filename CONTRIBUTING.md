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

### 6. Use the shared HTTP layer

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

### 7. Write tests

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

For optional live integration tests that hit the real site API, use a build tag:

```go
//go:build integration

func TestLive(t *testing.T) { ... }
```

Run with: `go test -tags integration -v -timeout 120s ./internal/scrapers/<site>/...`

### 8. Verify

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
| `manyvids` | Medium | JSON API, pricing, detail-page worker pool |
| `clips4sale` | Medium | Multi-page HTML, categories, pricing |
| `iwantclips` | Medium | JSON API, double HTML-unescaping |
| `mydirtyhobby` | Medium | JSON API with auth headers |
| `taratainton` | Medium | WordPress/sitemap-driven discovery, HTML meta parsing, worker pool |
