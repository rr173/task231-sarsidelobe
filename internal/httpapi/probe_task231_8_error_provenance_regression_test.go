package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"task231-sarsidelobe/internal/service"
	"task231-sarsidelobe/internal/store"
)

func TestMissingCandidatePreservesNotFoundHTTPStatus(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/probe.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer st.Close()
	srv := New(service.New(st), st, "")
	req := httptest.NewRequest(http.MethodPost, "/api/candidates/999/evidence", bytes.NewBufferString(`{"kind":"operator_note","note":"missing"}`))
	res := httptest.NewRecorder()
	srv.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("missing candidate status = %d, want %d; body=%s", res.Code, http.StatusNotFound, res.Body.String())
	}
}
