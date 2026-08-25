package service

import (
	"sync"
	"testing"

	"task231-sarsidelobe/internal/store"
)

func TestConcurrentCalibrationCreationUsesUniqueVersions(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	const workers = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := svc.CreateCalibration("cal", 13.26+float64(i), 0.25, 6, 20+float64(i))
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent CreateCalibration() error = %v", err)
		}
	}
	all, err := svc.ListCalibrations()
	if err != nil {
		t.Fatalf("ListCalibrations() error = %v", err)
	}
	if len(all) != workers {
		t.Fatalf("calibration count = %d, want %d", len(all), workers)
	}
	seen := map[int]bool{}
	for _, cal := range all {
		if seen[cal.Version] {
			t.Fatalf("duplicate calibration version %d in %+v", cal.Version, all)
		}
		seen[cal.Version] = true
	}
}
