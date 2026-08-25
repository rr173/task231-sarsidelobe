package service

import (
	"sync"
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/store"
)

func TestConcurrentPeakRegistrationIsIdempotent(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("PROBE-6", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	region := []model.PeakRegion{{RangeStart: 10, RangeEnd: 20, AzimuthStart: 100, AzimuthEnd: 110, PeakAzimuth: 105, PeakIntensityDB: 40}}
	const workers = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	inserted := make(chan int, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			n, err := svc.RegisterPeaks(b.ID, region)
			inserted <- n
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	close(inserted)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent RegisterPeaks() error = %v, want idempotent success", err)
		}
	}
	totalInserted := 0
	for n := range inserted {
		totalInserted += n
	}
	if totalInserted != 1 {
		t.Fatalf("sum of inserted counts = %d, want 1", totalInserted)
	}
	peaks, err := svc.ListPeaks(b.ID)
	if err != nil {
		t.Fatalf("ListPeaks() error = %v", err)
	}
	if len(peaks) != 1 {
		t.Fatalf("persisted peaks = %d, want 1", len(peaks))
	}
}
