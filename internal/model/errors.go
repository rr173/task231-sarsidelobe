// Package model defines the core entities, sentinel errors and state machines
// for the synthetic aperture radar (SAR) sidelobe contamination diagnosis
// service. All layers depend on this package for shared types and invariants.
package model

import "errors"

// Sentinel errors returned across layers. HTTP handlers map them to status
// codes via model.ToStatus.
var (
	// ErrNotFound is returned when an entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrDuplicate is returned when a uniqueness / idempotency key is violated.
	ErrDuplicate = errors.New("duplicate entity")
	// ErrBadRequest is returned for semantically invalid input.
	ErrBadRequest = errors.New("bad request")
	// ErrStateTransition is returned when a state machine transition is illegal.
	ErrStateTransition = errors.New("illegal state transition")
	// ErrArchivedMutation is returned when a caller mutates an archived batch.
	ErrArchivedMutation = errors.New("archived batch is read-only")
	// ErrPolarizationMissing is returned when the polarization mode is not set.
	ErrPolarizationMissing = errors.New("polarization mode missing")
	// ErrCoordinateRange is returned when a peak region coordinate is out of range.
	ErrCoordinateRange = errors.New("peak region coordinate out of range")
	// ErrRepeatedRegion is returned when the same region hash is registered twice.
	ErrRepeatedRegion = errors.New("repeated peak region")
	// ErrAnalyzeLocked is returned when analysis is already running on a batch.
	ErrAnalyzeLocked = errors.New("analysis already running on batch")
	// ErrSnapshotFrozen is returned when a snapshot is superseded and cannot change.
	ErrSnapshotFrozen = errors.New("snapshot already superseded")
	// ErrNoParams is returned when analysis needs imaging parameters that are absent.
	ErrNoParams = errors.New("imaging parameters missing")
	// ErrNoPeaks is returned when analysis needs peak regions that are absent.
	ErrNoPeaks = errors.New("no peak regions registered")
	// ErrPeakSealed is returned when a peak region is excluded and cannot change.
	ErrPeakSealed = errors.New("peak region is sealed")
)
