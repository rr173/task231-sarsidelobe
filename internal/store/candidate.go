package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task231-sarsidelobe/internal/model"
)

// InsertCandidates bulk-inserts analysis results. The pair
// (batch_id, main_peak_id, sidelobe_peak_id) is unique so re-running analysis
// is idempotent per pair.
func (s *Store) InsertCandidates(cands []model.Candidate) error {
	return s.Tx(func(tx *sql.Tx) error {
		for i := range cands {
			c := &cands[i]
			res, err := tx.Exec(`
				INSERT INTO candidates(batch_id,main_peak_id,sidelobe_peak_id,
					azimuth_offset_m,offset_units,intensity_ratio_db,response_score,
					source,status,created_at,updated_at)
				VALUES(?,?,?,?,?,?,?,?,?,?,?)
				ON CONFLICT(batch_id,main_peak_id,sidelobe_peak_id) DO NOTHING`,
				c.BatchID, c.MainPeakID, c.SidelobePeakID,
				c.AzimuthOffsetM, c.OffsetUnits, c.IntensityRatioDB, c.ResponseScore,
				c.Source, model.CandGenerated, nowISO(), nowISO())
			if err != nil {
				return err
			}
			id, err := res.LastInsertId()
			if err != nil {
				return err
			}
			c.ID = id
		}
		return nil
	})
}

// ListCandidates returns candidates for a batch (optionally filtered by
// status), newest first.
func (s *Store) ListCandidates(batchID int64, status string) ([]model.Candidate, error) {
	q := `SELECT id,batch_id,main_peak_id,sidelobe_peak_id,azimuth_offset_m,offset_units,
			intensity_ratio_db,response_score,source,status,created_at,updated_at
		FROM candidates WHERE batch_id=?`
	args := []any{batchID}
	if status != "" {
		q += ` AND status=?`
		args = append(args, status)
	}
	q += ` ORDER BY id DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}
	defer rows.Close()
	var out []model.Candidate
	for rows.Next() {
		var c model.Candidate
		if err := scanCandidateRows(rows, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCandidate fetches one candidate by id.
func (s *Store) GetCandidate(id int64) (*model.Candidate, error) {
	row := s.db.QueryRow(`
		SELECT id,batch_id,main_peak_id,sidelobe_peak_id,azimuth_offset_m,offset_units,
			intensity_ratio_db,response_score,source,status,created_at,updated_at
		FROM candidates WHERE id=?`, id)
	var c model.Candidate
	err := scanCandidateRows(row, &c)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get candidate: %w", err)
	}
	return &c, nil
}

// UpdateCandidateStatus moves a candidate along its state machine.
func (s *Store) UpdateCandidateStatus(id int64, status string) error {
	res, err := s.db.Exec(`UPDATE candidates SET status=?, updated_at=? WHERE id=?`,
		status, nowISO(), id)
	if err != nil {
		return fmt.Errorf("update candidate status: %w", err)
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

// AddEvidence attaches a review-time evidence record to a candidate.
func (s *Store) AddEvidence(candidateID int64, kind, note string) (*model.Evidence, error) {
	e := &model.Evidence{Kind: kind, Note: note, CreatedAt: nowISO()}
	res, err := s.db.Exec(
		`INSERT INTO evidence(candidate_id,kind,note,created_at) VALUES(?,?,?,?)`,
		candidateID, kind, note, e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("add evidence: %w", err)
	}
	e.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return e, nil
}

// ListEvidence returns evidence records for a candidate.
func (s *Store) ListEvidence(candidateID int64) ([]model.Evidence, error) {
	rows, err := s.db.Query(
		`SELECT id,candidate_id,kind,note,created_at FROM evidence WHERE candidate_id=? ORDER BY id`,
		candidateID)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	defer rows.Close()
	var out []model.Evidence
	for rows.Next() {
		var e model.Evidence
		if err := rows.Scan(&e.ID, &e.CandidateID, &e.Kind, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ResolvePeakByCandidate sets the source peak regions' final states: the main
// peak becomes a strong scatterer and the paired peak becomes a sidelobe when
// the candidate is confirmed.
func (s *Store) ResolvePeakByCandidate(c *model.Candidate, confirmed bool) error {
	mainStatus, sideStatus := model.PeakScatter, model.PeakExcluded
	if confirmed {
		sideStatus = model.PeakSidelobe
	}
	// Analysis produces candidates from raw regions. Promote each source to the
	// intermediate candidate state before applying the reviewed conclusion so
	// the persisted peak lifecycle remains valid.
	for _, id := range []int64{c.MainPeakID, c.SidelobePeakID} {
		peak, err := s.GetPeakRegion(id)
		if err != nil {
			return err
		}
		if peak.Status == model.PeakRaw {
			if err := s.UpdatePeakStatus(id, model.PeakCandidate); err != nil {
				return err
			}
		}
	}
	if err := s.UpdatePeakStatus(c.MainPeakID, mainStatus); err != nil {
		return err
	}
	return s.UpdatePeakStatus(c.SidelobePeakID, sideStatus)
}

// ResolveCandidate atomically records the candidate verdict and propagates it
// to both source peaks. A sealed source peak rejects the whole transition.
func (s *Store) ResolveCandidate(c *model.Candidate, next string, confirmed bool) error {
	if err := s.UpdateCandidateStatus(c.ID, next); err != nil {
		return err
	}
	if confirmed {
		return s.ResolvePeakByCandidate(c, true)
	}
	if next == model.CandRejected {
		return s.ResolvePeakByCandidate(c, false)
	}
	return nil
}

func scanCandidateRows(r rowScanner, c *model.Candidate) error {
	return r.Scan(&c.ID, &c.BatchID, &c.MainPeakID, &c.SidelobePeakID,
		&c.AzimuthOffsetM, &c.OffsetUnits, &c.IntensityRatioDB, &c.ResponseScore,
		&c.Source, &c.Status, &c.CreatedAt, &c.UpdatedAt)
}
