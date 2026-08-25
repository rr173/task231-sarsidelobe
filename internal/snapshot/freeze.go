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

// ValidateCreate prevents a snapshot row from being created for a batch that
// is not ready to publish: archived batches are read-only and unconfirmed
// batches have no finalized diagnosis to freeze. This mirrors the publish
// check so callers that bypass ValidatePublishBatch cannot leave a draft row.
func (f *Freezer) ValidateCreate(batchStatus string) error {
	if model.IsBatchImmutable(batchStatus) {
		return model.ErrArchivedMutation
	}
	if !model.CanPublishSnapshot(batchStatus) {
		return model.ErrStateTransition
	}
	return nil
}

// ValidatePublish checks that publishing is allowed for the batch state and
// that the snapshot is a draft. The batch must be confirmed (a published
// snapshot freezes a committed diagnosis) and must not be archived.
func (f *Freezer) ValidatePublish(batchStatus string, snap *model.Snapshot) error {
	if snap.Status != model.SnapDraft {
		return model.ErrStateTransition
	}
	if batchStatus == model.BatchArchived {
		return model.ErrArchivedMutation
	}
	if !model.CanPublishSnapshot(batchStatus) {
		return model.ErrStateTransition
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
