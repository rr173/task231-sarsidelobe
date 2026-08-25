package review

import (
	"testing"

	"task231-sarsidelobe/internal/model"
)

func TestDecideUsesGeometryAndEvidence(t *testing.T) {
	if got := Decide(model.Candidate{Source: "sidelobe", ResponseScore: 0.8}, nil, DefaultPolicy()); !got.Confirmed {
		t.Fatal("strong sidelobe geometry should confirm the candidate")
	}
	attitude := model.Candidate{Source: "attitude", ResponseScore: 0.1}
	ev := []model.Evidence{{Kind: "attitude_calibration"}}
	if got := Decide(attitude, ev, DefaultPolicy()); !got.Confirmed || got.Source != "attitude" {
		t.Fatalf("attitude evidence verdict = %+v, want confirmed attitude", got)
	}
	if got := Decide(attitude, nil, DefaultPolicy()); got.Confirmed || got.Rejected {
		t.Fatalf("missing attitude evidence verdict = %+v, want insufficient", got)
	}
}

func TestNeedsReviewRequiresUnconfirmedCandidate(t *testing.T) {
	if NeedsReview(nil) {
		t.Fatal("empty candidate list should not need review")
	}
	if !NeedsReview([]model.Candidate{{Status: model.CandGenerated}}) {
		t.Fatal("generated candidate should need review")
	}
	if NeedsReview([]model.Candidate{{Status: model.CandConfirmed}}) {
		t.Fatal("confirmed candidates should not need review")
	}
}
