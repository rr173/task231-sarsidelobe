package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task231-sarsidelobe/internal/model"
)

// CreateSnapshot inserts a new snapshot version for a batch. Version numbers
// start at 1 and increment per batch.
func (s *Store) CreateSnapshot(batchID int64, content string) (*model.Snapshot, error) {
	var batchStatus string
	if err := s.db.QueryRow(`SELECT status FROM batches WHERE id=?`, batchID).Scan(&batchStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	if model.IsBatchImmutable(batchStatus) {
		return nil, model.ErrArchivedMutation
	}
	// A snapshot may only be created once the batch diagnosis is confirmed;
	// rejecting here keeps the store consistent with the publish lifecycle
	// and guarantees an unconfirmed batch never leaves a draft snapshot row.
	if !model.CanPublishSnapshot(batchStatus) {
		return nil, model.ErrStateTransition
	}
	var version int
	err := s.db.QueryRow(
		`SELECT COALESCE(MAX(version),0)+1 FROM snapshots WHERE batch_id=?`, batchID).Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("next snapshot version: %w", err)
	}
	snap := &model.Snapshot{
		BatchID:   batchID,
		Version:   version,
		Status:    model.SnapDraft,
		Content:   content,
		CreatedAt: nowISO(),
		UpdatedAt: nowISO(),
	}
	res, err := s.db.Exec(`
		INSERT INTO snapshots(batch_id,version,status,content,created_at,updated_at)
		VALUES(?,?,?,?,?,?)`,
		snap.BatchID, snap.Version, snap.Status, snap.Content, snap.CreatedAt, snap.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}
	snap.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return snap, nil
}

// GetSnapshot fetches one snapshot by id.
func (s *Store) GetSnapshot(id int64) (*model.Snapshot, error) {
	row := s.db.QueryRow(`
		SELECT id,batch_id,version,status,content,created_at,updated_at
		FROM snapshots WHERE id=?`, id)
	var snap model.Snapshot
	err := row.Scan(&snap.ID, &snap.BatchID, &snap.Version, &snap.Status,
		&snap.Content, &snap.CreatedAt, &snap.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	return &snap, nil
}

// ListSnapshots returns all snapshots of a batch, newest first.
func (s *Store) ListSnapshots(batchID int64) ([]model.Snapshot, error) {
	rows, err := s.db.Query(`
		SELECT id,batch_id,version,status,content,created_at,updated_at
		FROM snapshots WHERE batch_id=? ORDER BY version DESC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()
	var out []model.Snapshot
	for rows.Next() {
		var snap model.Snapshot
		if err := rows.Scan(&snap.ID, &snap.BatchID, &snap.Version, &snap.Status,
			&snap.Content, &snap.CreatedAt, &snap.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// UpdateSnapshotStatus moves a snapshot along its state machine.
func (s *Store) UpdateSnapshotStatus(id int64, status string) error {
	res, err := s.db.Exec(`UPDATE snapshots SET status=?, updated_at=? WHERE id=?`,
		status, nowISO(), id)
	if err != nil {
		return fmt.Errorf("update snapshot status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// ReplaceSnapshot atomically supersedes a published snapshot and creates the
// next published version with the supplied frozen content.
func (s *Store) ReplaceSnapshot(oldID int64, content string) (*model.Snapshot, error) {
	var out model.Snapshot
	err := s.Tx(func(tx *sql.Tx) error {
		var batchID int64
		var status string
		if err := tx.QueryRow(`SELECT batch_id,status FROM snapshots WHERE id=?`, oldID).Scan(&batchID, &status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return model.ErrNotFound
			}
			return err
		}
		if status != model.SnapPublished {
			return model.ErrStateTransition
		}
		var version int
		if err := tx.QueryRow(`SELECT COALESCE(MAX(version),0)+1 FROM snapshots WHERE batch_id=?`, batchID).Scan(&version); err != nil {
			return err
		}
		now := nowISO()
		if _, err := tx.Exec(`UPDATE snapshots SET status=?, updated_at=? WHERE id=?`, model.SnapSuperseded, now, oldID); err != nil {
			return err
		}
		res, err := tx.Exec(`INSERT INTO snapshots(batch_id,version,status,content,created_at,updated_at) VALUES(?,?,?,?,?,?)`, batchID, version, model.SnapPublished, content, now, now)
		if err != nil {
			return err
		}
		out = model.Snapshot{ID: 0, BatchID: batchID, Version: version, Status: model.SnapPublished, Content: content, CreatedAt: now, UpdatedAt: now}
		out.ID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// StartAnalysisRun records a pending analysis run.
func (s *Store) StartAnalysisRun(batchID int64) (*model.AnalysisRun, error) {
	r := &model.AnalysisRun{
		BatchID:   batchID,
		Status:    model.RunRunning,
		StartedAt: nowISO(),
	}
	res, err := s.db.Exec(
		`INSERT INTO analysis_runs(batch_id,status,candidates_found,started_at,finished_at) VALUES(?,?,0,?,'')`,
		r.BatchID, r.Status, r.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("start analysis run: %w", err)
	}
	r.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r, nil
}

// FinishAnalysisRun marks an analysis run as finished with a candidate count.
func (s *Store) FinishAnalysisRun(runID int64, candidates int, failed bool) error {
	status := model.RunFinished
	if failed {
		status = model.RunFailed
	}
	_, err := s.db.Exec(
		`UPDATE analysis_runs SET status=?, candidates_found=?, finished_at=? WHERE id=?`,
		status, candidates, nowISO(), runID)
	if err != nil {
		return fmt.Errorf("finish analysis run: %w", err)
	}
	return nil
}

// ListAnalysisRuns returns analysis runs for a batch, newest first.
func (s *Store) ListAnalysisRuns(batchID int64) ([]model.AnalysisRun, error) {
	rows, err := s.db.Query(`
		SELECT id,batch_id,status,candidates_found,started_at,finished_at
		FROM analysis_runs WHERE batch_id=? ORDER BY id DESC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list analysis runs: %w", err)
	}
	defer rows.Close()
	var out []model.AnalysisRun
	for rows.Next() {
		var r model.AnalysisRun
		if err := rows.Scan(&r.ID, &r.BatchID, &r.Status, &r.CandidatesFound,
			&r.StartedAt, &r.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
