// Package httpapi exposes the REST surface for the SAR sidelobe diagnosis
// service. All routes are prefixed with /api and use the standard library
// mux with {id} path wildcards (Go 1.22+).
package httpapi

import (
	"net/http"

	"task231-sarsidelobe/internal/service"
	"task231-sarsidelobe/internal/store"
)

// Server holds the HTTP dependencies.
type Server struct {
	svc   *service.Service
	store *store.Store
	addr  string
}

// New builds an HTTP server bound to addr.
func New(svc *service.Service, st *store.Store, addr string) *Server {
	return &Server{svc: svc, store: st, addr: addr}
}

// Handler returns the configured mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// 自检与健康
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("POST /api/selftest", s.handleSelfTest)
	// 批次
	mux.HandleFunc("GET /api/batches", s.handleListBatches)
	mux.HandleFunc("POST /api/batches", s.handleCreateBatch)
	mux.HandleFunc("GET /api/batches/{id}", s.handleGetBatch)
	mux.HandleFunc("POST /api/batches/{id}/submit", s.handleSubmitBatch)
	mux.HandleFunc("POST /api/batches/{id}/review", s.handleReviewBatch)
	mux.HandleFunc("POST /api/batches/{id}/confirm", s.handleConfirmBatch)
	mux.HandleFunc("POST /api/batches/{id}/archive", s.handleArchiveBatch)
	// 成像参数
	mux.HandleFunc("PUT /api/batches/{id}/imaging-params", s.handlePutImagingParams)
	mux.HandleFunc("GET /api/batches/{id}/imaging-params", s.handleGetImagingParams)
	// 校准版本
	mux.HandleFunc("GET /api/calibrations", s.handleListCalibrations)
	mux.HandleFunc("POST /api/calibrations", s.handleCreateCalibration)
	mux.HandleFunc("POST /api/calibrations/{id}/activate", s.handleActivateCalibration)
	// 峰值区域
	mux.HandleFunc("POST /api/batches/{id}/peaks", s.handleRegisterPeaks)
	mux.HandleFunc("GET /api/batches/{id}/peaks", s.handleListPeaks)
	mux.HandleFunc("GET /api/peaks/{id}", s.handleGetPeak)
	mux.HandleFunc("POST /api/peaks/{id}/scatter", s.handleMarkScatter)
	mux.HandleFunc("POST /api/peaks/{id}/exclude", s.handleExcludePeak)
	// 分析
	mux.HandleFunc("POST /api/batches/{id}/analyze", s.handleAnalyze)
	mux.HandleFunc("GET /api/batches/{id}/candidates", s.handleListCandidates)
	mux.HandleFunc("GET /api/candidates/{id}", s.handleGetCandidate)
	// 复核
	mux.HandleFunc("POST /api/candidates/{id}/evidence", s.handleAddEvidence)
	mux.HandleFunc("POST /api/candidates/{id}/insufficient", s.handleInsufficient)
	mux.HandleFunc("POST /api/candidates/{id}/confirm", s.handleConfirmCandidate)
	mux.HandleFunc("POST /api/candidates/{id}/reject", s.handleRejectCandidate)
	// 快照
	mux.HandleFunc("POST /api/batches/{id}/snapshots", s.handlePublishSnapshot)
	mux.HandleFunc("GET /api/batches/{id}/snapshots", s.handleListSnapshots)
	mux.HandleFunc("GET /api/snapshots/{id}", s.handleGetSnapshot)
	mux.HandleFunc("POST /api/snapshots/{id}/supersede", s.handleSupersedeSnapshot)
	return mux
}

// ListenAndServe starts the long-running server.
func (s *Server) ListenAndServe() error {
	return http.ListenAndServe(s.addr, s.Handler())
}
