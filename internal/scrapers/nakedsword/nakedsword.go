// Package nakedsword scrapes nakedsword.com.
//
// NakedSword is part of the Falcon | NakedSword group, but unlike its sibling
// sites (falconstudios.com, hothouse.com, ragingstallion.com, …, all handled by
// the gamma scraper on the shared segment:falconstudios Algolia key) it runs its
// own React SPA backed by a REST API at ns-api.nakedsword.com. There is no
// server-rendered markup to parse — the page is an empty <div id="root"> — so
// this scraper talks to that API directly.
//
// The API requires an "x-ident" whitelabel header. The site's own bundle builds
// it by AES-encrypting {"date":<unix ms>,"propertyId":"1"} under a key derived
// from a passphrase shipped in the public JavaScript; see xIdent below.
package nakedsword

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wasylq/FSS/internal/httpx"
	"github.com/Wasylq/FSS/models"
	"github.com/Wasylq/FSS/scraper"
)

const (
	siteID     = "nakedsword"
	studioName = "NakedSword"
	siteBase   = "https://www.nakedsword.com"

	// perPage is the largest page size the feed honours; the default is 12,
	// which would turn a full catalogue walk into ~3900 requests instead of
	// ~470.
	perPage = 100

	// propertyID identifies this whitelabel to the API (REACT_APP_PROPERTY_ID).
	propertyID = "1"

	// passphrase is not a secret: it ships in the site's public JS bundle and
	// is the same for every visitor. It identifies the whitelabel, not a user,
	// and grants no access beyond the public catalogue the SPA already renders.
	// If the site rotates it, this scraper starts getting 403 "Bad Whitelabel
	// Identification" and the new value has to be read out of env.js/main.js.
	passphrase = "4238e#5a7bfc9209X894890e073a3&&12ea8+b@c"
)

var apiBase = "https://ns-api.nakedsword.com"

var (
	matchRe = regexp.MustCompile(`^https?://(?:www\.)?nakedsword\.com(?:/|$)`)
	// /studios/{id}/{slug} — a sub-studio such as "NakedSword x Rhyheim".
	studioURLRe = regexp.MustCompile(`/studios/(\d+)(?:/|$)`)
	// Each non-alphanumeric character maps to its own dash — runs are NOT
	// collapsed. This mirrors the site's own slugifier
	// (`replace(/[^A-Za-z0-9]/g, "-")`), so "Rio: Beau's" becomes
	// "rio--beau-s" and the generated URL is already canonical rather than
	// bouncing through a redirect.
	nonAlnumRe = regexp.MustCompile(`[^A-Za-z0-9]`)
)

type Scraper struct {
	Client *http.Client
}

var _ scraper.StudioScraper = (*Scraper)(nil)

func New() *Scraper {
	return &Scraper{Client: httpx.NewClient(30 * time.Second)}
}

func init() { scraper.Register(New()) }

func (s *Scraper) ID() string { return siteID }

func (s *Scraper) Patterns() []string {
	return []string{
		"nakedsword.com",
		"nakedsword.com/studios/{id}/{slug}",
	}
}

func (s *Scraper) MatchesURL(u string) bool { return matchRe.MatchString(u) }

func (s *Scraper) ListScenes(ctx context.Context, studioURL string, opts scraper.ListOpts) (<-chan scraper.SceneResult, error) {
	out := make(chan scraper.SceneResult)
	go s.run(ctx, studioURL, opts, out)
	return out, nil
}

// xIdent reproduces the whitelabel header the SPA sends on every API call.
//
// The bundle's builder is:
//
//	salt = random 256 bytes; iv = random 16 bytes
//	key  = PBKDF2-SHA512(passphrase, salt, 999 iterations, 32 bytes)
//	body = AES-256-CBC(key, iv, `{"date":<unix ms>,"propertyId":"1"}`)
//	hdr  = base64(JSON{ciphertext: base64(body), salt: hex, iv: hex})
//
// The browser caches the header for 30s; we build a fresh one per request,
// which costs one PBKDF2 derivation and avoids any staleness bookkeeping.
func xIdent(now time.Time) (string, error) {
	payload, err := json.Marshal(struct {
		Date       int64  `json:"date"`
		PropertyID string `json:"propertyId"`
	}{now.UnixMilli(), propertyID})
	if err != nil {
		return "", err
	}

	salt := make([]byte, 256)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", fmt.Errorf("generating iv: %w", err)
	}

	key, err := pbkdf2.Key(sha512.New, passphrase, salt, 999, 32)
	if err != nil {
		return "", fmt.Errorf("deriving key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	// CryptoJS pads with PKCS#7.
	pad := aes.BlockSize - len(payload)%aes.BlockSize
	plain := append(append([]byte{}, payload...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	ct := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, plain)

	envelope, err := json.Marshal(struct {
		Ciphertext string `json:"ciphertext"`
		Salt       string `json:"salt"`
		IV         string `json:"iv"`
	}{
		base64.StdEncoding.EncodeToString(ct),
		hex.EncodeToString(salt),
		hex.EncodeToString(iv),
	})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(envelope), nil
}

// ---- API types ----

type apiStar struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type apiImage struct {
	URL string `json:"url"`
}

type apiMovie struct {
	MovieID     int    `json:"movieId"`
	Title       string `json:"title"`
	TitleNs     string `json:"titleNs"`
	Description string `json:"description"`
	// DescriptionNs is the NakedSword-specific copy, preferred when present.
	DescriptionNs string `json:"descriptionNs"`
}

type apiScene struct {
	ID               int        `json:"id"`
	Index            int        `json:"index"`
	StartTimeSeconds int        `json:"startTimeSeconds"`
	EndTimeSeconds   int        `json:"endTimeSeconds"`
	SampleVideo      string     `json:"sample_video"`
	PublishStart     string     `json:"publish_start"`
	CoverImages      []apiImage `json:"cover_images"`
	Stars            []apiStar  `json:"stars"`
	Movie            *apiMovie  `json:"movie"`
}

type apiPagination struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	Total       int `json:"total"`
}

type feedResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Scenes     []apiScene    `json:"scenes"`
		Pagination apiPagination `json:"pagination"`
	} `json:"data"`
}

type studioDetailsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
}

// ---- scraping ----

func (s *Scraper) run(ctx context.Context, studioURL string, opts scraper.ListOpts, out chan<- scraper.SceneResult) {
	defer close(out)

	studioID := parseStudioID(studioURL)
	name := studioName
	if studioID != "" {
		scraper.Debugf(1, "%s: scraping studio %s", siteID, studioID)
		if n, err := s.fetchStudioName(ctx, studioID); err != nil {
			// Not fatal: the scenes are still scrapeable, they just carry the
			// parent studio name.
			scraper.Debugf(1, "%s: studio %s details failed: %v", siteID, studioID, err)
		} else if n != "" {
			name = n
		}
	}

	now := time.Now().UTC()
	scraper.Paginate(ctx, opts, siteID, out, func(ctx context.Context, page int) (scraper.PageResult, error) {
		resp, err := s.fetchFeed(ctx, studioID, page)
		if err != nil {
			return scraper.PageResult{}, err
		}

		scenes := make([]models.Scene, 0, len(resp.Data.Scenes))
		for _, sc := range resp.Data.Scenes {
			scenes = append(scenes, toScene(sc, studioURL, name, now))
		}

		res := scraper.PageResult{Scenes: scenes}
		if page == 1 {
			res.Total = resp.Data.Pagination.Total
		}
		if last := resp.Data.Pagination.LastPage; last > 0 && page >= last {
			res.Done = true
		}
		return res, nil
	})
}

// parseStudioID returns the numeric studio id from a /studios/{id}/{slug} URL,
// or "" for the main catalogue.
func parseStudioID(studioURL string) string {
	if m := studioURLRe.FindStringSubmatch(studioURL); m != nil {
		return m[1]
	}
	return ""
}

func (s *Scraper) fetchStudioName(ctx context.Context, studioID string) (string, error) {
	var out studioDetailsResponse
	if err := s.getJSON(ctx, apiBase+"/frontend/studios/"+studioID+"/details", &out); err != nil {
		return "", err
	}
	return out.Data.Name, nil
}

func (s *Scraper) fetchFeed(ctx context.Context, studioID string, page int) (*feedResponse, error) {
	q := url.Values{
		// Newest-first is what makes the KnownIDs early-stop in Paginate valid.
		"sort":     {"newest"},
		"per_page": {strconv.Itoa(perPage)},
		"page":     {strconv.Itoa(page)},
	}
	if studioID != "" {
		q.Set("studios_id", studioID)
	}

	var out feedResponse
	if err := s.getJSON(ctx, apiBase+"/frontend/scenes/feed?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("api reported failure: %s", out.Message)
	}
	return &out, nil
}

func (s *Scraper) getJSON(ctx context.Context, rawURL string, v any) error {
	ident, err := xIdent(time.Now())
	if err != nil {
		return err
	}

	resp, err := httpx.Do(ctx, s.Client, httpx.Request{
		URL: rawURL,
		Headers: map[string]string{
			"User-Agent": httpx.UserAgentFirefox,
			"Accept":     "application/json",
			"x-ident":    ident,
			"Origin":     siteBase,
			"Referer":    siteBase + "/",
		},
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return httpx.DecodeJSON(resp.Body, v)
}

func toScene(sc apiScene, studioURL, studio string, now time.Time) models.Scene {
	scene := models.Scene{
		ID:        strconv.Itoa(sc.ID),
		SiteID:    siteID,
		StudioURL: studioURL,
		Studio:    studio,
		Preview:   sc.SampleVideo,
		ScrapedAt: now,
	}

	// A "scene" here is a segment of a movie, so its title, description and
	// canonical URL all derive from the parent movie. The *Ns fields are the
	// NakedSword-specific copy and win when present.
	if m := sc.Movie; m != nil {
		scene.Title = firstNonEmpty(m.TitleNs, m.Title)
		scene.Description = firstNonEmpty(m.DescriptionNs, m.Description)
		scene.URL = sceneURL(m.MovieID, scene.Title, sc.Index)
	}

	if d := sc.EndTimeSeconds - sc.StartTimeSeconds; d > 0 {
		scene.Duration = d
	}
	if t, err := time.Parse(time.RFC3339, sc.PublishStart); err == nil {
		scene.Date = t.UTC()
	}
	if len(sc.CoverImages) > 0 {
		scene.Thumbnail = sc.CoverImages[0].URL
	}
	for _, st := range sc.Stars {
		if n := strings.TrimSpace(st.Name); n != "" {
			scene.Performers = append(scene.Performers, n)
		}
	}
	return scene
}

// sceneURL builds the canonical scene page URL:
// /movies/{movieId}/{slug}/scene/{index}. The server redirects any slug to the
// canonical lowercase one, so the slug is cosmetic.
func sceneURL(movieID int, title string, index int) string {
	if movieID == 0 {
		return ""
	}
	slug := nonAlnumRe.ReplaceAllString(strings.ToLower(title), "-")
	if slug == "" {
		slug = "movie"
	}
	return fmt.Sprintf("%s/movies/%d/%s/scene/%d", siteBase, movieID, slug, index)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
