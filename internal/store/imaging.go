package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task231-sarsidelobe/internal/model"
)

// UpsertImagingParams registers the antenna geometry for a batch. The batch
// can only have one parameter row (UNIQUE(batch_id)); a second registration
// replaces the first.
func (s *Store) UpsertImagingParams(p *model.ImagingParams) (*model.ImagingParams, error) {
	var batchStatus string
	if err := s.db.QueryRow(`SELECT status FROM batches WHERE id=?`, p.BatchID).Scan(&batchStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	if !model.CanUpdateImagingParams(batchStatus) {
		if batchStatus == model.BatchArchived {
			return nil, model.ErrArchivedMutation
		}
		return nil, model.ErrStateTransition
	}
	if p.CalibrationID == 0 {
		active, err := s.GetActiveCalibration()
		if err != nil {
			return nil, err
		}
		if active != nil {
			p.CalibrationID = active.ID
		}
	}
	p.CreatedAt = nowISO()
	_, err := s.db.Exec(`
		INSERT INTO imaging_params(batch_id,wavelength_m,slant_range_m,aperture_len_m,
			polarization,orbit_direction,look_angle_deg,attitude_err_deg,calibration_id,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(batch_id) DO UPDATE SET
			wavelength_m=excluded.wavelength_m,
			slant_range_m=excluded.slant_range_m,
			aperture_len_m=excluded.aperture_len_m,
			polarization=excluded.polarization,
			orbit_direction=excluded.orbit_direction,
			look_angle_deg=excluded.look_angle_deg,
			attitude_err_deg=excluded.attitude_err_deg,
			calibration_id=excluded.calibration_id`,
		p.BatchID, p.WavelengthM, p.SlantRangeM, p.ApertureLenM,
		p.Polarization, p.OrbitDirection, p.LookAngleDeg, p.AttitudeErrDeg,
		p.CalibrationID, p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert imaging params: %w", err)
	}
	return s.GetImagingParams(p.BatchID)
}

// GetImagingParams returns the parameter row for a batch.
func (s *Store) GetImagingParams(batchID int64) (*model.ImagingParams, error) {
	row := s.db.QueryRow(`
		SELECT id,batch_id,wavelength_m,slant_range_m,aperture_len_m,
			polarization,orbit_direction,look_angle_deg,attitude_err_deg,calibration_id,created_at
		FROM imaging_params WHERE batch_id=?`, batchID)
	var p model.ImagingParams
	err := row.Scan(&p.ID, &p.BatchID, &p.WavelengthM, &p.SlantRangeM, &p.ApertureLenM,
		&p.Polarization, &p.OrbitDirection, &p.LookAngleDeg, &p.AttitudeErrDeg,
		&p.CalibrationID, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get imaging params: %w", err)
	}
	return &p, nil
}

// CreateCalibration inserts a new calibration version (version number is
// auto-incremented relative to the current max).
func (s *Store) CreateCalibration(name string, firstLobeDB, offsetTol, ratioMinDB, ratioMaxDB float64) (*model.CalibrationVersion, error) {
	v := &model.CalibrationVersion{Name: name, Active: false, FirstLobeDB: firstLobeDB, OffsetTolerance: offsetTol, RatioMinDB: ratioMinDB, RatioMaxDB: ratioMaxDB, CreatedAt: nowISO()}
	err := s.Tx(func(tx *sql.Tx) error {
		if err := tx.QueryRow(`SELECT COALESCE(MAX(version),0)+1 FROM calibration_versions`).Scan(&v.Version); err != nil {
			return fmt.Errorf("next calibration version: %w", err)
		}
		res, err := tx.Exec(`
			INSERT INTO calibration_versions(version,name,active,first_lobe_db,offset_tolerance,ratio_min_db,ratio_max_db,created_at)
			VALUES(?,?,0,?,?,?,?,?)`,
			v.Version, v.Name, v.FirstLobeDB, v.OffsetTolerance, v.RatioMinDB, v.RatioMaxDB, v.CreatedAt)
		if err != nil {
			return fmt.Errorf("create calibration: %w", err)
		}
		v.ID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return nil, err
	}
	return v, nil
}

// ListCalibrations returns all calibration versions, newest first.
func (s *Store) ListCalibrations() ([]model.CalibrationVersion, error) {
	rows, err := s.db.Query(`
		SELECT id,version,name,active,first_lobe_db,offset_tolerance,ratio_min_db,ratio_max_db,created_at
		FROM calibration_versions ORDER BY version DESC`)
	if err != nil {
		return nil, fmt.Errorf("list calibrations: %w", err)
	}
	defer rows.Close()
	var out []model.CalibrationVersion
	for rows.Next() {
		var v model.CalibrationVersion
		if err := rows.Scan(&v.ID, &v.Version, &v.Name, &v.Active,
			&v.FirstLobeDB, &v.OffsetTolerance, &v.RatioMinDB, &v.RatioMaxDB, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ActivateCalibration deactivates all versions and activates the given one.
func (s *Store) ActivateCalibration(id int64) (*model.CalibrationVersion, error) {
	if err := s.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE calibration_versions SET active=0`); err != nil {
			return err
		}
		res, err := tx.Exec(`UPDATE calibration_versions SET active=1 WHERE id=?`, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return model.ErrNotFound
		}
		return nil
	}); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("activate calibration: %w", err)
	}
	return s.GetCalibration(id)
}

// GetCalibration fetches one version by id.
func (s *Store) GetCalibration(id int64) (*model.CalibrationVersion, error) {
	row := s.db.QueryRow(`
		SELECT id,version,name,active,first_lobe_db,offset_tolerance,ratio_min_db,ratio_max_db,created_at
		FROM calibration_versions WHERE id=?`, id)
	var v model.CalibrationVersion
	err := row.Scan(&v.ID, &v.Version, &v.Name, &v.Active,
		&v.FirstLobeDB, &v.OffsetTolerance, &v.RatioMinDB, &v.RatioMaxDB, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get calibration: %w", err)
	}
	return &v, nil
}

// GetActiveCalibration returns the currently active version, or nil if none.
func (s *Store) GetActiveCalibration() (*model.CalibrationVersion, error) {
	row := s.db.QueryRow(`SELECT id FROM calibration_versions WHERE active=1 LIMIT 1`)
	var id int64
	err := row.Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active calibration: %w", err)
	}
	return s.GetCalibration(id)
}
