// Package store provides SQLite-backed persistence for the SAR sidelobe
// diagnosis service using the pure-Go modernc.org/sqlite driver so builds
// stay CGO-free and offline-capable.
package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the underlying *sql.DB and exposes CRUD for every entity.
type Store struct {
	db      *sql.DB
	writeMu sync.Mutex
}

// Open opens (or creates) the SQLite database at dbPath and applies the
// schema migration. It is safe to call on an existing database; reopening
// the same path restores all persisted entities.
func Open(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying handle for advanced callers (tests / diagnostics).
func (s *Store) DB() *sql.DB { return s.db }

// Tx runs fn inside a transaction; the transaction commits only when fn
// returns nil.
func (s *Store) Tx(fn func(tx *sql.Tx) error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS batches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			sensor TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS imaging_params (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES batches(id),
			wavelength_m REAL NOT NULL,
			slant_range_m REAL NOT NULL,
			aperture_len_m REAL NOT NULL,
			polarization TEXT NOT NULL,
			orbit_direction TEXT NOT NULL,
			look_angle_deg REAL NOT NULL,
			attitude_err_deg REAL NOT NULL,
			calibration_id INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			UNIQUE(batch_id)
		)`,
		`CREATE TABLE IF NOT EXISTS calibration_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version INTEGER NOT NULL UNIQUE,
			name TEXT NOT NULL,
			active INTEGER NOT NULL DEFAULT 0,
			first_lobe_db REAL NOT NULL DEFAULT 13.26,
			offset_tolerance REAL NOT NULL DEFAULT 0.25,
			ratio_min_db REAL NOT NULL DEFAULT 6.0,
			ratio_max_db REAL NOT NULL DEFAULT 20.0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS peak_regions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES batches(id),
			region_hash TEXT NOT NULL,
			range_start INTEGER NOT NULL,
			range_end INTEGER NOT NULL,
			azimuth_start INTEGER NOT NULL,
			azimuth_end INTEGER NOT NULL,
			peak_azimuth INTEGER NOT NULL,
			peak_intensity_db REAL NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(batch_id, region_hash)
		)`,
		`CREATE TABLE IF NOT EXISTS candidates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES batches(id),
			main_peak_id INTEGER NOT NULL REFERENCES peak_regions(id),
			sidelobe_peak_id INTEGER NOT NULL REFERENCES peak_regions(id),
			azimuth_offset_m REAL NOT NULL,
			offset_units REAL NOT NULL,
			intensity_ratio_db REAL NOT NULL,
			response_score REAL NOT NULL,
			source TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(batch_id, main_peak_id, sidelobe_peak_id)
		)`,
		`CREATE TABLE IF NOT EXISTS evidence (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			candidate_id INTEGER NOT NULL REFERENCES candidates(id),
			kind TEXT NOT NULL,
			note TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES batches(id),
			version INTEGER NOT NULL,
			status TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(batch_id, version)
		)`,
		`CREATE TABLE IF NOT EXISTS analysis_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES batches(id),
			status TEXT NOT NULL,
			candidates_found INTEGER NOT NULL DEFAULT 0,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_peaks_batch ON peak_regions(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cands_batch ON candidates(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_evidence_cand ON evidence(candidate_id)`,
		`CREATE INDEX IF NOT EXISTS idx_snap_batch ON snapshots(batch_id)`,
	}
	for _, st := range stmts {
		if _, err := s.db.Exec(st); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }
