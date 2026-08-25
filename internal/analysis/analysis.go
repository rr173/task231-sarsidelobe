// Package analysis implements the core sidelobe contamination diagnosis:
// it pairs strong-scatterer candidates, compares azimuth offsets against the
// theoretical lobe spacing, correlates azimuth responses and classifies the
// contamination source. Pair analysis is parallel; candidate collection is
// serialised on the caller side.
package analysis

import (
	"sync"

	"task231-sarsidelobe/internal/imaging"
	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/peak"
)

// Analyzer runs the sidelobe diagnosis for one batch.
type Analyzer struct {
	Params model.ImagingParams
	Cal    imaging.Calibration
	Geom   imaging.Geometry
}

// New builds an Analyzer from acquisition parameters and the active
// calibration. The parameters must already have passed imaging.ValidateParams.
func New(params model.ImagingParams, cal imaging.Calibration) *Analyzer {
	return &Analyzer{
		Params: params,
		Cal:    cal,
		Geom:   imaging.Compute(params),
	}
}

// Correlate computes the per-pair evidence for a strong-scatterer / candidate
// pair.
func (a *Analyzer) Correlate(main, cand model.PeakRegion) CorrelationReport {
	units := AzimuthDeltaUnits(main.PeakAzimuth, cand.PeakAzimuth)
	offsetM := units * a.Geom.AzimuthResolutionM
	ratioDB := main.PeakIntensityDB - cand.PeakIntensityDB
	_, offsetMatched := imaging.MatchesLobeOffset(offsetM, a.Geom, a.Cal.OffsetTolerance)
	ratioOK := imaging.IntensityRatioOK(ratioDB, a.Cal.RatioMinDB, a.Cal.RatioMaxDB)
	return CorrelationReport{
		OffsetUnits:      units,
		AzimuthOffsetM:   offsetM,
		IntensityRatioDB: ratioDB,
		ResponseScore:    ResponseScore(ratioDB, imaging.TheoreticalSidelobeRatioDB(a.Cal.FirstLobeDB)),
		OffsetMatched:    offsetMatched,
		RatioOK:          ratioOK,
	}
}

// Run performs the diagnosis over all active peak pairs in parallel. It
// returns only candidates whose source is sidelobe or attitude contamination;
// pairs classified as genuine strong scatterers are not candidates.
func (a *Analyzer) Run(regions []model.PeakRegion) []model.Candidate {
	pairs := peak.StrongScatterCandidates(regions)
	if len(pairs) == 0 {
		return nil
	}
	results := make([]model.Candidate, 0, len(pairs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	worker := func(p peak.Pair) {
		defer wg.Done()
		rep := a.Correlate(p.Main, p.Cand)
		cls := Classify(rep, a.Params, a.Cal)
		if cls.Source == SourceScatter {
			return // 真实目标，不构成污染候选
		}
		c := model.Candidate{
			BatchID:          p.Main.BatchID,
			MainPeakID:       p.Main.ID,
			SidelobePeakID:   p.Cand.ID,
			AzimuthOffsetM:   rep.AzimuthOffsetM,
			OffsetUnits:      rep.OffsetUnits,
			IntensityRatioDB: rep.IntensityRatioDB,
			ResponseScore:    rep.ResponseScore,
			Source:           cls.Source,
			Status:           model.CandGenerated,
		}
		mu.Lock()
		results = append(results, c)
		mu.Unlock()
	}
	for _, p := range pairs {
		wg.Add(1)
		go worker(p)
	}
	wg.Wait()
	return results
}
