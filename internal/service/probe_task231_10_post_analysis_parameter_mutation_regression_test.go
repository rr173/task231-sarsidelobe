package service

import (
	"errors"
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/store"
)

func TestPostAnalysisImagingParameterUpdateIsRejected(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("PROBE-10", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		t.Fatalf("SubmitBatch() error = %v", err)
	}
	params := &model.ImagingParams{WavelengthM: 0.031, SlantRangeM: 600000, ApertureLenM: 3100, Polarization: "HH"}
	if _, err := svc.RegisterImagingParams(b.ID, params); err != nil {
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
	if _, err := svc.AnalyzeBatch(b.ID); err != nil {
		t.Fatalf("AnalyzeBatch() error = %v", err)
	}
	params.ApertureLenM = 2500
	if _, err := svc.RegisterImagingParams(b.ID, params); !errors.Is(err, model.ErrStateTransition) {
		t.Fatalf("post-analysis RegisterImagingParams() error = %v, want %v", err, model.ErrStateTransition)
	}
	got, err := svc.GetImagingParams(b.ID)
	if err != nil {
		t.Fatalf("GetImagingParams() error = %v", err)
	}
	if got.ApertureLenM != 3100 {
		t.Fatalf("stored aperture length = %v, want original 3100", got.ApertureLenM)
	}
}
