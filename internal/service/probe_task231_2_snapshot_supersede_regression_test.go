package service

import (
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/store"
)

func publishedSnapshotForProbe(t *testing.T) (*Service, int64, *model.Snapshot) {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := New(st)
	b, err := svc.CreateBatch("PROBE-2", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		t.Fatalf("SubmitBatch() error = %v", err)
	}
	if _, err := svc.RegisterImagingParams(b.ID, &model.ImagingParams{
		WavelengthM: 0.031, SlantRangeM: 600000, ApertureLenM: 3100, Polarization: "HH",
	}); err != nil {
		t.Fatalf("RegisterImagingParams() error = %v", err)
	}
	if _, err := svc.EnsureCalibrationActive("probe", 13.26); err != nil {
		t.Fatalf("EnsureCalibrationActive() error = %v", err)
	}
	if _, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{
		{RangeStart: 100, RangeEnd: 110, AzimuthStart: 495, AzimuthEnd: 505, PeakAzimuth: 500, PeakIntensityDB: 45},
		{RangeStart: 102, RangeEnd: 112, AzimuthStart: 504, AzimuthEnd: 510, PeakAzimuth: 506, PeakIntensityDB: 31.8},
	}); err != nil {
		t.Fatalf("RegisterPeaks() error = %v", err)
	}
	if _, err := svc.AnalyzeAndReview(b.ID); err != nil {
		t.Fatalf("AnalyzeAndReview() error = %v", err)
	}
	if _, err := svc.ConfirmBatch(b.ID); err != nil {
		t.Fatalf("ConfirmBatch() error = %v", err)
	}
	snap, err := svc.PublishSnapshot(b.ID)
	if err != nil || snap.Status != model.SnapPublished {
		t.Fatalf("PublishSnapshot() = %+v, err=%v", snap, err)
	}
	return svc, b.ID, snap
}

func TestSupersedeCreatesReplacementSnapshot(t *testing.T) {
	svc, batchID, old := publishedSnapshotForProbe(t)
	newSnap, err := svc.SupersedeSnapshot(old.ID)
	if err != nil {
		t.Fatalf("SupersedeSnapshot() error = %v", err)
	}
	if newSnap.Status != model.SnapPublished || newSnap.Version != old.Version+1 {
		t.Fatalf("replacement snapshot = %+v, want published version %d", newSnap, old.Version+1)
	}
	snaps, err := svc.ListSnapshots(batchID)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}
	if len(snaps) != 2 || snaps[1].Status != model.SnapSuperseded || snaps[0].Status != model.SnapPublished {
		t.Fatalf("snapshots after supersede = %+v, want old superseded and new published", snaps)
	}
}
