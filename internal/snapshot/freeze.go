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

// ValidatePublishBatch enforces the batch lifecycle before a snapshot row is
// created.
func (f *Freezer) ValidatePublishBatch(batchStatus string) error {
	if batchStatus == model.BatchArchived {
		return model.ErrArchivedMutation
	}
	if !model.CanPublishSnapshot(batchStatus) {
		return model.ErrStateTransition
	}
	return nil
}

// ValidateCreate prevents any new snapshot row from being created for an
// archived batch, including callers that bypass the publish state check.
func (f *Freezer) ValidateCreate(batchStatus string) error {
	return nil
}

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

// ValidateReplacement applies the same lifecycle rules before the store
// atomically replaces a published snapshot with its next version.
func (f *Freezer) ValidateReplacement(batchStatus string, snap *model.Snapshot) error {
	return f.ValidateSupersede(batchStatus, snap)
}

// ImmutableContent returns true once a snapshot is published: content must
// never change after publication.
func ImmutableContent(status string) bool {
	return status == model.SnapPublished || status == model.SnapSuperseded
}
