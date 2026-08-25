package review

import (
	"task231-sarsidelobe/internal/model"
)

// Verdict is the outcome of reviewing one candidate.
type Verdict struct {
	Confirmed bool   // 确认污染
	Rejected  bool   // 否决污染
	Source    string // 结论来源（sidelobe / attitude）
}

// Decide confirms a candidate when either of the following holds:
//   - the candidate already has strong geometry + amplitude evidence
//     (source sidelobe with high response score), or
//   - an attitude_calibration evidence record supports the attitude source.
//
// Otherwise the verdict is "insufficient evidence" unless the operator
// explicitly rejects it.
func Decide(c model.Candidate, ev []model.Evidence, pol Policy) Verdict {
	switch c.Source {
	case "sidelobe":
		// Geometry and amplitude already matched; response score >= 0.7 is
		// treated as conclusive, otherwise evidence can push it over.
		if c.ResponseScore >= 0.7 {
			return Verdict{Confirmed: true, Source: c.Source}
		}
		if HasAttitudeEvidence(ev) {
			return Verdict{Confirmed: true, Source: c.Source}
		}
		return Verdict{Source: c.Source}
	case "attitude":
		if HasAttitudeEvidence(ev) && pol.AttitudeEvidenceConfirmsSource {
			return Verdict{Confirmed: true, Source: c.Source}
		}
		return Verdict{Source: c.Source}
	default:
		return Verdict{Source: c.Source}
	}
}
