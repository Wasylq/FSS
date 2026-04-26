package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Wasylq/FSS/models"
	_ "modernc.org/sqlite"
)

// SQLite is the optional store backed by a SQLite database.
// Enabled with the --db flag. JSON/CSV are exported from it on request.
type SQLite struct {
	db *sql.DB
}

func NewSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1) // SQLite does not support concurrent writes
	s := &SQLite{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}
	return s, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

// ---- schema ----

const schema = `
CREATE TABLE IF NOT EXISTS scenes (
    id                TEXT NOT NULL,
    site_id           TEXT NOT NULL,
    studio_url        TEXT NOT NULL,
    title             TEXT NOT NULL DEFAULT '',
    url               TEXT NOT NULL DEFAULT '',
    date              TEXT,
    description       TEXT DEFAULT '',
    thumbnail         TEXT DEFAULT '',
    preview           TEXT DEFAULT '',
    performers        TEXT DEFAULT '[]',
    director          TEXT DEFAULT '',
    studio            TEXT DEFAULT '',
    tags              TEXT DEFAULT '[]',
    categories        TEXT DEFAULT '[]',
    series            TEXT DEFAULT '',
    series_part       INTEGER DEFAULT 0,
    duration          INTEGER DEFAULT 0,
    resolution        TEXT DEFAULT '',
    width             INTEGER DEFAULT 0,
    height            INTEGER DEFAULT 0,
    format            TEXT DEFAULT '',
    views             INTEGER DEFAULT 0,
    likes             INTEGER DEFAULT 0,
    comments          INTEGER DEFAULT 0,
    lowest_price      REAL DEFAULT 0,
    lowest_price_date TEXT,
    scraped_at        TEXT NOT NULL,
    deleted_at        TEXT,
    PRIMARY KEY (id, site_id)
);

CREATE TABLE IF NOT EXISTS price_history (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    scene_id         TEXT NOT NULL,
    site_id          TEXT NOT NULL,
    date             TEXT NOT NULL,
    regular          REAL NOT NULL DEFAULT 0,
    discounted       REAL DEFAULT 0,
    is_free          INTEGER NOT NULL DEFAULT 0,
    is_on_sale       INTEGER NOT NULL DEFAULT 0,
    discount_percent INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS studios (
    url              TEXT PRIMARY KEY,
    site_id          TEXT NOT NULL,
    name             TEXT DEFAULT '',
    added_at         TEXT NOT NULL,
    last_scraped_at  TEXT
);

CREATE INDEX IF NOT EXISTS idx_scenes_studio_url ON scenes(studio_url);
CREATE INDEX IF NOT EXISTS idx_price_history_scene ON price_history(scene_id, site_id);
`

func (s *SQLite) migrate() error {
	_, err := s.db.Exec(schema)
	return err
}

// ---- Store interface ----

func (s *SQLite) Load(studioURL string) ([]models.Scene, error) {
	rows, err := s.db.Query(`
		SELECT id, site_id, studio_url, title, url, date, description,
		       thumbnail, preview, performers, director, studio,
		       tags, categories, series, series_part,
		       duration, resolution, width, height, format,
		       views, likes, comments,
		       lowest_price, lowest_price_date, scraped_at, deleted_at
		FROM scenes WHERE studio_url = ?`, studioURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var scenes []models.Scene
	for rows.Next() {
		sc, err := scanScene(rows)
		if err != nil {
			return nil, err
		}
		scenes = append(scenes, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := s.loadPriceHistory(studioURL, scenes); err != nil {
		return nil, err
	}
	return scenes, nil
}

func (s *SQLite) Save(studioURL string, scenes []models.Scene) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, sc := range scenes {
		if err := upsertScene(tx, sc); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) MarkDeleted(studioURL string, ids []string) error {
	now := timeStr(time.Now().UTC())
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, id := range ids {
		if _, err := tx.Exec(
			`UPDATE scenes SET deleted_at = ? WHERE id = ? AND studio_url = ? AND deleted_at IS NULL`,
			now, id, studioURL,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLite) UpsertStudio(studio models.Studio) error {
	_, err := s.db.Exec(`
		INSERT INTO studios (url, site_id, name, added_at, last_scraped_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
		    name            = CASE WHEN excluded.name != '' THEN excluded.name ELSE studios.name END,
		    last_scraped_at = excluded.last_scraped_at`,
		studio.URL, studio.SiteID, studio.Name,
		timeStr(studio.AddedAt), timePtrStr(studio.LastScrapedAt),
	)
	return err
}

func (s *SQLite) ListStudios() ([]models.Studio, error) {
	rows, err := s.db.Query(`
		SELECT url, site_id, name, added_at, last_scraped_at
		FROM studios ORDER BY added_at ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var studios []models.Studio
	for rows.Next() {
		var st models.Studio
		var addedAt string
		var lastScrapedAt sql.NullString
		if err := rows.Scan(&st.URL, &st.SiteID, &st.Name, &addedAt, &lastScrapedAt); err != nil {
			return nil, err
		}
		if st.AddedAt, err = parseStr(addedAt); err != nil {
			return nil, fmt.Errorf("parsing added_at for %s: %w", st.URL, err)
		}
		if st.LastScrapedAt, err = parseStrPtr(lastScrapedAt); err != nil {
			return nil, fmt.Errorf("parsing last_scraped_at for %s: %w", st.URL, err)
		}
		studios = append(studios, st)
	}
	return studios, rows.Err()
}

func (s *SQLite) Export(format, path, studioURL string) error {
	scenes, err := s.Load(studioURL)
	if err != nil {
		return err
	}
	switch format {
	case "json":
		sf := studioFile{
			StudioURL:  studioURL,
			ScrapedAt:  time.Now().UTC(),
			SceneCount: len(scenes),
			Scenes:     scenes,
		}
		return WriteJSON(sf, path)
	case "csv":
		return WriteCSV(scenes, path)
	default:
		return fmt.Errorf("unknown export format %q", format)
	}
}

// ---- helpers ----

func upsertScene(tx *sql.Tx, sc models.Scene) error {
	performers, err := jsonStr(sc.Performers)
	if err != nil {
		return fmt.Errorf("encoding performers for %s: %w", sc.ID, err)
	}
	tags, err := jsonStr(sc.Tags)
	if err != nil {
		return fmt.Errorf("encoding tags for %s: %w", sc.ID, err)
	}
	categories, err := jsonStr(sc.Categories)
	if err != nil {
		return fmt.Errorf("encoding categories for %s: %w", sc.ID, err)
	}
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO scenes (
		    id, site_id, studio_url, title, url, date, description,
		    thumbnail, preview, performers, director, studio,
		    tags, categories, series, series_part,
		    duration, resolution, width, height, format,
		    views, likes, comments,
		    lowest_price, lowest_price_date, scraped_at, deleted_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sc.ID, sc.SiteID, sc.StudioURL, sc.Title, sc.URL,
		timeStr(sc.Date), sc.Description, sc.Thumbnail, sc.Preview,
		performers, sc.Director, sc.Studio,
		tags, categories,
		sc.Series, sc.SeriesPart,
		sc.Duration, sc.Resolution, sc.Width, sc.Height, sc.Format,
		sc.Views, sc.Likes, sc.Comments,
		sc.LowestPrice, timePtrStr(sc.LowestPriceDate),
		timeStr(sc.ScrapedAt), timePtrStr(sc.DeletedAt),
	)
	if err != nil {
		return fmt.Errorf("upserting scene %s: %w", sc.ID, err)
	}

	if _, err := tx.Exec(`DELETE FROM price_history WHERE scene_id = ? AND site_id = ?`,
		sc.ID, sc.SiteID); err != nil {
		return err
	}
	for _, p := range sc.PriceHistory {
		if _, err := tx.Exec(`
			INSERT INTO price_history (scene_id, site_id, date, regular, discounted, is_free, is_on_sale, discount_percent)
			VALUES (?,?,?,?,?,?,?,?)`,
			sc.ID, sc.SiteID, timeStr(p.Date),
			p.Regular, p.Discounted, boolInt(p.IsFree), boolInt(p.IsOnSale), p.DiscountPercent,
		); err != nil {
			return fmt.Errorf("inserting price history for %s: %w", sc.ID, err)
		}
	}
	return nil
}

func (s *SQLite) loadPriceHistory(studioURL string, scenes []models.Scene) error {
	if len(scenes) == 0 {
		return nil
	}
	idx := make(map[string]int, len(scenes))
	for i, sc := range scenes {
		idx[sc.SiteID+":"+sc.ID] = i
	}

	rows, err := s.db.Query(`
		SELECT ph.scene_id, ph.site_id, ph.date, ph.regular, ph.discounted,
		       ph.is_free, ph.is_on_sale, ph.discount_percent
		FROM price_history ph
		JOIN scenes sc ON ph.scene_id = sc.id AND ph.site_id = sc.site_id
		WHERE sc.studio_url = ?
		ORDER BY ph.date ASC`, studioURL)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var sceneID, siteID, dateStr string
		var p models.PriceSnapshot
		var isFree, isOnSale int
		if err := rows.Scan(&sceneID, &siteID, &dateStr,
			&p.Regular, &p.Discounted, &isFree, &isOnSale, &p.DiscountPercent); err != nil {
			return err
		}
		p.Date, err = parseStr(dateStr)
		if err != nil {
			return fmt.Errorf("parsing price_history date for %s: %w", sceneID, err)
		}
		p.IsFree = isFree != 0
		p.IsOnSale = isOnSale != 0
		if i, ok := idx[siteID+":"+sceneID]; ok {
			scenes[i].PriceHistory = append(scenes[i].PriceHistory, p)
		}
	}
	return rows.Err()
}

func scanScene(rows *sql.Rows) (models.Scene, error) {
	var sc models.Scene
	var (
		dateStr         string
		performers      string
		tags            string
		categories      string
		lowestPriceDate sql.NullString
		scrapedAt       string
		deletedAt       sql.NullString
	)
	err := rows.Scan(
		&sc.ID, &sc.SiteID, &sc.StudioURL, &sc.Title, &sc.URL,
		&dateStr, &sc.Description, &sc.Thumbnail, &sc.Preview,
		&performers, &sc.Director, &sc.Studio,
		&tags, &categories, &sc.Series, &sc.SeriesPart,
		&sc.Duration, &sc.Resolution, &sc.Width, &sc.Height, &sc.Format,
		&sc.Views, &sc.Likes, &sc.Comments,
		&sc.LowestPrice, &lowestPriceDate, &scrapedAt, &deletedAt,
	)
	if err != nil {
		return sc, err
	}
	if sc.Date, err = parseStr(dateStr); err != nil {
		return sc, fmt.Errorf("parsing date for %s: %w", sc.ID, err)
	}
	if sc.ScrapedAt, err = parseStr(scrapedAt); err != nil {
		return sc, fmt.Errorf("parsing scraped_at for %s: %w", sc.ID, err)
	}
	if sc.LowestPriceDate, err = parseStrPtr(lowestPriceDate); err != nil {
		return sc, fmt.Errorf("parsing lowest_price_date for %s: %w", sc.ID, err)
	}
	if sc.DeletedAt, err = parseStrPtr(deletedAt); err != nil {
		return sc, fmt.Errorf("parsing deleted_at for %s: %w", sc.ID, err)
	}
	if err := json.Unmarshal([]byte(performers), &sc.Performers); err != nil {
		return sc, fmt.Errorf("unmarshalling performers: %w", err)
	}
	if err := json.Unmarshal([]byte(tags), &sc.Tags); err != nil {
		return sc, fmt.Errorf("unmarshalling tags: %w", err)
	}
	if err := json.Unmarshal([]byte(categories), &sc.Categories); err != nil {
		return sc, fmt.Errorf("unmarshalling categories: %w", err)
	}
	return sc, nil
}

// ---- time / encoding helpers ----

func timeStr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func timePtrStr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func parseStr(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}

func parseStrPtr(s sql.NullString) (*time.Time, error) {
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func jsonStr(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
