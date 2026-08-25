package service

import (
	"fmt"

	"task231-sarsidelobe/internal/analysis"
	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/review"
)

// AnalyzeBatch runs the sidelobe diagnosis for a batch: it loads the imaging
// parameters and active calibration, pairs strong-scatterer candidates,
// computes per-pair evidence in parallel and persists the resulting
// contamination candidates. The batch must have parameters and at least one
// peak region; analysis per batch is serialised (ErrAnalyzeLocked while
// running).
func (s *Service) AnalyzeBatch(batchID int64) ([]model.Candidate, error) {
	// Per-batch analysis lock: only one run at a time per batch.
	s.analyzeMu.Lock()
	if s.analyzing[batchID] {
		s.analyzeMu.Unlock()
		return nil, model.ErrAnalyzeLocked
	}
	s.analyzing[batchID] = true
	s.analyzeMu.Unlock()
	defer func() {
		s.analyzeMu.Lock()
		delete(s.analyzing, batchID)
		s.analyzeMu.Unlock()
	}()

	b, err := s.Store.GetBatch(batchID)
	if err != nil {
		return nil, err
	}
	if b.Status == model.BatchArchived {
		return nil, model.ErrArchivedMutation
	}
	params, err := s.Store.GetImagingParams(batchID)
	if err != nil {
		return nil, model.ErrNoParams
	}
	cal, err := s.activeCalibration()
	if err != nil {
		return nil, err
	}
	regions, err := s.Store.ListPeakRegions(batchID)
	if err != nil {
		return nil, err
	}
	if len(regions) == 0 {
		return nil, model.ErrNoPeaks
	}
	run, err := s.Store.StartAnalysisRun(batchID)
	if err != nil {
		return nil, err
	}
	analyzer := analysis.New(*params, cal)
	cands := analyzer.Run(regions)
	if err := s.Store.InsertCandidates(cands); err != nil {
		_ = s.Store.FinishAnalysisRun(run.ID, 0, true)
		return nil, err
	}
	// Move the batch to needs_review when candidates need a human verdict.
	if review.NeedsReview(cands) && model.CanBatchTransition(b.Status, model.BatchReview) {
		_ = s.Store.UpdateBatchStatus(batchID, model.BatchReview)
	}
	if err := s.Store.FinishAnalysisRun(run.ID, len(cands), false); err != nil {
		return nil, err
	}
	return s.Store.ListCandidates(batchID, "")
}

// AnalyzeAndReview runs analysis and immediately applies the review policy to
// every generated candidate. It is used by the self-test to demonstrate the
// full "sample bright band -> sidelobe candidate -> confirmed -> snapshot"
// loop without manual HTTP steps.
func (s *Service) AnalyzeAndReview(batchID int64) ([]model.Candidate, error) {
	cands, err := s.AnalyzeBatch(batchID)
	if err != nil {
		return nil, err
	}
	for _, c := range cands {
		if _, err := s.reviewCandidate(c.ID, false, false); err != nil {
			return nil, fmt.Errorf("auto review candidate %d: %w", c.ID, err)
		}
	}
	return s.Store.ListCandidates(batchID, "")
}

// EnsureCalibrationActive is a convenience used by self-test/CLI to make a
// calibration version active without a separate HTTP round trip.
func (s *Service) EnsureCalibrationActive(name string, firstLobeDB float64) (*model.CalibrationVersion, error) {
	v, err := s.Store.CreateCalibration(name, firstLobeDB, 0.25, 6.0, 20.0)
	if err != nil {
		return nil, err
	}
	return s.Store.ActivateCalibration(v.ID)
}
