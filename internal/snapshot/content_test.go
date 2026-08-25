package snapshot

import (
	"encoding/json"
	"testing"

	"task231-sarsidelobe/internal/imaging"
	"task231-sarsidelobe/internal/model"
)

func TestBuildCapturesCandidateCountsAndGeometry(t *testing.T) {
	content, err := Build("SAR-1", imaging.Geometry{AzimuthResolutionM: 3, FirstLobeSpacingM: 4.5}, imaging.DefaultCalibration(), []model.Candidate{
		{Source: "sidelobe", OffsetUnits: 4, IntensityRatioDB: 13.2, ResponseScore: 0.98, Status: model.CandConfirmed},
		{Source: "scatter", Status: model.CandRejected},
		{Source: "attitude", Status: model.CandInsufficient},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	var got Content
	if err := json.Unmarshal([]byte(content), &got); err != nil {
		t.Fatalf("snapshot JSON is invalid: %v", err)
	}
	if got.BatchCode != "SAR-1" || got.FinalCandidates != 3 || got.Confirmed != 1 || got.Rejected != 1 || got.Insufficient != 1 {
		t.Fatalf("snapshot content = %+v, want preserved counts", got)
	}
	if got.Geometry.FirstLobeSpacingM != 4.5 || len(got.Sources) != 3 {
		t.Fatalf("snapshot geometry/sources = %+v, want geometry and 3 sources", got)
	}
}

func TestFreezerRejectsPublishedAndArchivedMutations(t *testing.T) {
	f := NewFreezer()
	if err := f.ValidatePublish(model.BatchArchived, &model.Snapshot{Status: model.SnapDraft}); err != model.ErrArchivedMutation {
		t.Fatalf("ValidatePublish(archived) = %v, want %v", err, model.ErrArchivedMutation)
	}
	if err := f.ValidatePublish(model.BatchConfirmed, &model.Snapshot{Status: model.SnapPublished}); err != model.ErrStateTransition {
		t.Fatalf("ValidatePublish(published) = %v, want %v", err, model.ErrStateTransition)
	}
	if !ImmutableContent(model.SnapPublished) || !ImmutableContent(model.SnapSuperseded) {
		t.Fatal("published and superseded snapshots must be immutable")
	}
}
