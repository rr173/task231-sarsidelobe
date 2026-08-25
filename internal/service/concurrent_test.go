package service

import (
	"sync"
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/store"
)

// TestRegisterPeaksConcurrentIdempotent reproduces the concurrent duplicate
// registration scenario: many goroutines register the very same peak region
// against one batch. The UNIQUE(batch_id, region_hash) constraint must not
// surface as an error — each duplicate request is a no-op (inserted=0) and
// exactly one region row survives in the store.
func TestRegisterPeaksConcurrentIdempotent(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/concurrent.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("S-CONC", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		t.Fatalf("SubmitBatch() error = %v", err)
	}

	region := model.PeakRegion{
		RangeStart: 100, RangeEnd: 110, AzimuthStart: 495, AzimuthEnd: 505,
		PeakAzimuth: 500, PeakIntensityDB: 45.0,
	}

	const n = 20
	var wg sync.WaitGroup
	errs := make([]error, n)
	inserted := make([]int, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			ins, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{region})
			inserted[i] = ins
			errs[i] = err
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: RegisterPeaks() error = %v (want idempotent success)", i, err)
		}
	}

	peaks, err := svc.ListPeaks(b.ID)
	if err != nil {
		t.Fatalf("ListPeaks() error = %v", err)
	}
	if len(peaks) != 1 {
		t.Fatalf("expected exactly 1 peak region after %d concurrent duplicates, got %d", n, len(peaks))
	}
}
