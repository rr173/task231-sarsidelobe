// Package service orchestrates the SAR sidelobe diagnosis workflow: batch
// lifecycle, imaging parameter registration, peak ingestion, analysis runs,
// candidate review and snapshot publication. It enforces every state machine
// and invariant from the model layer.
package service

import (
	"fmt"
	"sync"

	"task231-sarsidelobe/internal/imaging"
	"task231-sarsidelobe/internal/model"
	"task231-sarsidelobe/internal/peak"
	"task231-sarsidelobe/internal/review"
	"task231-sarsidelobe/internal/snapshot"
	"task231-sarsidelobe/internal/store"
)

// Service wires the store and the domain packages into one orchestrator.
type Service struct {
	Store     *store.Store
	Freezer   *snapshot.Freezer
	Policy    review.Policy
	analyzeMu sync.Mutex
	analyzing map[int64]bool // per-batch analysis lock
	peakMu    sync.Mutex
	calMu     sync.Mutex
}

// New builds a Service over the given store.
func New(st *store.Store) *Service {
	return &Service{
		Store:     st,
		Freezer:   snapshot.NewFreezer(),
		Policy:    review.DefaultPolicy(),
		analyzing: map[int64]bool{},
	}
}

// CreateBatch registers a new imaging batch. Codes are unique.
func (s *Service) CreateBatch(code, name, sensor string) (*model.Batch, error) {
	if code == "" || name == "" {
		return nil, model.ErrBadRequest
	}
	return s.Store.CreateBatch(code, name, sensor)
}

// SubmitBatch moves a batch from receiving to pending_diagnosis.
func (s *Service) SubmitBatch(id int64) (*model.Batch, error) {
	b, err := s.Store.GetBatch(id)
	if err != nil {
		return nil, err
	}
	if !model.CanBatchTransition(b.Status, model.BatchPending) {
		return nil, model.ErrStateTransition
	}
	if err := s.Store.UpdateBatchStatus(id, model.BatchPending); err != nil {
		return nil, err
	}
	return s.Store.GetBatch(id)
}

// RequestReview marks a batch as needing engineer review (after analysis).
func (s *Service) RequestReview(id int64) (*model.Batch, error) {
	b, err := s.Store.GetBatch(id)
	if err != nil {
		return nil, err
	}
	if !model.CanBatchTransition(b.Status, model.BatchReview) {
		return nil, model.ErrStateTransition
	}
	if err := s.Store.UpdateBatchStatus(id, model.BatchReview); err != nil {
		return nil, err
	}
	return s.Store.GetBatch(id)
}

// ConfirmBatch confirms the diagnosis for a batch.
func (s *Service) ConfirmBatch(id int64) (*model.Batch, error) {
	b, err := s.Store.GetBatch(id)
	if err != nil {
		return nil, err
	}
	if !model.CanBatchTransition(b.Status, model.BatchConfirmed) {
		return nil, model.ErrStateTransition
	}
	if err := s.Store.UpdateBatchStatus(id, model.BatchConfirmed); err != nil {
		return nil, err
	}
	return s.Store.GetBatch(id)
}

// ArchiveBatch freezes a batch; after archival all mutations are rejected.
func (s *Service) ArchiveBatch(id int64) (*model.Batch, error) {
	b, err := s.Store.GetBatch(id)
	if err != nil {
		return nil, err
	}
	if !model.CanBatchTransition(b.Status, model.BatchArchived) {
		return nil, model.ErrStateTransition
	}
	if err := s.Store.UpdateBatchStatus(id, model.BatchArchived); err != nil {
		return nil, err
	}
	return s.Store.GetBatch(id)
}

// RegisterImagingParams validates and stores the acquisition geometry. The
// polarization mode is mandatory; archived batches reject any update.
func (s *Service) RegisterImagingParams(batchID int64, p *model.ImagingParams) (*model.ImagingParams, error) {
	b, err := s.Store.GetBatch(batchID)
	if err != nil {
		return nil, err
	}
	if !model.CanUpdateImagingParams(b.Status) {
		if b.Status == model.BatchArchived {
			return nil, model.ErrArchivedMutation
		}
		return nil, model.ErrStateTransition
	}
	if err := imaging.ValidateParams(p); err != nil {
		return nil, err
	}
	p.BatchID = batchID
	return s.Store.UpsertImagingParams(p)
}

// GetImagingParams returns the geometry for a batch.
func (s *Service) GetImagingParams(batchID int64) (*model.ImagingParams, error) {
	return s.Store.GetImagingParams(batchID)
}

// CreateCalibration adds a calibration version (auto version number).
func (s *Service) CreateCalibration(name string, firstLobeDB, offsetTol, ratioMinDB, ratioMaxDB float64) (*model.CalibrationVersion, error) {
	s.calMu.Lock()
	defer s.calMu.Unlock()
	if name == "" || firstLobeDB <= 0 || offsetTol <= 0 || ratioMinDB <= 0 || ratioMaxDB <= ratioMinDB {
		return nil, model.ErrBadRequest
	}
	return s.Store.CreateCalibration(name, firstLobeDB, offsetTol, ratioMinDB, ratioMaxDB)
}

// ListCalibrations returns all calibration versions.
func (s *Service) ListCalibrations() ([]model.CalibrationVersion, error) {
	return s.Store.ListCalibrations()
}

// ActivateCalibration makes a version the active one.
func (s *Service) ActivateCalibration(id int64) (*model.CalibrationVersion, error) {
	return s.Store.ActivateCalibration(id)
}

// RegisterPeaks ingests peak regions idempotently: regions whose content hash
// already exists are skipped, all remaining regions are inserted in one
// transaction. Any invalid region aborts the whole batch.
func (s *Service) RegisterPeaks(batchID int64, in []model.PeakRegion) (inserted int, err error) {
	s.peakMu.Lock()
	defer s.peakMu.Unlock()
	b, err := s.Store.GetBatch(batchID)
	if err != nil {
		return 0, err
	}
	if b.Status == model.BatchArchived {
		return 0, model.ErrArchivedMutation
	}
	// Once analysis has entered review the peak set is frozen: appending new
	// regions would change the data the reviewer is judging.
	if !model.CanRegisterAnalysisInput(b.Status) {
		return 0, model.ErrStateTransition
	}
	existing, err := s.Store.ExistingRegionHashes(batchID)
	if err != nil {
		return 0, err
	}
	clean, err := peak.Deduplicate(in, existing)
	if err != nil {
		return 0, err
	}
	for i := range clean {
		clean[i].BatchID = batchID
	}
	if len(clean) == 0 {
		return 0, nil
	}
	if err := s.Store.InsertPeakRegions(batchID, clean); err != nil {
		return 0, err
	}
	return len(clean), nil
}

// ListPeaks returns all regions of a batch.
func (s *Service) ListPeaks(batchID int64) ([]model.PeakRegion, error) {
	return s.Store.ListPeakRegions(batchID)
}

// GetPeak returns one region.
func (s *Service) GetPeak(id int64) (*model.PeakRegion, error) {
	return s.Store.GetPeakRegion(id)
}

// MarkScatter classifies a raw/candidate region as a strong scatterer.
func (s *Service) MarkScatter(id int64) (*model.PeakRegion, error) {
	r, err := s.Store.GetPeakRegion(id)
	if err != nil {
		return nil, err
	}
	if !model.CanPeakTransition(r.Status, model.PeakScatter) {
		return nil, model.ErrStateTransition
	}
	if err := s.Store.UpdatePeakStatus(id, model.PeakScatter); err != nil {
		return nil, err
	}
	return s.Store.GetPeakRegion(id)
}

// ExcludePeak removes a region from further analysis (sealed).
func (s *Service) ExcludePeak(id int64) (*model.PeakRegion, error) {
	r, err := s.Store.GetPeakRegion(id)
	if err != nil {
		return nil, err
	}
	if !model.CanPeakTransition(r.Status, model.PeakExcluded) {
		return nil, model.ErrStateTransition
	}
	if err := s.Store.UpdatePeakStatus(id, model.PeakExcluded); err != nil {
		return nil, err
	}
	return s.Store.GetPeakRegion(id)
}

// ListCandidates returns candidates for a batch, optionally by status.
func (s *Service) ListCandidates(batchID int64, status string) ([]model.Candidate, error) {
	return s.Store.ListCandidates(batchID, status)
}

// GetCandidate returns one candidate.
func (s *Service) GetCandidate(id int64) (*model.Candidate, error) {
	return s.Store.GetCandidate(id)
}

// AddEvidence attaches an evidence record to a candidate.
func (s *Service) AddEvidence(candidateID int64, kind, note string) (*model.Evidence, error) {
	if err := review.ValidateEvidenceKind(kind); err != nil {
		return nil, err
	}
	if _, err := s.Store.GetCandidate(candidateID); err != nil {
		return nil, err
	}
	return s.Store.AddEvidence(candidateID, kind, note)
}

// reviewCandidate applies the review policy and resolves the source peaks.
func (s *Service) reviewCandidate(id int64, forceConfirmed, forceRejected bool) (*model.Candidate, error) {
	c, err := s.Store.GetCandidate(id)
	if err != nil {
		return nil, err
	}
	b, err := s.Store.GetBatch(c.BatchID)
	if err != nil {
		return nil, err
	}
	if b.Status == model.BatchArchived {
		return nil, model.ErrArchivedMutation
	}
	ev, err := s.Store.ListEvidence(id)
	if err != nil {
		return nil, err
	}
	var next string
	var confirmed bool
	switch {
	case forceConfirmed:
		if !model.CanCandidateTransition(c.Status, model.CandConfirmed) {
			return nil, model.ErrStateTransition
		}
		next, confirmed = model.CandConfirmed, true
	case forceRejected:
		if !model.CanCandidateTransition(c.Status, model.CandRejected) {
			return nil, model.ErrStateTransition
		}
		next = model.CandRejected
	default:
		v := review.Decide(*c, ev, s.Policy)
		if v.Confirmed {
			if !model.CanCandidateTransition(c.Status, model.CandConfirmed) {
				return nil, model.ErrStateTransition
			}
			next, confirmed = model.CandConfirmed, true
		} else if v.Rejected {
			if !model.CanCandidateTransition(c.Status, model.CandRejected) {
				return nil, model.ErrStateTransition
			}
			next = model.CandRejected
		} else {
			if !model.CanCandidateTransition(c.Status, model.CandInsufficient) {
				return nil, model.ErrStateTransition
			}
			next = model.CandInsufficient
		}
	}
	if err := s.Store.ResolveCandidate(c, next, confirmed); err != nil {
		return nil, err
	}
	return s.Store.GetCandidate(id)
}

// ConfirmCandidate confirms a candidate as real contamination.
func (s *Service) ConfirmCandidate(id int64) (*model.Candidate, error) {
	return s.reviewCandidate(id, true, false)
}

// RejectCandidate rejects a candidate (the suspicious peak is not sidelobe
// contamination).
func (s *Service) RejectCandidate(id int64) (*model.Candidate, error) {
	return s.reviewCandidate(id, false, true)
}

// MarkInsufficient marks a candidate as lacking evidence.
func (s *Service) MarkInsufficient(id int64) (*model.Candidate, error) {
	c, err := s.Store.GetCandidate(id)
	if err != nil {
		return nil, err
	}
	if !model.CanCandidateTransition(c.Status, model.CandInsufficient) {
		return nil, model.ErrStateTransition
	}
	if err := s.Store.UpdateCandidateStatus(id, model.CandInsufficient); err != nil {
		return nil, err
	}
	return s.Store.GetCandidate(id)
}

// PublishSnapshot freezes the current diagnosis into a published snapshot.
func (s *Service) PublishSnapshot(batchID int64) (*model.Snapshot, error) {
	b, err := s.Store.GetBatch(batchID)
	if err != nil {
		return nil, err
	}
	if err := s.Freezer.ValidatePublishBatch(b.Status); err != nil {
		return nil, err
	}
	if err := s.Freezer.ValidateCreate(b.Status); err != nil {
		return nil, err
	}
	params, err := s.Store.GetImagingParams(batchID)
	if err != nil {
		return nil, model.ErrNoParams
	}
	var cal imaging.Calibration
	if params.CalibrationID != 0 {
		v, err := s.Store.GetCalibration(params.CalibrationID)
		if err != nil {
			return nil, err
		}
		cal = imaging.FromVersion(v)
	} else {
		cal, err = s.activeCalibration()
		if err != nil {
			return nil, err
		}
	}
	geom := imaging.Compute(*params)
	cands, err := s.Store.ListCandidates(batchID, "")
	if err != nil {
		return nil, err
	}
	content, err := snapshot.Build(b.Code, geom, cal, cands)
	if err != nil {
		return nil, fmt.Errorf("build snapshot content: %w", err)
	}
	snap, err := s.Store.CreateSnapshot(batchID, content)
	if err != nil {
		return nil, err
	}
	if err := s.Freezer.ValidatePublish(b.Status, snap); err != nil {
		return nil, err
	}
	if err := s.Store.UpdateSnapshotStatus(snap.ID, model.SnapPublished); err != nil {
		return nil, err
	}
	return s.Store.GetSnapshot(snap.ID)
}

// SupersedeSnapshot replaces a published snapshot with a new draft-free
// version (a newer snapshot supersedes the old one).
func (s *Service) SupersedeSnapshot(id int64) (*model.Snapshot, error) {
	snap, err := s.Store.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	b, err := s.Store.GetBatch(snap.BatchID)
	if err != nil {
		return nil, err
	}
	if err := s.Freezer.ValidateReplacement(b.Status, snap); err != nil {
		return nil, err
	}
	return s.Store.ReplaceSnapshot(id, snap.Content)
}

// ListSnapshots returns all snapshots of a batch.
func (s *Service) ListSnapshots(batchID int64) ([]model.Snapshot, error) {
	return s.Store.ListSnapshots(batchID)
}

// GetSnapshot returns one snapshot.
func (s *Service) GetSnapshot(id int64) (*model.Snapshot, error) {
	return s.Store.GetSnapshot(id)
}

// activeCalibration returns the active calibration version or the default.
func (s *Service) activeCalibration() (imaging.Calibration, error) {
	v, err := s.Store.GetActiveCalibration()
	if err != nil {
		return imaging.Calibration{}, err
	}
	return imaging.FromVersion(v), nil
}
