package httpapi

import (
	"net/http"
	"strconv"

	"task231-sarsidelobe/internal/model"
)

// ---- 自检与健康 ----

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	batches, err := s.store.ListBatches()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"batches": len(batches),
	})
}

func (s *Server) handleSelfTest(w http.ResponseWriter, _ *http.Request) {
	res, err := runSelfTest(s.svc)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---- 批次 ----

func (s *Server) handleListBatches(w http.ResponseWriter, _ *http.Request) {
	batches, err := s.store.ListBatches()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": batches})
}

func (s *Server) handleCreateBatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Sensor string `json:"sensor"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	b, err := s.svc.CreateBatch(in.Code, in.Name, in.Sensor)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	b, err := s.store.GetBatch(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleSubmitBatch(w http.ResponseWriter, r *http.Request) {
	s.batchAction(w, r, s.svc.SubmitBatch)
}

func (s *Server) handleReviewBatch(w http.ResponseWriter, r *http.Request) {
	s.batchAction(w, r, s.svc.RequestReview)
}

func (s *Server) handleConfirmBatch(w http.ResponseWriter, r *http.Request) {
	s.batchAction(w, r, s.svc.ConfirmBatch)
}

func (s *Server) handleArchiveBatch(w http.ResponseWriter, r *http.Request) {
	s.batchAction(w, r, s.svc.ArchiveBatch)
}

func (s *Server) batchAction(w http.ResponseWriter, r *http.Request, fn func(int64) (*model.Batch, error)) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	b, err := fn(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// ---- 成像参数 ----

func (s *Server) handlePutImagingParams(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	var p model.ImagingParams
	if err := readJSON(r, &p); err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	got, err := s.svc.RegisterImagingParams(id, &p)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) handleGetImagingParams(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	p, err := s.svc.GetImagingParams(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ---- 校准版本 ----

func (s *Server) handleListCalibrations(w http.ResponseWriter, _ *http.Request) {
	cal, err := s.svc.ListCalibrations()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": cal})
}

func (s *Server) handleCreateCalibration(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string  `json:"name"`
		FirstLobeDB float64 `json:"first_lobe_db"`
		OffsetTol   float64 `json:"offset_tolerance"`
		RatioMinDB  float64 `json:"ratio_min_db"`
		RatioMaxDB  float64 `json:"ratio_max_db"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	v, err := s.svc.CreateCalibration(in.Name, in.FirstLobeDB, in.OffsetTol, in.RatioMinDB, in.RatioMaxDB)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) handleActivateCalibration(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	v, err := s.svc.ActivateCalibration(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// ---- 峰值区域 ----

func (s *Server) handleRegisterPeaks(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	var in struct {
		Regions []model.PeakRegion `json:"regions"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	n, err := s.svc.RegisterPeaks(id, in.Regions)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"inserted": n})
}

func (s *Server) handleListPeaks(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	peaks, err := s.svc.ListPeaks(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": peaks})
}

func (s *Server) handleGetPeak(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	p, err := s.svc.GetPeak(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleMarkScatter(w http.ResponseWriter, r *http.Request) {
	s.peakAction(w, r, s.svc.MarkScatter)
}

func (s *Server) handleExcludePeak(w http.ResponseWriter, r *http.Request) {
	s.peakAction(w, r, s.svc.ExcludePeak)
}

func (s *Server) peakAction(w http.ResponseWriter, r *http.Request, fn func(int64) (*model.PeakRegion, error)) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	p, err := fn(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ---- 分析 ----

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	cands, err := s.svc.AnalyzeBatch(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": len(cands), "items": cands})
}

func (s *Server) handleListCandidates(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	status := r.URL.Query().Get("status")
	cands, err := s.svc.ListCandidates(id, status)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": cands})
}

func (s *Server) handleGetCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	c, err := s.svc.GetCandidate(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// ---- 复核 ----

func (s *Server) handleAddEvidence(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	var in struct {
		Kind string `json:"kind"`
		Note string `json:"note"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	e, err := s.svc.AddEvidence(id, in.Kind, in.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) handleInsufficient(w http.ResponseWriter, r *http.Request) {
	s.candidateAction(w, r, s.svc.MarkInsufficient)
}

func (s *Server) handleConfirmCandidate(w http.ResponseWriter, r *http.Request) {
	s.candidateAction(w, r, s.svc.ConfirmCandidate)
}

func (s *Server) handleRejectCandidate(w http.ResponseWriter, r *http.Request) {
	s.candidateAction(w, r, s.svc.RejectCandidate)
}

func (s *Server) candidateAction(w http.ResponseWriter, r *http.Request, fn func(int64) (*model.Candidate, error)) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	c, err := fn(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// ---- 快照 ----

func (s *Server) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	snap, err := s.svc.PublishSnapshot(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	snaps, err := s.svc.ListSnapshots(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": snaps})
}

func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	snap, err := s.svc.GetSnapshot(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleSupersedeSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeErr(w, model.ErrBadRequest)
		return
	}
	snap, err := s.svc.SupersedeSnapshot(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// ---- 辅助 ----

func pathID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, model.ToStatus(err), map[string]string{"error": err.Error()})
}
