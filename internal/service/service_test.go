package service

import (
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/store"
)

func TestServiceCompletesDiagnosisLifecycle(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/service.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("S-1", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		t.Fatalf("SubmitBatch() error = %v", err)
	}
	if _, err := svc.RegisterImagingParams(b.ID, &model.ImagingParams{
		WavelengthM: 0.031, SlantRangeM: 600000, ApertureLenM: 3100,
		Polarization: "HH", LookAngleDeg: 35,
	}); err != nil {
		t.Fatalf("RegisterImagingParams() error = %v", err)
	}
	if _, err := svc.EnsureCalibrationActive("sinc", 13.26); err != nil {
		t.Fatalf("EnsureCalibrationActive() error = %v", err)
	}
	inserted, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{
		{RangeStart: 100, RangeEnd: 110, AzimuthStart: 495, AzimuthEnd: 505, PeakAzimuth: 500, PeakIntensityDB: 45},
		{RangeStart: 102, RangeEnd: 112, AzimuthStart: 504, AzimuthEnd: 510, PeakAzimuth: 506, PeakIntensityDB: 31.8},
	})
	if err != nil || inserted != 2 {
		t.Fatalf("RegisterPeaks() inserted=%d, err=%v", inserted, err)
	}
	cands, err := svc.AnalyzeAndReview(b.ID)
	if err != nil || len(cands) != 1 || cands[0].Status != model.CandConfirmed {
		t.Fatalf("AnalyzeAndReview() = %+v, err=%v", cands, err)
	}
	if _, err := svc.ConfirmBatch(b.ID); err != nil {
		t.Fatalf("ConfirmBatch() error = %v", err)
	}
	snap, err := svc.PublishSnapshot(b.ID)
	if err != nil || snap.Status != model.SnapPublished {
		t.Fatalf("PublishSnapshot() = %+v, err=%v", snap, err)
	}
	if _, err := svc.ArchiveBatch(b.ID); err != nil {
		t.Fatalf("ArchiveBatch() error = %v", err)
	}
	if _, err := svc.RegisterImagingParams(b.ID, &model.ImagingParams{WavelengthM: 0.03, SlantRangeM: 1000, ApertureLenM: 10, Polarization: "HH"}); err != model.ErrArchivedMutation {
		t.Fatalf("archived RegisterImagingParams() error = %v, want %v", err, model.ErrArchivedMutation)
	}
}

// TestImagingParamsFrozenAfterReview guards against the regression where
// modifying the imaging parameters succeeds after a batch has entered
// needs_review, silently replacing the geometric basis of the already-
// generated analysis candidates.
func TestImagingParamsFrozenAfterReview(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/review.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("S-2", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		t.Fatalf("SubmitBatch() error = %v", err)
	}
	original := &model.ImagingParams{
		WavelengthM: 0.031, SlantRangeM: 600000, ApertureLenM: 3100,
		Polarization: "HH", LookAngleDeg: 35,
	}
	if _, err := svc.RegisterImagingParams(b.ID, original); err != nil {
		t.Fatalf("RegisterImagingParams() error = %v", err)
	}
	if _, err := svc.EnsureCalibrationActive("sinc", 13.26); err != nil {
		t.Fatalf("EnsureCalibrationActive() error = %v", err)
	}
	if _, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{
		{RangeStart: 100, RangeEnd: 110, AzimuthStart: 495, AzimuthEnd: 505, PeakAzimuth: 500, PeakIntensityDB: 45},
		{RangeStart: 102, RangeEnd: 112, AzimuthStart: 504, AzimuthEnd: 510, PeakAzimuth: 506, PeakIntensityDB: 31.8},
	}); err != nil {
		t.Fatalf("RegisterPeaks() error = %v", err)
	}
	// Analysis moves the batch into needs_review (no candidate is auto-confirmed).
	if _, err := svc.AnalyzeBatch(b.ID); err != nil {
		t.Fatalf("AnalyzeBatch() error = %v", err)
	}
	got, err := st.GetBatch(b.ID)
	if err != nil {
		t.Fatalf("GetBatch() error = %v", err)
	}
	if got.Status != model.BatchReview {
		t.Fatalf("batch status = %q, want %q", got.Status, model.BatchReview)
	}
	// Attempting to replace the imaging parameters now must fail — otherwise the
	// already-generated candidates' geometric basis is silently overwritten.
	_, err = svc.RegisterImagingParams(b.ID, &model.ImagingParams{
		WavelengthM: 0.05, SlantRangeM: 800000, ApertureLenM: 4000,
		Polarization: "VV", LookAngleDeg: 40,
	})
	if err != model.ErrStateTransition {
		t.Fatalf("post-review RegisterImagingParams() error = %v, want %v", err, model.ErrStateTransition)
	}
	// The stored parameters must be untouched.
	persisted, err := svc.GetImagingParams(b.ID)
	if err != nil {
		t.Fatalf("GetImagingParams() error = %v", err)
	}
	if persisted.WavelengthM != original.WavelengthM ||
		persisted.SlantRangeM != original.SlantRangeM ||
		persisted.ApertureLenM != original.ApertureLenM ||
		persisted.Polarization != original.Polarization ||
		persisted.LookAngleDeg != original.LookAngleDeg {
		t.Fatalf("imaging params mutated after review: %+v", persisted)
	}
}
