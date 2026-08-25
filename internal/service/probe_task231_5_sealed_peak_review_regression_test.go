package service

import (
	"errors"
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/store"
)

func TestConfirmCandidateDoesNotMutateSealedPeak(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("PROBE-5", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		t.Fatalf("SubmitBatch() error = %v", err)
	}
	if _, err := svc.RegisterImagingParams(b.ID, &model.ImagingParams{WavelengthM: 0.031, SlantRangeM: 600000, ApertureLenM: 3100, Polarization: "HH"}); err != nil {
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
	cands, err := svc.AnalyzeBatch(b.ID)
	if err != nil || len(cands) != 1 {
		t.Fatalf("AnalyzeBatch() = %+v, err=%v", cands, err)
	}
	if _, err := svc.ExcludePeak(cands[0].SidelobePeakID); err != nil {
		t.Fatalf("ExcludePeak() error = %v", err)
	}
	if _, err := svc.ConfirmCandidate(cands[0].ID); !errors.Is(err, model.ErrPeakSealed) {
		t.Fatalf("ConfirmCandidate() error = %v, want %v", err, model.ErrPeakSealed)
	}
	got, err := svc.GetCandidate(cands[0].ID)
	if err != nil {
		t.Fatalf("GetCandidate() error = %v", err)
	}
	if got.Status != model.CandGenerated {
		t.Fatalf("candidate status = %q, want generated after rejected propagation", got.Status)
	}
	peak, err := svc.GetPeak(cands[0].SidelobePeakID)
	if err != nil {
		t.Fatalf("GetPeak() error = %v", err)
	}
	if peak.Status != model.PeakExcluded {
		t.Fatalf("sealed peak status = %q, want excluded", peak.Status)
	}
}
