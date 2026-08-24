// Package review implements the engineer's review policy for contamination
// candidates: attaching evidence, marking a candidate as insufficient,
// confirming or rejecting it, and propagating the verdict onto the source
// peak regions.
package review

import (
	"task231-sarsidelobe/internal/model"
)

// Valid evidence kinds for candidates.
var ValidEvidenceKinds = map[string]bool{
	"attitude_calibration": true, // 姿态校准报告
	"geometry_override":    true, // 几何参数修正
	"operator_note":        true, // 人工备注
}

// ValidateEvidenceKind checks that a kind is one of the supported kinds.
func ValidateEvidenceKind(kind string) error {
	if !ValidEvidenceKinds[kind] {
		return model.ErrBadRequest
	}
	return nil
}

// NeedsReview reports whether a batch should be moved to needs_review after
// analysis: at least one candidate exists and none is confirmed yet.
func NeedsReview(candidates []model.Candidate) bool {
	confirmed := false
	for _, c := range candidates {
		if c.Status == model.CandConfirmed {
			confirmed = true
		}
	}
	return len(candidates) > 0 && !confirmed
}

// Policy is a small value object capturing the review thresholds.
type Policy struct {
	// AttitudeEvidenceConfirmsSource: when a candidate carries an
	// attitude_calibration evidence record, an attitude source is confirmed
	// even without a perfect geometry match.
	AttitudeEvidenceConfirmsSource bool
}

// DefaultPolicy returns the standard review policy.
func DefaultPolicy() Policy {
	return Policy{AttitudeEvidenceConfirmsSource: true}
}

// HasAttitudeEvidence reports whether any evidence record is an attitude
// calibration attachment.
func HasAttitudeEvidence(ev []model.Evidence) bool {
	for _, e := range ev {
		if e.Kind == "attitude_calibration" {
			return true
		}
	}
	return false
}
