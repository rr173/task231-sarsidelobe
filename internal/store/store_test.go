package store

import (
	"errors"
	"sync"
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/peak"
)

func TestStoreRoundTripAndCalibrationActivation(t *testing.T) {
	dbPath := t.TempDir() + "/roundtrip.db"
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	b, err := st.CreateBatch("B-1", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	if _, err := st.CreateBatch("B-1", "duplicate", "sensor"); !errors.Is(err, model.ErrDuplicate) {
		t.Fatalf("duplicate CreateBatch() error = %v, want %v", err, model.ErrDuplicate)
	}
	cal, err := st.CreateCalibration("cal-1", 13.26, 0.25, 6, 20)
	if err != nil {
		t.Fatalf("CreateCalibration() error = %v", err)
	}
	if _, err := st.ActivateCalibration(cal.ID); err != nil {
		t.Fatalf("ActivateCalibration() error = %v", err)
	}
	p := model.PeakRegion{BatchID: b.ID, RegionHash: peak.HashRegion(&model.PeakRegion{RangeStart: 1, RangeEnd: 3, AzimuthStart: 10, AzimuthEnd: 12, PeakAzimuth: 11, PeakIntensityDB: 40}), RangeStart: 1, RangeEnd: 3, AzimuthStart: 10, AzimuthEnd: 12, PeakAzimuth: 11, PeakIntensityDB: 40, Status: model.PeakRaw}
	if err := st.InsertPeakRegions(b.ID, []model.PeakRegion{p}); err != nil {
		t.Fatalf("InsertPeakRegions() error = %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	st, err = Open(dbPath)
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	defer st.Close()
	batches, err := st.ListBatches()
	if err != nil || len(batches) != 1 || batches[0].Code != "B-1" {
		t.Fatalf("ListBatches() = %+v, err=%v", batches, err)
	}
	active, err := st.GetActiveCalibration()
	if err != nil || active == nil || !active.Active {
		t.Fatalf("GetActiveCalibration() = %+v, err=%v", active, err)
	}
	peaks, err := st.ListPeakRegions(b.ID)
	if err != nil || len(peaks) != 1 || peaks[0].PeakAzimuth != 11 {
		t.Fatalf("ListPeakRegions() = %+v, err=%v", peaks, err)
	}
}

// TestStoreCreateCalibrationConcurrent drives many concurrent calibration
// creations and asserts that the resulting versions form a complete, unique,
// contiguous sequence — the guarantee that broke under the old read-then-write
// race on MAX(version)+1.
func TestStoreCreateCalibrationConcurrent(t *testing.T) {
	st, err := Open(t.TempDir() + "/concurrent.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()

	const n = 20
	var wg sync.WaitGroup
	results := make([]*model.CalibrationVersion, n)
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			v, e := st.CreateCalibration("cal", 13.26, 0.25, 6, 20)
			results[i], errs[i] = v, e
		}(i)
	}
	wg.Wait()

	seen := make(map[int]bool, n)
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("CreateCalibration()[%d] error = %v", i, errs[i])
		}
		ver := results[i].Version
		if ver < 1 || ver > n {
			t.Fatalf("version %d out of range [1,%d]", ver, n)
		}
		if seen[ver] {
			t.Fatalf("duplicate version %d", ver)
		}
		seen[ver] = true
	}
	for v := 1; v <= n; v++ {
		if !seen[v] {
			t.Fatalf("missing version %d from contiguous sequence", v)
		}
	}
}
