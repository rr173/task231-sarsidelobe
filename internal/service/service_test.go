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

// TestPublishSnapshotRejectedLeavesNoDraft verifies that when a publish
// request is rejected (here because the batch is archived, which is an
// immutable terminal state), no draft snapshot row is left behind: the
// snapshot list must stay empty rather than resurface an unusable draft.
func TestPublishSnapshotRejectedLeavesNoDraft(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/service-reject.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)

	b, err := svc.CreateBatch("S-REJ", "scene", "sensor")
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
	if _, err := svc.AnalyzeAndReview(b.ID); err != nil {
		t.Fatalf("AnalyzeAndReview() error = %v", err)
	}
	if _, err := svc.ConfirmBatch(b.ID); err != nil {
		t.Fatalf("ConfirmBatch() error = %v", err)
	}
	if _, err := svc.ArchiveBatch(b.ID); err != nil {
		t.Fatalf("ArchiveBatch() error = %v", err)
	}

	// An archived batch cannot be snapshotted; the publish request must be
	// rejected.
	if _, err := svc.PublishSnapshot(b.ID); err == nil {
		t.Fatal("PublishSnapshot(archived) error = nil, want non-nil")
	}

	// The rejection must not leave any draft snapshot behind.
	snaps, err := svc.ListSnapshots(b.ID)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots after rejected publish, got %d (%+v)", len(snaps), snaps)
	}
}
