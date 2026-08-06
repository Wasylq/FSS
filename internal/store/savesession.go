package store

import (
	"database/sql"
	"fmt"
)

// saveSession carries the per-transaction caches that make a large Save
// affordable. Both exist because Save's inner loop runs once per scene and, for
// a first ingest, there can be tens of thousands of them.
//
// Profiling a 59k-scene ingest before these existed:
//
//   - 32% of CPU in sqlite3_prepare_v2 — every Exec re-parsed its SQL, because
//     database/sql prepares, executes and discards on each call;
//   - 48% cumulative in Tx.QueryRow — almost all of it syncRelation resolving
//     performer/tag/category names to IDs, one round trip per name per scene
//     (~900k calls) for names that are globally unique and mostly already
//     stored.
//
// Statements are bound to the transaction and must be closed with it, which is
// what close does.
type saveSession struct {
	tx *sql.Tx

	// prepared caches by SQL text. Every query in the write path is either a
	// constant or built from a validated identifier set, so the number of
	// distinct strings is bounded and small.
	prepared map[string]*sql.Stmt

	// entityIDs caches `entityTable -> name -> id`. Names are globally unique
	// per table, so a resolution is valid for the whole transaction.
	entityIDs map[string]map[string]int64
}

func newSaveSession(tx *sql.Tx) *saveSession {
	return &saveSession{
		tx:        tx,
		prepared:  map[string]*sql.Stmt{},
		entityIDs: map[string]map[string]int64{},
	}
}

// close releases every prepared statement. Safe to call after a rollback.
func (s *saveSession) close() {
	for _, st := range s.prepared {
		_ = st.Close()
	}
	s.prepared = nil
}

func (s *saveSession) stmt(query string) (*sql.Stmt, error) {
	if st, ok := s.prepared[query]; ok {
		return st, nil
	}
	st, err := s.tx.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("preparing statement: %w", err)
	}
	s.prepared[query] = st
	return st, nil
}

func (s *saveSession) exec(query string, args ...any) (sql.Result, error) {
	st, err := s.stmt(query)
	if err != nil {
		return nil, err
	}
	return st.Exec(args...)
}

func (s *saveSession) query(query string, args ...any) (*sql.Rows, error) {
	st, err := s.stmt(query)
	if err != nil {
		return nil, err
	}
	return st.Query(args...)
}

// entityID resolves a name in performers/tags/categories to its row id,
// inserting it if new. Repeat lookups within one Save are served from memory.
func (s *saveSession) entityID(entityTable, name string) (int64, error) {
	byName, ok := s.entityIDs[entityTable]
	if !ok {
		byName = map[string]int64{}
		s.entityIDs[entityTable] = byName
	}
	if id, ok := byName[name]; ok {
		return id, nil
	}

	st, err := s.stmt(`INSERT INTO ` + entityTable + ` (name) VALUES (?)
		 ON CONFLICT(name) DO UPDATE SET name = excluded.name
		 RETURNING id`)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := st.QueryRow(name).Scan(&id); err != nil {
		return 0, err
	}
	byName[name] = id
	return id, nil
}
