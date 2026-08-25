package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task231-sarsidelobe/internal/model"
)

// CreateBatch inserts a new batch; the code must be unique.
func (s *Store) CreateBatch(code, name, sensor string) (*model.Batch, error) {
	b := &model.Batch{
		Code:      code,
		Name:      name,
		Sensor:    sensor,
		Status:    model.BatchReceiving,
		CreatedAt: nowISO(),
		UpdatedAt: nowISO(),
	}
	res, err := s.db.Exec(
		`INSERT INTO batches(code,name,sensor,status,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		b.Code, b.Name, b.Sensor, b.Status, b.CreatedAt, b.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, model.ErrDuplicate
		}
		return nil, fmt.Errorf("create batch: %w", err)
	}
	b.ID, err = res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return b, nil
}

// GetBatch fetches one batch by id.
func (s *Store) GetBatch(id int64) (*model.Batch, error) {
	row := s.db.QueryRow(
		`SELECT id,code,name,sensor,status,created_at,updated_at FROM batches WHERE id=?`, id)
	b, err := scanBatch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get batch: %w", err)
	}
	return b, nil
}

// ListBatches returns all batches, newest first.
func (s *Store) ListBatches() ([]model.Batch, error) {
	rows, err := s.db.Query(
		`SELECT id,code,name,sensor,status,created_at,updated_at FROM batches ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list batches: %w", err)
	}
	defer rows.Close()
	var out []model.Batch
	for rows.Next() {
		var b model.Batch
		if err := scanBatchRows(rows, &b); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateBatchStatus atomically moves a batch to a new status.
func (s *Store) UpdateBatchStatus(id int64, status string) error {
	res, err := s.db.Exec(
		`UPDATE batches SET status=?, updated_at=? WHERE id=?`, status, nowISO(), id)
	if err != nil {
		return fmt.Errorf("update batch status: %w", err)
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBatch(r rowScanner) (*model.Batch, error) {
	var b model.Batch
	if err := scanBatchRows(r, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func scanBatchRows(r rowScanner, b *model.Batch) error {
	return r.Scan(&b.ID, &b.Code, &b.Name, &b.Sensor, &b.Status, &b.CreatedAt, &b.UpdatedAt)
}

func isUniqueViolation(err error) bool {
	msg := err.Error()
	return containsAny(msg, "UNIQUE constraint failed", "constraint failed")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && len(s) >= len(sub) {
			found := false
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					found = true
					break
				}
			}
			if found {
				return true
			}
		}
	}
	return false
}
