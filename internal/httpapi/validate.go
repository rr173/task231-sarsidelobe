package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/service"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// runSelfTest drives the full diagnosis loop through the service layer and
// returns a summary of what was created. It mirrors the --smoke-test flow
// used by the container contract.
func runSelfTest(svc *service.Service) (map[string]any, error) {
	// 1. batch + imaging params (X-band SAR, azimuth resolution ~3 m).
	b, err := svc.CreateBatch("SAR-2026-094-01", "Sample bright-band scene", "SAR-X1")
	if err != nil {
		return nil, fmt.Errorf("create batch: %w", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		return nil, fmt.Errorf("submit batch: %w", err)
	}
	params := &model.ImagingParams{
		WavelengthM:    0.031, // X 波段
		SlantRangeM:    600000.0,
		ApertureLenM:   3100.0, // ρ_a = λR/2L = 3 m
		Polarization:   "HH",
		OrbitDirection: "descending",
		LookAngleDeg:   35,
		AttitudeErrDeg: 0.2,
	}
	if _, err := svc.RegisterImagingParams(b.ID, params); err != nil {
		return nil, fmt.Errorf("register params: %w", err)
	}
	// 2. calibration with the theoretical first-lobe attenuation.
	if _, err := svc.EnsureCalibrationActive("sinc-theory", 13.26); err != nil {
		return nil, fmt.Errorf("activate calibration: %w", err)
	}
	// 3. peak regions: a strong scatterer at azimuth 500 and a suspicious
	//    bright band 2 first-lobe spacings away (azimuth 506; ρ_a = 3 m).
	regions := []model.PeakRegion{
		{RangeStart: 100, RangeEnd: 110, AzimuthStart: 495, AzimuthEnd: 505,
			PeakAzimuth: 500, PeakIntensityDB: 45.0},
		{RangeStart: 102, RangeEnd: 112, AzimuthStart: 504, AzimuthEnd: 510,
			PeakAzimuth: 506, PeakIntensityDB: 31.8},
		{RangeStart: 300, RangeEnd: 310, AzimuthStart: 700, AzimuthEnd: 710,
			PeakAzimuth: 705, PeakIntensityDB: 22.0},
	}
	if _, err := svc.RegisterPeaks(b.ID, regions); err != nil {
		return nil, fmt.Errorf("register peaks: %w", err)
	}
	// 4. analyze + auto-review.
	cands, err := svc.AnalyzeAndReview(b.ID)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	// 5. publish a snapshot.
	snap, err := svc.PublishSnapshot(b.ID)
	if err != nil {
		return nil, fmt.Errorf("publish snapshot: %w", err)
	}
	if snap.Status != model.SnapPublished {
		return nil, fmt.Errorf("snapshot not published: %s", snap.Status)
	}
	return map[string]any{
		"batch_id":     b.ID,
		"candidates":   len(cands),
		"snapshot_id":  snap.ID,
		"snapshot_version": snap.Version,
		"status":       "ok",
	}, nil
}
