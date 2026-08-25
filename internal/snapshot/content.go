// Package snapshot freezes diagnosis outcomes into immutable published
// snapshots. A snapshot is built from the batch's final candidate verdicts,
// the acquisition geometry and the active calibration; once published it can
// only be superseded by a newer version.
package snapshot

import (
	"encoding/json"

	"task231-sarsidelobe/internal/imaging"
	"task231-sarsidelobe/internal/model"
)

// Content is the JSON payload frozen into a snapshot.
type Content struct {
	BatchCode       string           `json:"batch_code"`
	Geometry        imaging.Geometry `json:"geometry"`
	CalibrationDB   float64          `json:"calibration_first_lobe_db"`
	Sources         []SourceSummary  `json:"sources"`
	FinalCandidates int              `json:"final_candidates"`
	Confirmed       int              `json:"confirmed"`
	Rejected        int              `json:"rejected"`
	Insufficient    int              `json:"insufficient"`
}

// SourceSummary is one candidate's frozen verdict.
type SourceSummary struct {
	Source      string  `json:"source"`
	OffsetUnits float64 `json:"offset_units"`
	RatioDB     float64 `json:"intensity_ratio_db"`
	Response    float64 `json:"response_score"`
	Status      string  `json:"status"`
}

// Build renders the frozen content for a batch. Batch code and geometry come
// from the caller; candidates are summarised with their current verdicts.
func Build(batchCode string, geom imaging.Geometry, cal imaging.Calibration, candidates []model.Candidate) (string, error) {
	c := Content{
		BatchCode:     batchCode,
		Geometry:      geom,
		CalibrationDB: cal.FirstLobeDB,
	}
	for _, cand := range candidates {
		c.Sources = append(c.Sources, SourceSummary{
			Source:      cand.Source,
			OffsetUnits: cand.OffsetUnits,
			RatioDB:     cand.IntensityRatioDB,
			Response:    cand.ResponseScore,
			Status:      cand.Status,
		})
		switch cand.Status {
		case model.CandConfirmed:
			c.Confirmed++
		case model.CandRejected:
			c.Rejected++
		case model.CandInsufficient:
			c.Insufficient++
		}
	}
	c.FinalCandidates = len(candidates)
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CountsByStatus aggregates candidate statuses for the snapshot summary.
func CountsByStatus(candidates []model.Candidate) (confirmed, rejected, insufficient int) {
	for _, c := range candidates {
		switch c.Status {
		case model.CandConfirmed:
			confirmed++
		case model.CandRejected:
			rejected++
		case model.CandInsufficient:
			insufficient++
		}
	}
	return
}
