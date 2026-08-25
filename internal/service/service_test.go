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

// TestRegisterPeaksRejectedAfterReview guards the invariant that once a batch
// has entered the review stage the peak set is frozen: appending new regions
// would silently change the data collection the reviewer is judging.
func TestRegisterPeaksRejectedAfterReview(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/service.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("S-r", "scene", "sensor")
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
	if _, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{
		{RangeStart: 100, RangeEnd: 110, AzimuthStart: 495, AzimuthEnd: 505, PeakAzimuth: 500, PeakIntensityDB: 45},
		{RangeStart: 102, RangeEnd: 112, AzimuthStart: 504, AzimuthEnd: 510, PeakAzimuth: 506, PeakIntensityDB: 31.8},
	}); err != nil {
		t.Fatalf("RegisterPeaks() error = %v", err)
	}
	// Peaks are still appendable while the batch awaits diagnosis.
	if _, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{
		{RangeStart: 300, RangeEnd: 310, AzimuthStart: 700, AzimuthEnd: 710, PeakAzimuth: 705, PeakIntensityDB: 22},
	}); err != nil {
		t.Fatalf("pre-review RegisterPeaks() error = %v", err)
	}
	// The batch enters the review stage (the same transition AnalyzeBatch
	// drives once it finds candidates needing a human verdict).
	if _, err := svc.RequestReview(b.ID); err != nil {
		t.Fatalf("RequestReview() error = %v", err)
	}
	// Appending a fresh region after review must be rejected: the data set the
	// reviewer works against is immutable from this point on.
	if _, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{
		{RangeStart: 400, RangeEnd: 410, AzimuthStart: 900, AzimuthEnd: 910, PeakAzimuth: 905, PeakIntensityDB: 25},
	}); err != model.ErrStateTransition {
		t.Fatalf("post-review RegisterPeaks() error = %v, want %v", err, model.ErrStateTransition)
	}
	// The same freeze holds once the diagnosis is confirmed.
	if _, err := svc.ConfirmBatch(b.ID); err != nil {
		t.Fatalf("ConfirmBatch() error = %v", err)
	}
	if _, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{
		{RangeStart: 500, RangeEnd: 510, AzimuthStart: 1000, AzimuthEnd: 1010, PeakAzimuth: 1005, PeakIntensityDB: 30},
	}); err != model.ErrStateTransition {
		t.Fatalf("post-confirm RegisterPeaks() error = %v, want %v", err, model.ErrStateTransition)
	}
	// And after archival it surfaces the dedicated archived-mutation error.
	if _, err := svc.ArchiveBatch(b.ID); err != nil {
		t.Fatalf("ArchiveBatch() error = %v", err)
	}
	if _, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{
		{RangeStart: 600, RangeEnd: 610, AzimuthStart: 1100, AzimuthEnd: 1110, PeakAzimuth: 1105, PeakIntensityDB: 35},
	}); err != model.ErrArchivedMutation {
		t.Fatalf("archived RegisterPeaks() error = %v, want %v", err, model.ErrArchivedMutation)
	}
}
