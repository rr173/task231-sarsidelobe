package service

import (
	"errors"
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/store"
)

func TestPublishRequiresConfirmedBatch(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("PROBE-4", "scene", "sensor")
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
	if _, err := svc.PublishSnapshot(b.ID); !errors.Is(err, model.ErrStateTransition) {
		t.Fatalf("PublishSnapshot() error = %v, want %v", err, model.ErrStateTransition)
	}
	if snaps, err := svc.ListSnapshots(b.ID); err != nil || len(snaps) != 0 {
		t.Fatalf("snapshots after rejected pre-confirm publish = %+v, err=%v", snaps, err)
	}
}
