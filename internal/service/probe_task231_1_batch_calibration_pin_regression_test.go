package service

import (
	"encoding/json"
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/store"
)

func TestSnapshotUsesCalibrationPinnedToBatch(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("PROBE-1", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		t.Fatalf("SubmitBatch() error = %v", err)
	}
	first, err := svc.EnsureCalibrationActive("first", 13.26)
	if err != nil {
		t.Fatalf("first calibration error = %v", err)
	}
	if _, err := svc.RegisterImagingParams(b.ID, &model.ImagingParams{WavelengthM: 0.031, SlantRangeM: 600000, ApertureLenM: 3100, Polarization: "HH"}); err != nil {
		t.Fatalf("RegisterImagingParams() error = %v", err)
	}
	if first.FirstLobeDB != 13.26 {
		t.Fatalf("first calibration = %+v", first)
	}
	if _, err := svc.EnsureCalibrationActive("second", 19.5); err != nil {
		t.Fatalf("second calibration error = %v", err)
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
	if err != nil {
		t.Fatalf("PublishSnapshot() error = %v", err)
	}
	var content struct {
		CalibrationDB float64 `json:"calibration_first_lobe_db"`
	}
	if err := json.Unmarshal([]byte(snap.Content), &content); err != nil {
		t.Fatalf("snapshot content is invalid JSON: %v", err)
	}
	if content.CalibrationDB != first.FirstLobeDB {
		t.Fatalf("snapshot calibration = %v, want batch-pinned %v", content.CalibrationDB, first.FirstLobeDB)
	}
}
