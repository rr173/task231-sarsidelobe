package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task231-sarsidelobe/internal/model"
)

// InsertPeakRegions bulk-inserts regions inside a single transaction. A
// duplicate region hash for the batch aborts the whole insert (idempotency
// is enforced at the service layer by filtering hashes already present).
func (s *Store) InsertPeakRegions(batchID int64, regions []model.PeakRegion) error {
	var batchStatus string
	if err := s.db.QueryRow(`SELECT status FROM batches WHERE id=?`, batchID).Scan(&batchStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		}
		return err
	}
	return s.Tx(func(tx *sql.Tx) error {
		for i := range regions {
			r := &regions[i]
			if r.ID != 0 {
				return fmt.Errorf("insert peak region with preassigned id")
			}
			res, err := tx.Exec(`
				INSERT INTO peak_regions(batch_id,region_hash,range_start,range_end,
					azimuth_start,azimuth_end,peak_azimuth,peak_intensity_db,status,created_at)
				VALUES(?,?,?,?,?,?,?,?,?,?)
				ON CONFLICT(batch_id,region_hash) DO NOTHING`,
				r.BatchID, r.RegionHash, r.RangeStart, r.RangeEnd,
				r.AzimuthStart, r.AzimuthEnd, r.PeakAzimuth, r.PeakIntensityDB,
				r.Status, nowISO())
			if err != nil {
				return err
			}
			id, err := res.LastInsertId()
			if err != nil {
				return err
			}
			r.ID = id
		}
		return nil
	})
}

// ListPeakRegions returns all regions for a batch ordered by azimuth.
func (s *Store) ListPeakRegions(batchID int64) ([]model.PeakRegion, error) {
	rows, err := s.db.Query(`
		SELECT id,batch_id,region_hash,range_start,range_end,azimuth_start,azimuth_end,
			peak_azimuth,peak_intensity_db,status,created_at
		FROM peak_regions WHERE batch_id=? ORDER BY peak_azimuth, id`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list peak regions: %w", err)
	}
	defer rows.Close()
	var out []model.PeakRegion
	for rows.Next() {
		var r model.PeakRegion
		if err := scanPeakRows(rows, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetPeakRegion fetches one region by id.
func (s *Store) GetPeakRegion(id int64) (*model.PeakRegion, error) {
	row := s.db.QueryRow(`
		SELECT id,batch_id,region_hash,range_start,range_end,azimuth_start,azimuth_end,
			peak_azimuth,peak_intensity_db,status,created_at
		FROM peak_regions WHERE id=?`, id)
	var r model.PeakRegion
	err := scanPeakRows(row, &r)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get peak region: %w", err)
	}
	return &r, nil
}

// UpdatePeakStatus moves a peak region to a new state.
func (s *Store) UpdatePeakStatus(id int64, status string) error {
	current, err := s.GetPeakRegion(id)
	if err != nil {
		return err
	}
	if current.Status == model.PeakExcluded || current.Status == model.PeakSidelobe {
		return model.ErrPeakSealed
	}
	if !model.CanPeakTransition(current.Status, status) {
		return model.ErrStateTransition
	}
	res, err := s.db.Exec(`UPDATE peak_regions SET status=? WHERE id=?`, status, id)
	if err != nil {
		return fmt.Errorf("update peak status: %w", err)
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

// ExistingRegionHashes returns the set of region hashes already present for a
// batch, used for idempotent registration.
func (s *Store) ExistingRegionHashes(batchID int64) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT region_hash FROM peak_regions WHERE batch_id=?`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		out[h] = true
	}
	return out, rows.Err()
}

func scanPeakRows(r rowScanner, p *model.PeakRegion) error {
	return r.Scan(&p.ID, &p.BatchID, &p.RegionHash, &p.RangeStart, &p.RangeEnd,
		&p.AzimuthStart, &p.AzimuthEnd, &p.PeakAzimuth, &p.PeakIntensityDB,
		&p.Status, &p.CreatedAt)
}
