// Command sarsidelobe is the entry point for the synthetic aperture radar
// (SAR) sidelobe contamination diagnosis service. It supports a long-running
// HTTP server and a --smoke-test mode that exercises the full diagnosis loop
// and verifies persistence / restart recovery against the same SQLite file.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"task231-sarsidelobe/internal/httpapi"
	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/service"
	"task231-sarsidelobe/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "sarsidelobe.db", "SQLite database path")
	smoke := flag.Bool("smoke-test", false, "run self-test loop and exit")
	flag.Parse()

	if *smoke {
		if err := runSmokeTest(*dbPath); err != nil {
			log.Fatalf("smoke-test failed: %v", err)
		}
		fmt.Println("smoke-test: OK")
		os.Exit(0)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	svc := service.New(st)
	srv := httpapi.New(svc, st, *addr)
	log.Printf("sarsidelobe listening on %s (db=%s)", *addr, *dbPath)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// runSmokeTest builds a batch, registers imaging parameters and peak
// regions, runs the sidelobe analysis, reviews the candidate, publishes a
// snapshot and then closes and reopens the database to prove persistence and
// restart recovery.
func runSmokeTest(dbPath string) error {
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	svc := service.New(st)

	// 1. batch + imaging parameters (X-band, azimuth resolution 3 m).
	b, err := svc.CreateBatch("SAR-2026-094-SMOKE", "Smoke bright-band scene", "SAR-X1")
	if err != nil {
		return fmt.Errorf("create batch: %w", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		return fmt.Errorf("submit: %w", err)
	}
	params := &model.ImagingParams{
		WavelengthM:    0.031,
		SlantRangeM:    600000.0,
		ApertureLenM:   3100.0,
		Polarization:   "HH",
		OrbitDirection: "descending",
		LookAngleDeg:   35,
		AttitudeErrDeg: 0.2,
	}
	if _, err := svc.RegisterImagingParams(b.ID, params); err != nil {
		return fmt.Errorf("imaging params: %w", err)
	}

	// 2. activate the theoretical sinc calibration.
	if _, err := svc.EnsureCalibrationActive("sinc-theory", 13.26); err != nil {
		return fmt.Errorf("calibration: %w", err)
	}

	// 3. peak regions: strong scatterer at azimuth 500, a suspicious bright
	//    band 4 first-lobe spacings away (azimuth 506), and a weak far peak.
	regions := []model.PeakRegion{
		{RangeStart: 100, RangeEnd: 110, AzimuthStart: 495, AzimuthEnd: 505,
			PeakAzimuth: 500, PeakIntensityDB: 45.0},
		{RangeStart: 102, RangeEnd: 112, AzimuthStart: 504, AzimuthEnd: 510,
			PeakAzimuth: 506, PeakIntensityDB: 31.8},
		{RangeStart: 300, RangeEnd: 310, AzimuthStart: 700, AzimuthEnd: 710,
			PeakAzimuth: 705, PeakIntensityDB: 22.0},
	}
	inserted, err := svc.RegisterPeaks(b.ID, regions)
	if err != nil {
		return fmt.Errorf("register peaks: %w", err)
	}
	if inserted != len(regions) {
		return fmt.Errorf("expected %d inserted peaks, got %d", len(regions), inserted)
	}

	// 4. analyze + auto-review: the bright band matches the 4th sidelobe
	//    position (18 m / 4.5 m = 4.0 lobe units) with a 13.2 dB ratio.
	cands, err := svc.AnalyzeAndReview(b.ID)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	if len(cands) != 1 {
		return fmt.Errorf("expected 1 candidate, got %d", len(cands))
	}
	if cands[0].Source != "sidelobe" {
		return fmt.Errorf("expected sidelobe source, got %s", cands[0].Source)
	}
	if cands[0].Status != model.CandConfirmed {
		return fmt.Errorf("expected confirmed candidate, got %s", cands[0].Status)
	}

	// 5. publish the diagnosis snapshot.
	snap, err := svc.PublishSnapshot(b.ID)
	if err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	if snap.Status != model.SnapPublished {
		return fmt.Errorf("snapshot not published: %s", snap.Status)
	}

	// 6. close and reopen to verify persistence / restart recovery.
	if err := st.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	st2, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	defer st2.Close()
	batches, err := st2.ListBatches()
	if err != nil {
		return fmt.Errorf("list after reopen: %w", err)
	}
	if len(batches) != 1 {
		return fmt.Errorf("expected 1 batch after reopen, got %d", len(batches))
	}
	peaks, err := st2.ListPeakRegions(b.ID)
	if err != nil {
		return fmt.Errorf("peaks after reopen: %w", err)
	}
	if len(peaks) != 3 {
		return fmt.Errorf("expected 3 peaks after reopen, got %d", len(peaks))
	}
	snaps, err := st2.ListSnapshots(b.ID)
	if err != nil {
		return fmt.Errorf("snapshots after reopen: %w", err)
	}
	if len(snaps) != 1 {
		return fmt.Errorf("expected 1 snapshot after reopen, got %d", len(snaps))
	}
	// The confirmed candidate must survive the restart (persistence of the
	// analysis result and its verdict).
	cands2, err := st2.ListCandidates(b.ID, "")
	if err != nil {
		return fmt.Errorf("candidates after reopen: %w", err)
	}
	if len(cands2) != 1 || cands2[0].Status != model.CandConfirmed {
		return fmt.Errorf("expected 1 confirmed candidate after reopen, got %d (status=%s)",
			len(cands2), cands2[0].Status)
	}
	return nil
}
