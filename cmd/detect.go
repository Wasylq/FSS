package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/scraper"
)

var detectCmd = &cobra.Command{
	Use:   "detect <url>",
	Short: "Fetch a URL and detect which platform/CMS it uses",
	Long: `Fetches the given URL once and checks the response for known platform
signals (Aylo instance_token, Algolia API, psmcdn.net, ModelCentro, etc.).

Reports the detected platform and the corresponding util package, or
"unknown" if no signal matches. Useful when deciding whether a new site
needs a standalone scraper or belongs to an existing shared package.`,
	Args: cobra.ExactArgs(1),
	RunE: runDetect,
}

func init() {
	rootCmd.AddCommand(detectCmd)
}

type detection struct {
	platform string
	pkg      string
	detail   string
}

func runDetect(cmd *cobra.Command, args []string) error {
	rawURL := args[0]
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	w := cmd.OutOrStdout()

	// Check if already supported by a registered scraper.
	if s, err := scraper.ForURL(rawURL); err == nil {
		_, _ = fmt.Fprintf(w, "Already supported by scraper: %s\n", s.ID())
		_, _ = fmt.Fprintf(w, "Patterns: %s\n", strings.Join(s.Patterns(), ", "))
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := httpx.NewClient(30 * time.Second)
	resp, err := httpx.Do(ctx, client, httpx.Request{
		URL:     rawURL,
		Headers: httpx.BrowserHeaders(httpx.UserAgentFirefox),
	})
	if err != nil {
		return fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := httpx.ReadBody(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	page := string(body)
	cookies := resp.Cookies()

	detections := detectPlatform(page, cookies, resp.Header)

	if len(detections) == 0 {
		_, _ = fmt.Fprintln(w, "Platform: unknown")
		_, _ = fmt.Fprintln(w, "No known platform signals detected. Build a standalone scraper.")
		return nil
	}

	for _, d := range detections {
		_, _ = fmt.Fprintf(w, "Platform: %s\n", d.platform)
		_, _ = fmt.Fprintf(w, "Package:  %s\n", d.pkg)
		if d.detail != "" {
			_, _ = fmt.Fprintf(w, "Detail:   %s\n", d.detail)
		}
		_, _ = fmt.Fprintln(w)
	}
	return nil
}

// TODO: response headers are fetched and passed in but not yet used as a
// detection signal. Server / X-Powered-By / Set-Cookie names would identify
// several CMSes that currently require body-content matching. Renamed to _
// so the unused-parameter lint stays green until that lands.
func detectPlatform(page string, cookies []*http.Cookie, _ http.Header) []detection {
	var results []detection
	lp := strings.ToLower(page)

	// Aylo/Juan — instance_token cookie
	for _, c := range cookies {
		if c.Name == "instance_token" {
			results = append(results, detection{"Aylo/Juan", "ayloutil", "instance_token cookie found"})
			break
		}
	}

	// TeamSkeet/PSM — psmcdn.net
	if strings.Contains(lp, "psmcdn.net") {
		results = append(results, detection{"TeamSkeet/PSM", "teamskeetutil", "psmcdn.net CDN detected"})
	}

	// Gamma/Algolia
	if strings.Contains(page, "TSMKFA364Q") || (strings.Contains(lp, "algolia.net") && strings.Contains(lp, "applicationid")) {
		detail := "Algolia API detected"
		if strings.Contains(page, "TSMKFA364Q") {
			detail = "Algolia applicationID TSMKFA364Q"
		}
		results = append(results, detection{"Gamma Entertainment", "gammautil", detail})
	}

	// ModelCentro
	if strings.Contains(lp, "centrofiles.com") || strings.Contains(page, "fox.createApplication") || strings.Contains(lp, "/api/content.load") {
		results = append(results, detection{"ModelCentro", "modelcentroutil", ""})
	}

	// Adult Prime
	if strings.Contains(lp, "cdnstatic.imctransfer.com") || strings.Contains(lp, "portal-video-wrapper") {
		results = append(results, detection{"Adult Prime", "adultprimeutil", ""})
	}

	// FYC/PornPros — __NUXT_DATA__ + pornpros CDN
	if strings.Contains(page, `id="__NUXT_DATA__"`) {
		detail := "__NUXT_DATA__ tag found"
		if strings.Contains(lp, "pornpros") || strings.Contains(lp, "fuckyoucash") {
			detail += " + FYC/PornPros signals"
		}
		results = append(results, detection{"FYC/PornPros (Nuxt)", "fycutil", detail})
	}

	// Next.js __NEXT_DATA__ (KB Productions, Ghost Pro, Wank It Now, etc.)
	if strings.Contains(page, `id="__NEXT_DATA__"`) {
		detail := "__NEXT_DATA__ tag found"
		if strings.Contains(lp, "mjedge.net") {
			detail += " + mjedge.net CDN"
			if strings.Contains(lp, "wankitnow") {
				results = append(results, detection{"Wank It Now", "wankitnowutil", detail})
			}
		}
		if strings.Contains(lp, "yppcdn.com") || strings.Contains(lp, "nats_site_id") {
			results = append(results, detection{"Next.js Paysite (Ghost Pro / KB Productions)", "ghostpro / kbproductions", detail})
		}
	}

	// Score Group
	if strings.Contains(lp, "scoreland.com") || strings.Contains(lp, "scoregroup.com") || strings.Contains(lp, "score-group") {
		results = append(results, detection{"Score Group", "scoregrouputil", ""})
	}

	// MetArt Network
	if strings.Contains(lp, "metartnetwork.com") || strings.Contains(lp, "gccdn.metartnetwork.com") {
		results = append(results, detection{"MetArt Network", "metartutil", ""})
	}

	// Up-Timely CMS
	if strings.Contains(lp, "cdn.up-timely.com") || strings.Contains(lp, "p-workpage__title") {
		results = append(results, detection{"Up-Timely CMS", "uptimelyutil", ""})
	}

	// Czech AV / HQ Media Go
	if strings.Contains(lp, "hqmediago.com") || strings.Contains(lp, "cdn77.hqmediago.com") {
		results = append(results, detection{"Czech AV / HQ Media Go", "czechavutil", ""})
	}

	// Teen Mega World
	if strings.Contains(lp, "teenmegaworld") {
		results = append(results, detection{"Teen Mega World", "tmwutil", ""})
	}

	// Full Porn Network
	if strings.Contains(lp, "fullpornnetwork.com") || strings.Contains(lp, "fpncash.com") {
		results = append(results, detection{"Full Porn Network", "fpnutil", ""})
	}

	// Grooby
	if strings.Contains(lp, "grooby.com") && strings.Contains(lp, "set-target-") {
		results = append(results, detection{"Grooby CMS", "groobyutil", ""})
	}

	// Jules Jordan Network
	if strings.Contains(lp, "julesjordan.com") || strings.Contains(lp, "jj-content-card") {
		results = append(results, detection{"Jules Jordan Network", "julesjordanutil", ""})
	}

	// SexMex Pro
	if strings.Contains(lp, "sexmex.xxx") || strings.Contains(lp, "sexmexpro") {
		results = append(results, detection{"SexMex Pro CMS", "sexmexutil", ""})
	}

	// POVR/WankzVR
	if strings.Contains(lp, "povr.com") || strings.Contains(lp, "wankzvr.com") {
		results = append(results, detection{"POVR/WankzVR", "povrutil", ""})
	}

	// Railway/Express
	if strings.Contains(lp, "sites-api-production.up.railway.app") {
		results = append(results, detection{"Railway/Express", "railwayutil", ""})
	}

	// New Sensations
	if strings.Contains(lp, "newsensations.com") && strings.Contains(lp, "videothumb_") {
		results = append(results, detection{"New Sensations", "newsensationsutil", ""})
	}

	// Wow Network
	if strings.Contains(lp, "wowmodels.com") {
		results = append(results, detection{"Wow Network", "wownetworkutil", ""})
	}

	// VNA Girls
	if strings.Contains(lp, "vnagirls.com") || strings.Contains(lp, "stickydollars.htm") {
		results = append(results, detection{"VNA Girls", "vnautil", ""})
	}

	// MissaX CMS
	if strings.Contains(lp, "missax") && strings.Contains(lp, "photo-thumb video-thumb") {
		results = append(results, detection{"MissaX CMS", "missaxutil", ""})
	}

	// Cherry Pimps
	if strings.Contains(lp, "cherrypimps.com") || (strings.Contains(lp, "elx_styles.css") && strings.Contains(lp, "tourhelper.js")) {
		results = append(results, detection{"Cherry Pimps", "cherrypimpsutil", ""})
	}

	// Real Spankings
	if strings.Contains(lp, "realspankingsnetwork.com") || strings.Contains(lp, "alpine entertainment group") {
		results = append(results, detection{"Real Spankings", "realspankingsutil", ""})
	}

	// FTV
	if strings.Contains(lp, "ftvcash.com") || strings.Contains(lp, "cdn.ftvgirls.com") || strings.Contains(lp, "cdn.ftvmilfs.com") {
		results = append(results, detection{"FTV", "ftvutil", ""})
	}

	// Wankz
	if strings.Contains(lp, "images.wankz.com") || strings.Contains(lp, "images.lethalpass.com") {
		results = append(results, detection{"Wankz", "wankzutil", ""})
	}

	// UTG/Glamose
	if strings.Contains(lp, "assets.utgnetworks.com") || strings.Contains(lp, "utg networks ltd") {
		results = append(results, detection{"UTG Networks / Glamose", "utgutil", ""})
	}

	// Pornstar Platinum
	if strings.Contains(lp, "pornstarplatinum.com") {
		results = append(results, detection{"Pornstar Platinum", "pornstarplatinum", ""})
	}

	// My Gay Cash NATS
	if strings.Contains(lp, "nats.mygaycash.com") || strings.Contains(lp, "natscms-app") {
		results = append(results, detection{"My Gay Cash NATS CMS", "marsmedia", ""})
	}

	// Puba
	if strings.Contains(lp, "puba.com") {
		results = append(results, detection{"Puba", "puba", ""})
	}

	// WordPress (generic — check last)
	if strings.Contains(lp, "/wp-json/") || strings.Contains(lp, "/wp-content/") || strings.Contains(lp, "wp-includes") {
		detail := "WordPress detected"
		if strings.Contains(lp, "video-elements") {
			results = append(results, detection{"WP video-elements", "veutil", detail + " + video-elements theme"})
		} else {
			results = append(results, detection{"WordPress", "wputil (standalone)", detail})
		}
	}

	// Spizoo
	if strings.Contains(lp, "spizoo.com") {
		results = append(results, detection{"Spizoo", "spizooutil", ""})
	}

	// Vixen MG
	if strings.Contains(lp, "vixen.com") || strings.Contains(lp, "blacked.com") || strings.Contains(lp, "tushy.com") {
		results = append(results, detection{"Vixen Media Group", "vixenutil", ""})
	}

	// Nasty Media Group
	if strings.Contains(page, "WYSIWYG Web Builder 18") || strings.Contains(lp, "nasty media group") {
		results = append(results, detection{"Nasty Media Group (WWB18)", "nastymedia", ""})
	}

	return results
}
