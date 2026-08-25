package service

import (
	"errors"
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/store"
)

func TestArchivedPublishDoesNotLeaveDraftSnapshot(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b, err := svc.CreateBatch("PROBE-3", "scene", "sensor")
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
	if _, err := svc.ConfirmBatch(b.ID); err != nil {
		t.Fatalf("ConfirmBatch() error = %v", err)
	}
	if _, err := svc.ArchiveBatch(b.ID); err != nil {
		t.Fatalf("ArchiveBatch() error = %v", err)
	}
	if _, err := svc.PublishSnapshot(b.ID); !errors.Is(err, model.ErrArchivedMutation) {
		t.Fatalf("PublishSnapshot() error = %v, want %v", err, model.ErrArchivedMutation)
	}
	snaps, err := svc.ListSnapshots(b.ID)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}
	if len(snaps) != 0 {
		t.Fatalf("snapshots after rejected publish = %+v, want none", snaps)
	}
}
