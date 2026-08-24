package snapshot

import (
	"task231-sarsidelobe/internal/model"
)

// Freezer applies snapshot state machine rules: publishing a draft is only
// legal when the batch has been fully analysed and archived batches cannot be
// snapshotted again.
type Freezer struct{}

// NewFreezer builds a Freezer.
func NewFreezer() *Freezer { return &Freezer{} }

// ValidatePublish checks that publishing is allowed for the batch state and
// that the snapshot is a draft.
func (f *Freezer) ValidatePublish(batchStatus string, snap *model.Snapshot) error {
	if snap.Status != model.SnapDraft {
		return model.ErrStateTransition
	}
	if batchStatus == model.BatchArchived {
		return model.ErrArchivedMutation
	}
	return nil
}

// ValidateSupersede checks that a published snapshot can be superseded and
// that the batch is not archived (frozen verdicts stay immutable).
func (f *Freezer) ValidateSupersede(batchStatus string, snap *model.Snapshot) error {
	if snap.Status != model.SnapPublished {
		return model.ErrStateTransition
	}
	if batchStatus == model.BatchArchived {
		return model.ErrArchivedMutation
	}
	return nil
}

// ImmutableContent returns true once a snapshot is published: content must
// never change after publication.
func ImmutableContent(status string) bool {
	return status == model.SnapPublished || status == model.SnapSuperseded
}
