package service

import (
	"encoding/json"
	"testing"

	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/snapshot"
	"task231-sarsidelobe/internal/store"
)

// publishConfirmedBatch drives a batch through the full lifecycle up to a
// published snapshot so supersede tests start from a known state.
func publishConfirmedBatch(t *testing.T, svc *Service) *model.Batch {
	t.Helper()
	b, err := svc.CreateBatch("SUP-1", "scene", "sensor")
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}
	if _, err := svc.SubmitBatch(b.ID); err != nil {
		t.Fatalf("SubmitBatch() error = %v", err)
	}
	if _, err := svc.RegisterImagingParams(b.ID, &model.ImagingParams{
		WavelengthM: 0.031, SlantRangeM: 600000, ApertureLenM: 3100,
		Polarization: "HH", LookAngleDeg: 35,
	}); err != nil {
		t.Fatalf("RegisterImagingParams() error = %v", err)
	}
	if _, err := svc.EnsureCalibrationActive("sinc", 13.26); err != nil {
		t.Fatalf("EnsureCalibrationActive() error = %v", err)
	}
	inserted, err := svc.RegisterPeaks(b.ID, []model.PeakRegion{
		{RangeStart: 100, RangeEnd: 110, AzimuthStart: 495, AzimuthEnd: 505, PeakAzimuth: 500, PeakIntensityDB: 45},
		{RangeStart: 102, RangeEnd: 112, AzimuthStart: 504, AzimuthEnd: 510, PeakAzimuth: 506, PeakIntensityDB: 31.8},
	})
	if err != nil || inserted != 2 {
		t.Fatalf("RegisterPeaks() inserted=%d, err=%v", inserted, err)
	}
	if _, err := svc.AnalyzeAndReview(b.ID); err != nil {
		t.Fatalf("AnalyzeAndReview() error = %v", err)
	}
	if _, err := svc.ConfirmBatch(b.ID); err != nil {
		t.Fatalf("ConfirmBatch() error = %v", err)
	}
	if _, err := svc.PublishSnapshot(b.ID); err != nil {
		t.Fatalf("PublishSnapshot() error = %v", err)
	}
	return b
}

// TestSupersedeCreatesNextPublishedVersion reproduces the reported defect:
// superseding the first published snapshot must atomically produce the next
// published version and leave the old one superseded, instead of merely
// flagging the old version and returning it.
func TestSupersedeCreatesNextPublishedVersion(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/supersede.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b := publishConfirmedBatch(t, svc)

	first, err := svc.ListSnapshots(b.ID)
	if err != nil || len(first) != 1 || first[0].Status != model.SnapPublished || first[0].Version != 1 {
		t.Fatalf("precondition: snapshots = %+v, err=%v (want one published v1)", first, err)
	}
	oldID := first[0].ID

	// The old published snapshot is now superseded. A new published version
	// with the next version number must be produced atomically.
	got, err := svc.SupersedeSnapshot(oldID)
	if err != nil {
		t.Fatalf("SupersedeSnapshot() error = %v", err)
	}
	if got.Status != model.SnapPublished {
		t.Fatalf("returned snapshot status = %q, want %q", got.Status, model.SnapPublished)
	}
	if got.Version != 2 {
		t.Fatalf("returned snapshot version = %d, want 2", got.Version)
	}
	if got.ID == oldID {
		t.Fatalf("returned snapshot id = old id %d, want a brand-new row", got.ID)
	}

	// Persisted state: the batch keeps exactly one published version and the
	// superseded old version is retained for the evidence chain.
	snaps, err := svc.ListSnapshots(b.ID)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}
	var published, superseded []model.Snapshot
	for _, s := range snaps {
		switch s.Status {
		case model.SnapPublished:
			published = append(published, s)
		case model.SnapSuperseded:
			superseded = append(superseded, s)
		}
	}
	if len(published) != 1 || published[0].Version != 2 {
		t.Fatalf("published snapshots = %+v, want exactly one published v2", published)
	}
	if len(superseded) != 1 || superseded[0].ID != oldID || superseded[0].Version != 1 {
		t.Fatalf("superseded snapshots = %+v, want the old v1 retained as superseded", superseded)
	}
	if !snapshot.ImmutableContent(superseded[0].Status) {
		t.Fatalf("superseded snapshot must remain content-immutable")
	}
}

// TestSupersedeReturnsNewRowNotOld guards against the original defect where
// supersede only flipped the old snapshot to superseded and returned the old
// row. The returned snapshot must be a brand-new published row built from the
// current diagnosis; the old row stays superseded with its v1 content frozen.
func TestSupersedeReturnsNewRowNotOld(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/supersede-content.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b := publishConfirmedBatch(t, svc)

	old, err := svc.ListSnapshots(b.ID)
	if err != nil || len(old) != 1 {
		t.Fatalf("precondition: snapshots = %+v, err=%v", old, err)
	}
	oldV1 := old[0]

	newSnap, err := svc.SupersedeSnapshot(oldV1.ID)
	if err != nil {
		t.Fatalf("SupersedeSnapshot() error = %v", err)
	}
	if newSnap.ID == oldV1.ID {
		t.Fatalf("supersede returned the old row id %d; a new published row must be created", newSnap.ID)
	}
	if newSnap.Status != model.SnapPublished || newSnap.Version != 2 {
		t.Fatalf("returned snapshot = %+v, want published v2", newSnap)
	}
	// The new version carries freshly built, valid frozen diagnosis content.
	var got snapshot.Content
	if err := json.Unmarshal([]byte(newSnap.Content), &got); err != nil {
		t.Fatalf("new snapshot content is invalid JSON: %v", err)
	}
	if got.BatchCode != b.Code || got.FinalCandidates == 0 {
		t.Fatalf("new snapshot content = %+v, want rebuilt diagnosis for %q", got, b.Code)
	}

	// The superseded old version is retained with its original content frozen
	// (the evidence chain must not change once published).
	oldNow, err := svc.GetSnapshot(oldV1.ID)
	if err != nil {
		t.Fatalf("GetSnapshot(old) error = %v", err)
	}
	if oldNow.Status != model.SnapSuperseded {
		t.Fatalf("old snapshot status = %q, want %q", oldNow.Status, model.SnapSuperseded)
	}
	if oldNow.Content != oldV1.Content {
		t.Fatalf("old snapshot content changed after supersede; published content must be immutable")
	}
}

// TestSupersedeRejectsNonPublished guards against superseding an already
// superseded (or draft) snapshot, which must not spawn an orphan version.
func TestSupersedeRejectsNonPublished(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/supersede-reject.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	svc := New(st)
	b := publishConfirmedBatch(t, svc)

	old, err := svc.ListSnapshots(b.ID)
	if err != nil || len(old) != 1 {
		t.Fatalf("precondition: snapshots = %+v, err=%v", old, err)
	}
	if _, err := svc.SupersedeSnapshot(old[0].ID); err != nil {
		t.Fatalf("first SupersedeSnapshot() error = %v", err)
	}
	// The old snapshot is now superseded; superseding it again must fail
	// without creating a third version.
	if _, err := svc.SupersedeSnapshot(old[0].ID); err == nil {
		t.Fatalf("superseding a non-published snapshot must fail")
	}
	snaps, err := svc.ListSnapshots(b.ID)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("after double supersede there are %d snapshots, want 2", len(snaps))
	}
}
