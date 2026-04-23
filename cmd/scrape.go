package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/store"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

var scrapeCmd = &cobra.Command{
	Use:   "scrape <studio-url> [studio-url ...]",
	Short: "Scrape all scenes from one or more studio URLs",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runScrape,
}

func init() {
	rootCmd.AddCommand(scrapeCmd)

	scrapeCmd.Flags().IntP("workers", "w", 0, "max parallel fetchers (0 = use config/default)")
	scrapeCmd.Flags().Bool("full", false, "ignore existing data, scrape everything from scratch")
	scrapeCmd.Flags().Bool("refresh", false, "re-fetch metadata for all known scenes, soft-delete missing")
	scrapeCmd.Flags().StringP("output", "o", "", "export formats: json, csv, or json,csv (default from config)")
	scrapeCmd.Flags().String("out", "", "output directory (default from config)")
	scrapeCmd.Flags().String("db", "", "enable SQLite store at this path")
	scrapeCmd.Flags().String("name", "", "human-readable label for this studio (stored when --db is set)")
	scrapeCmd.Flags().Int("delay", 0, "milliseconds between page requests (0 = no delay)")
}

func runScrape(cmd *cobra.Command, args []string) error {
	// --- resolve flags against config ---
	full, _ := cmd.Flags().GetBool("full")
	refresh, _ := cmd.Flags().GetBool("refresh")
	if full && refresh {
		return fmt.Errorf("--full and --refresh are mutually exclusive")
	}

	workers, _ := cmd.Flags().GetInt("workers")
	if workers <= 0 {
		workers = cfg.Workers
	}

	outputFlag, _ := cmd.Flags().GetString("output")
	outputStr := cfg.Output
	if outputFlag != "" {
		outputStr = outputFlag
	}
	formats := parseFormats(outputStr)

	outDir, _ := cmd.Flags().GetString("out")
	if outDir == "" {
		outDir = cfg.OutDir
	}

	dbPath, _ := cmd.Flags().GetString("db")
	if dbPath == "" {
		dbPath = cfg.DB
	}

	name, _ := cmd.Flags().GetString("name")
	if name != "" && len(args) > 1 {
		return fmt.Errorf("--name cannot be used when scraping multiple URLs")
	}
	if name != "" && dbPath == "" {
		fmt.Fprintln(os.Stderr, "warning: --name has no effect without --db (studio names are only stored in SQLite)")
	}

	delayFlag, _ := cmd.Flags().GetInt("delay")
	delayMS := cfg.Delay
	if delayFlag > 0 {
		delayMS = delayFlag
	}
	delay := time.Duration(delayMS) * time.Millisecond

	// --- pick store (opened once, shared across all URLs) ---
	var st store.Store
	if dbPath != "" {
		db, err := store.NewSQLite(dbPath)
		if err != nil {
			return fmt.Errorf("opening database: %w", err)
		}
		defer func() { _ = db.Close() }()
		st = db
	} else {
		st = store.NewFlat(outDir, formats)
	}

	// --- graceful shutdown on Ctrl+C ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var firstErr error
	for i, studioURL := range args {
		if i > 0 {
			fmt.Println()
		}
		if err := scrapeOne(ctx, st, studioURL, name, dbPath, outDir, formats, full, refresh, workers, delay); err != nil {
			fmt.Fprintf(os.Stderr, "error scraping %s: %v\n", studioURL, err)
			if firstErr == nil {
				firstErr = err
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	return firstErr
}

func scrapeOne(ctx context.Context, st store.Store, studioURL, name, dbPath, outDir string, formats []string, full, refresh bool, workers int, delay time.Duration) error {
	sc, err := scraper.ForURL(studioURL)
	if err != nil {
		return err
	}

	var scenes []models.Scene
	switch {
	case full:
		fmt.Printf("Full scrape: %s\n", studioURL)
		scenes, err = scrapeAll(ctx, sc, studioURL, workers, delay)
	case refresh:
		fmt.Printf("Refresh scrape: %s\n", studioURL)
		scenes, err = scrapeRefresh(ctx, sc, st, studioURL, workers, delay)
	default:
		fmt.Printf("Incremental scrape: %s\n", studioURL)
		scenes, err = scrapeIncremental(ctx, sc, st, studioURL, workers, delay)
	}
	if err != nil {
		return err
	}

	if name == "" && len(scenes) > 0 && scenes[0].Studio != "" {
		name = scenes[0].Studio
	}

	if err := st.Save(studioURL, scenes); err != nil {
		return fmt.Errorf("saving results: %w", err)
	}

	if dbPath != "" {
		slug := store.Slugify(studioURL)
		for _, format := range formats {
			path := filepath.Join(outDir, slug+"."+format)
			if err := st.Export(format, path, studioURL); err != nil {
				return fmt.Errorf("exporting %s: %w", format, err)
			}
		}
	}

	now := time.Now().UTC()
	if uErr := st.UpsertStudio(models.Studio{
		URL:           studioURL,
		SiteID:        sc.ID(),
		Name:          name,
		AddedAt:       now,
		LastScrapedAt: &now,
	}); uErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update studio record: %v\n", uErr)
	}

	if ctx.Err() != nil {
		fmt.Printf("Partial save complete: %d scenes.\n", len(scenes))
	} else {
		fmt.Printf("Done: %d scenes saved.\n", len(scenes))
	}
	return nil
}

// scrapeAll fetches every scene from scratch, ignoring any existing data.
func scrapeAll(ctx context.Context, sc scraper.StudioScraper, studioURL string, workers int, delay time.Duration) ([]models.Scene, error) {
	return collectScenes(ctx, sc, studioURL, scraper.ListOpts{Workers: workers, Delay: delay})
}

// scrapeIncremental loads existing scene IDs, passes them to the scraper as a
// hint for early-stop optimisation (date-sorted sites), then merges results.
//
// Scrapers that cannot use early-stop (e.g. recommended-sorted sites) may emit
// known scenes in correct site order. In that case fresh takes priority and
// price history is carried forward so no history is lost.
func scrapeIncremental(ctx context.Context, sc scraper.StudioScraper, st store.Store, studioURL string, workers int, delay time.Duration) ([]models.Scene, error) {
	existing, err := st.Load(studioURL)
	if err != nil {
		return nil, fmt.Errorf("loading existing scenes: %w", err)
	}

	knownIDs := make(map[string]bool, len(existing))
	existingByID := make(map[string]models.Scene, len(existing))
	for _, s := range existing {
		knownIDs[s.ID] = true
		existingByID[s.ID] = s
	}

	fresh, err := collectScenes(ctx, sc, studioURL, scraper.ListOpts{Workers: workers, KnownIDs: knownIDs, Delay: delay})
	if err != nil {
		return nil, err
	}

	// Merge: emit fresh scenes first (preserving scraper order); carry price
	// history when a fresh scene was already stored (happens when a scraper
	// re-emits known IDs rather than skipping them). Append any existing scenes
	// that were not re-emitted (older entries on date-sorted sites).
	freshIDs := make(map[string]bool, len(fresh))
	result := make([]models.Scene, 0, len(fresh)+len(existing))
	newCount := 0
	for _, s := range fresh {
		freshIDs[s.ID] = true
		if prev, ok := existingByID[s.ID]; ok {
			s = carryOverPriceHistory(s, prev)
		} else {
			newCount++
		}
		result = append(result, s)
	}
	for _, s := range existing {
		if !freshIDs[s.ID] {
			result = append(result, s)
		}
	}

	fmt.Printf("  %d new, %d existing → %d total\n", newCount, len(existing), len(result))
	return result, nil
}

// scrapeRefresh re-fetches all scenes and soft-deletes any that have disappeared.
// Price history from prior scrapes is carried forward onto each re-fetched scene.
func scrapeRefresh(ctx context.Context, sc scraper.StudioScraper, st store.Store, studioURL string, workers int, delay time.Duration) ([]models.Scene, error) {
	existing, err := st.Load(studioURL)
	if err != nil {
		return nil, fmt.Errorf("loading existing scenes: %w", err)
	}
	existingByID := make(map[string]models.Scene, len(existing))
	for _, s := range existing {
		existingByID[s.ID] = s
	}

	// Full traversal — no KnownIDs
	fresh, err := collectScenes(ctx, sc, studioURL, scraper.ListOpts{Workers: workers, Delay: delay})
	if err != nil {
		return nil, err
	}

	// Build result: fresh scenes with accumulated price history.
	scrapedIDs := make(map[string]bool, len(fresh))
	result := make([]models.Scene, 0, len(existing))
	for _, s := range fresh {
		scrapedIDs[s.ID] = true
		if prev, ok := existingByID[s.ID]; ok {
			s = carryOverPriceHistory(s, prev)
		}
		result = append(result, s)
	}

	// Carry forward previously-deleted scenes; mark newly-missing ones as deleted.
	now := time.Now().UTC()
	newlyDeleted := 0
	for _, s := range existing {
		if scrapedIDs[s.ID] {
			continue
		}
		if s.DeletedAt == nil {
			s.DeletedAt = &now
			newlyDeleted++
		}
		result = append(result, s)
	}

	if newlyDeleted > 0 {
		fmt.Printf("  %d scenes no longer found, marked deleted\n", newlyDeleted)
	}
	fmt.Printf("  %d scraped, %d total\n", len(fresh), len(result))
	return result, nil
}

// collectScenes drains the scraper channel, printing a live count and warnings.
func collectScenes(ctx context.Context, sc scraper.StudioScraper, studioURL string, opts scraper.ListOpts) ([]models.Scene, error) {
	ch, err := sc.ListScenes(ctx, studioURL, opts)
	if err != nil {
		return nil, fmt.Errorf("starting scrape: %w", err)
	}
	var scenes []models.Scene
	errCount := 0
	total := 0
	stoppedEarly := false
	for result := range ch {
		if result.Total > 0 {
			total = result.Total
			continue
		}
		if result.StoppedEarly {
			stoppedEarly = true
			continue
		}
		if result.Err != nil {
			errCount++
			fmt.Fprintf(os.Stderr, "\rwarning: %v\n", result.Err)
			continue
		}
		scenes = append(scenes, result.Scene)
		if total > 0 {
			fmt.Printf("\r  fetching: %d / %d scenes", len(scenes), total)
		} else {
			fmt.Printf("\r  fetching: %d scenes", len(scenes))
		}
	}
	fmt.Println() // end the progress line
	if stoppedEarly {
		fmt.Println("  stopped early at known ID — remaining scenes already stored")
	}
	if errCount > 0 {
		fmt.Fprintf(os.Stderr, "  %d fetch error(s) — see warnings above\n", errCount)
	}
	if ctx.Err() != nil {
		fmt.Printf("Interrupted — saving %d partial results...\n", len(scenes))
	}
	return scenes, nil
}

// carryOverPriceHistory prepends the existing scene's price history onto a
// freshly scraped scene, then appends the new snapshot so LowestPrice is correct.
func carryOverPriceHistory(fresh, existing models.Scene) models.Scene {
	if len(fresh.PriceHistory) == 0 {
		return fresh
	}
	newSnap := fresh.PriceHistory[len(fresh.PriceHistory)-1]
	fresh.PriceHistory = existing.PriceHistory
	fresh.LowestPrice = existing.LowestPrice
	fresh.LowestPriceDate = existing.LowestPriceDate
	fresh.AddPrice(newSnap)
	return fresh
}

// parseFormats splits "json,csv" into ["json","csv"], trimming spaces and deduplicating.
func parseFormats(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range strings.Split(s, ",") {
		f = strings.TrimSpace(f)
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}
