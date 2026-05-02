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

// baseSchema creates the core tables present since v0. The performers/tags/categories
// TEXT columns are kept for backwards compatibility but are no longer read or written.
const baseSchema = `
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL
);

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

// migration1 adds normalised junction tables for performers, tags, and categories.
const migration1 = `
CREATE TABLE IF NOT EXISTS performers (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS tags (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS categories (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS scene_performers (
    scene_id     TEXT NOT NULL,
    site_id      TEXT NOT NULL,
    performer_id INTEGER NOT NULL,
    position     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (scene_id, site_id, performer_id),
    FOREIGN KEY (scene_id, site_id) REFERENCES scenes(id, site_id) ON DELETE CASCADE,
    FOREIGN KEY (performer_id) REFERENCES performers(id)
);

CREATE TABLE IF NOT EXISTS scene_tags (
    scene_id TEXT NOT NULL,
    site_id  TEXT NOT NULL,
    tag_id   INTEGER NOT NULL,
    PRIMARY KEY (scene_id, site_id, tag_id),
    FOREIGN KEY (scene_id, site_id) REFERENCES scenes(id, site_id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id)
);

CREATE TABLE IF NOT EXISTS scene_categories (
    scene_id    TEXT NOT NULL,
    site_id     TEXT NOT NULL,
    category_id INTEGER NOT NULL,
    PRIMARY KEY (scene_id, site_id, category_id),
    FOREIGN KEY (scene_id, site_id) REFERENCES scenes(id, site_id) ON DELETE CASCADE,
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

CREATE INDEX IF NOT EXISTS idx_scene_performers_performer ON scene_performers(performer_id);
CREATE INDEX IF NOT EXISTS idx_scene_tags_tag ON scene_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_scene_categories_category ON scene_categories(category_id);
`

func (s *SQLite) migrate() error {
	if _, err := s.db.Exec(baseSchema); err != nil {
		return err
	}

	var version int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}

	if version < 1 {
		if err := s.applyMigration1(); err != nil {
			return fmt.Errorf("migration 1: %w", err)
		}
	}
	return nil
}

// applyMigration1 creates the junction tables and migrates existing JSON data.
func (s *SQLite) applyMigration1() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(migration1); err != nil {
		return err
	}

	// Migrate existing JSON data from the scenes table into junction tables.
	rows, err := tx.Query(`SELECT id, site_id, performers, tags, categories FROM scenes`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	type sceneStrings struct {
		id, siteID                   string
		performers, tags, categories string
	}
	var all []sceneStrings
	for rows.Next() {
		var r sceneStrings
		if err := rows.Scan(&r.id, &r.siteID, &r.performers, &r.tags, &r.categories); err != nil {
			return err
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range all {
		performers, err := unmarshalStrings(r.performers)
		if err != nil {
			return fmt.Errorf("parsing performers for scene %s: %w", r.id, err)
		}
		if err := syncRelation(tx, "performers", "scene_performers", "performer_id", r.id, r.siteID, performers, true); err != nil {
			return err
		}

		tags, err := unmarshalStrings(r.tags)
		if err != nil {
			return fmt.Errorf("parsing tags for scene %s: %w", r.id, err)
		}
		if err := syncRelation(tx, "tags", "scene_tags", "tag_id", r.id, r.siteID, tags, false); err != nil {
			return err
		}

		cats, err := unmarshalStrings(r.categories)
		if err != nil {
			return fmt.Errorf("parsing categories for scene %s: %w", r.id, err)
		}
		if err := syncRelation(tx, "categories", "scene_categories", "category_id", r.id, r.siteID, cats, false); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM schema_version`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_version (version) VALUES (1)`); err != nil {
		return err
	}
	return tx.Commit()
}

// ---- Store interface ----

func (s *SQLite) Load(studioURL string) ([]models.Scene, error) {
	rows, err := s.db.Query(`
		SELECT id, site_id, studio_url, title, url, date, description,
		       thumbnail, preview, director, studio,
		       series, series_part,
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

	if err := s.loadRelation(studioURL, "scene_performers", "performers", "performer_id", "position", scenes); err != nil {
		return nil, err
	}
	if err := s.loadRelation(studioURL, "scene_tags", "tags", "tag_id", "", scenes); err != nil {
		return nil, err
	}
	if err := s.loadRelation(studioURL, "scene_categories", "categories", "category_id", "", scenes); err != nil {
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

// ---- upsert helpers ----

func upsertScene(tx *sql.Tx, sc models.Scene) error {
	_, err := tx.Exec(`
		INSERT INTO scenes (
		    id, site_id, studio_url, title, url, date, description,
		    thumbnail, preview, performers, director, studio,
		    tags, categories, series, series_part,
		    duration, resolution, width, height, format,
		    views, likes, comments,
		    lowest_price, lowest_price_date, scraped_at, deleted_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id, site_id) DO UPDATE SET
		    studio_url        = excluded.studio_url,
		    title             = excluded.title,
		    url               = excluded.url,
		    date              = excluded.date,
		    description       = excluded.description,
		    thumbnail         = excluded.thumbnail,
		    preview           = excluded.preview,
		    performers        = excluded.performers,
		    director          = excluded.director,
		    studio            = excluded.studio,
		    tags              = excluded.tags,
		    categories        = excluded.categories,
		    series            = excluded.series,
		    series_part       = excluded.series_part,
		    duration          = excluded.duration,
		    resolution        = excluded.resolution,
		    width             = excluded.width,
		    height            = excluded.height,
		    format            = excluded.format,
		    views             = excluded.views,
		    likes             = excluded.likes,
		    comments          = excluded.comments,
		    lowest_price      = excluded.lowest_price,
		    lowest_price_date = excluded.lowest_price_date,
		    scraped_at        = excluded.scraped_at,
		    deleted_at        = excluded.deleted_at`,
		sc.ID, sc.SiteID, sc.StudioURL, sc.Title, sc.URL,
		timeStr(sc.Date), sc.Description, sc.Thumbnail, sc.Preview,
		"[]", sc.Director, sc.Studio,
		"[]", "[]",
		sc.Series, sc.SeriesPart,
		sc.Duration, sc.Resolution, sc.Width, sc.Height, sc.Format,
		sc.Views, sc.Likes, sc.Comments,
		sc.LowestPrice, timePtrStr(sc.LowestPriceDate),
		timeStr(sc.ScrapedAt), timePtrStr(sc.DeletedAt),
	)
	if err != nil {
		return fmt.Errorf("upserting scene %s: %w", sc.ID, err)
	}

	if err := syncRelation(tx, "performers", "scene_performers", "performer_id", sc.ID, sc.SiteID, sc.Performers, true); err != nil {
		return fmt.Errorf("upserting performers for %s: %w", sc.ID, err)
	}
	if err := syncRelation(tx, "tags", "scene_tags", "tag_id", sc.ID, sc.SiteID, sc.Tags, false); err != nil {
		return fmt.Errorf("upserting tags for %s: %w", sc.ID, err)
	}
	if err := syncRelation(tx, "categories", "scene_categories", "category_id", sc.ID, sc.SiteID, sc.Categories, false); err != nil {
		return fmt.Errorf("upserting categories for %s: %w", sc.ID, err)
	}

	return syncPriceHistory(tx, sc)
}

// syncRelation diffs the desired (name, position) set against the junction rows
// already stored for this scene and applies only the deltas. Entity upserts use
// INSERT ... ON CONFLICT ... RETURNING id to fetch the id in a single round
// trip whether the entity is new or already known.
func syncRelation(tx *sql.Tx, entityTable, junctionTable, fkCol, sceneID, siteID string, names []string, withPosition bool) error {
	type want struct {
		name     string
		position int
	}
	seen := make(map[string]struct{}, len(names))
	desired := make([]want, 0, len(names))
	for i, name := range names {
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		desired = append(desired, want{name: name, position: i})
	}
	desiredSet := make(map[string]int, len(desired))
	for _, w := range desired {
		desiredSet[w.name] = w.position
	}

	type linked struct {
		id       int64
		position int
	}
	posCol := "0"
	if withPosition {
		posCol = "j.position"
	}
	rows, err := tx.Query(
		`SELECT e.name, j.`+fkCol+`, `+posCol+`
		 FROM `+junctionTable+` j
		 JOIN `+entityTable+` e ON j.`+fkCol+` = e.id
		 WHERE j.scene_id = ? AND j.site_id = ?`,
		sceneID, siteID,
	)
	if err != nil {
		return err
	}
	existing := map[string]linked{}
	for rows.Next() {
		var name string
		var l linked
		if err := rows.Scan(&name, &l.id, &l.position); err != nil {
			_ = rows.Close()
			return err
		}
		existing[name] = l
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Drop junction rows whose entity is no longer in the desired set.
	for name, l := range existing {
		if _, keep := desiredSet[name]; keep {
			continue
		}
		if _, err := tx.Exec(
			`DELETE FROM `+junctionTable+
				` WHERE scene_id = ? AND site_id = ? AND `+fkCol+` = ?`,
			sceneID, siteID, l.id,
		); err != nil {
			return err
		}
		delete(existing, name)
	}

	// Walk desired in input order so newly inserted rowids reflect that order
	// (loadRelation has no ORDER BY for tags/categories and relies on this).
	for _, w := range desired {
		if l, ok := existing[w.name]; ok {
			if withPosition && l.position != w.position {
				if _, err := tx.Exec(
					`UPDATE `+junctionTable+
						` SET position = ? WHERE scene_id = ? AND site_id = ? AND `+fkCol+` = ?`,
					w.position, sceneID, siteID, l.id,
				); err != nil {
					return err
				}
			}
			continue
		}

		var id int64
		if err := tx.QueryRow(
			`INSERT INTO `+entityTable+` (name) VALUES (?)
			 ON CONFLICT(name) DO UPDATE SET name = excluded.name
			 RETURNING id`,
			w.name,
		).Scan(&id); err != nil {
			return err
		}
		if withPosition {
			_, err = tx.Exec(
				`INSERT INTO `+junctionTable+` (scene_id, site_id, `+fkCol+`, position) VALUES (?,?,?,?)`,
				sceneID, siteID, id, w.position,
			)
		} else {
			_, err = tx.Exec(
				`INSERT INTO `+junctionTable+` (scene_id, site_id, `+fkCol+`) VALUES (?,?,?)`,
				sceneID, siteID, id,
			)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// syncPriceHistory diffs the desired snapshot list against price_history rows
// already stored for this scene, deleting and inserting only the differences.
// Identity is the full snapshot tuple (date + price fields), so re-saves with
// unchanged history are a single SELECT, and adding one snapshot is a single
// INSERT — instead of the old DELETE-all + reinsert-all pattern.
func syncPriceHistory(tx *sql.Tx, sc models.Scene) error {
	type key struct {
		date            string
		regular         float64
		discounted      float64
		isFree          int
		isOnSale        int
		discountPercent int
	}
	keyOf := func(p models.PriceSnapshot) key {
		return key{
			date:            timeStr(p.Date),
			regular:         p.Regular,
			discounted:      p.Discounted,
			isFree:          boolInt(p.IsFree),
			isOnSale:        boolInt(p.IsOnSale),
			discountPercent: p.DiscountPercent,
		}
	}

	desired := make(map[key]int, len(sc.PriceHistory))
	for _, p := range sc.PriceHistory {
		desired[keyOf(p)]++
	}

	rows, err := tx.Query(`
		SELECT id, date, regular, discounted, is_free, is_on_sale, discount_percent
		FROM price_history WHERE scene_id = ? AND site_id = ?`,
		sc.ID, sc.SiteID,
	)
	if err != nil {
		return err
	}
	type existingRow struct {
		id  int64
		key key
	}
	var existing []existingRow
	for rows.Next() {
		var r existingRow
		if err := rows.Scan(
			&r.id, &r.key.date, &r.key.regular, &r.key.discounted,
			&r.key.isFree, &r.key.isOnSale, &r.key.discountPercent,
		); err != nil {
			_ = rows.Close()
			return err
		}
		existing = append(existing, r)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// For each existing row: if the desired set still wants one, consume it;
	// otherwise the row is no longer needed and is deleted.
	for _, r := range existing {
		if desired[r.key] > 0 {
			desired[r.key]--
			continue
		}
		if _, err := tx.Exec(
			`DELETE FROM price_history WHERE id = ?`, r.id,
		); err != nil {
			return err
		}
	}

	// Anything still owed in `desired` represents new snapshots to insert.
	for _, p := range sc.PriceHistory {
		k := keyOf(p)
		if desired[k] <= 0 {
			continue
		}
		desired[k]--
		if _, err := tx.Exec(`
			INSERT INTO price_history (scene_id, site_id, date, regular, discounted, is_free, is_on_sale, discount_percent)
			VALUES (?,?,?,?,?,?,?,?)`,
			sc.ID, sc.SiteID, k.date,
			k.regular, k.discounted, k.isFree, k.isOnSale, k.discountPercent,
		); err != nil {
			return fmt.Errorf("inserting price history for %s: %w", sc.ID, err)
		}
	}

	return nil
}

// ---- load helpers ----

// loadRelation batch-loads a string relation (performers, tags, or categories)
// for all scenes belonging to studioURL and attaches results to the scene slice.
// orderCol is the column to ORDER BY (empty = no ordering).
func (s *SQLite) loadRelation(studioURL, junctionTable, entityTable, fkCol, orderCol string, scenes []models.Scene) error {
	if len(scenes) == 0 {
		return nil
	}
	idx := make(map[string]int, len(scenes))
	for i, sc := range scenes {
		idx[sc.SiteID+":"+sc.ID] = i
	}

	q := `SELECT j.scene_id, j.site_id, e.name
		FROM ` + junctionTable + ` j
		JOIN ` + entityTable + ` e ON j.` + fkCol + ` = e.id
		JOIN scenes s ON j.scene_id = s.id AND j.site_id = s.site_id
		WHERE s.studio_url = ?`
	if orderCol != "" {
		q += ` ORDER BY j.scene_id, j.site_id, j.` + orderCol
	}

	rows, err := s.db.Query(q, studioURL)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var sceneID, siteID, name string
		if err := rows.Scan(&sceneID, &siteID, &name); err != nil {
			return err
		}
		i, ok := idx[siteID+":"+sceneID]
		if !ok {
			continue
		}
		switch junctionTable {
		case "scene_performers":
			scenes[i].Performers = append(scenes[i].Performers, name)
		case "scene_tags":
			scenes[i].Tags = append(scenes[i].Tags, name)
		case "scene_categories":
			scenes[i].Categories = append(scenes[i].Categories, name)
		}
	}
	return rows.Err()
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
		var parseErr error
		p.Date, parseErr = parseStr(dateStr)
		if parseErr != nil {
			return fmt.Errorf("parsing price_history date for %s: %w", sceneID, parseErr)
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
		lowestPriceDate sql.NullString
		scrapedAt       string
		deletedAt       sql.NullString
	)
	err := rows.Scan(
		&sc.ID, &sc.SiteID, &sc.StudioURL, &sc.Title, &sc.URL,
		&dateStr, &sc.Description, &sc.Thumbnail, &sc.Preview,
		&sc.Director, &sc.Studio,
		&sc.Series, &sc.SeriesPart,
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

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// unmarshalStrings decodes a JSON array of strings, returning nil for empty/null arrays.
func unmarshalStrings(s string) ([]string, error) {
	if s == "" || s == "[]" || s == "null" {
		return nil, nil
	}
	var result []string
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil, err
	}
	return result, nil
}
