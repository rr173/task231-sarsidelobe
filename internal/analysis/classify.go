package analysis

import (
	"task231-sarsidelobe/internal/imaging"
	"task231-sarsidelobe/internal/model"
)

// Source classifications produced by the classifier.
const (
	SourceSidelobe = "sidelobe" // 旁瓣污染
	SourceAttitude = "attitude" // 姿态误差
	SourceScatter  = "scatter"  // 强散射（真实目标，不生成候选）
)

// ClassifyResult is the classifier output for one peak pair.
type ClassifyResult struct {
	Source         string
	IsSidelobeLike bool // offset 匹配（几何证据充分）
	RatioOK        bool
}

// Classify decides the contamination source for a peak pair given the
// acquisition geometry and the active calibration.
//
// Decision order:
//  1. Geometry match + amplitude band match  -> sidelobe (污染候选)
//  2. Geometry mismatch but attitude error   -> attitude (污染候选)
//  3. Otherwise                              -> scatter (真实目标，不生成候选)
func Classify(rep CorrelationReport, params model.ImagingParams, cal imaging.Calibration) ClassifyResult {
	offsetOK := rep.OffsetMatched
	ratioOK := rep.RatioOK
	if offsetOK && ratioOK {
		return ClassifyResult{Source: SourceSidelobe, IsSidelobeLike: true, RatioOK: true}
	}
	if !offsetOK && imaging.AttitudeSuspicious(params.AttitudeErrDeg) {
		return ClassifyResult{Source: SourceAttitude, IsSidelobeLike: false, RatioOK: ratioOK}
	}
	return ClassifyResult{Source: SourceScatter, IsSidelobeLike: false, RatioOK: ratioOK}
}
