package peak

import (
	"testing"

	"task231-sarsidelobe/internal/model"
)

func validPeak(azimuth, intensity float64) model.PeakRegion {
	return model.PeakRegion{
		RangeStart: 10, RangeEnd: 20, AzimuthStart: int(azimuth - 2), AzimuthEnd: int(azimuth + 2),
		PeakAzimuth: int(azimuth), PeakIntensityDB: intensity, Status: model.PeakRaw,
	}
}

func TestDeduplicateFiltersExistingAndIncomingRegions(t *testing.T) {
	a := validPeak(100, 40)
	b := validPeak(110, 30)
	out, err := Deduplicate([]model.PeakRegion{a, a, b}, map[string]bool{HashRegion(&b): true})
	if err != nil {
		t.Fatalf("Deduplicate() error = %v", err)
	}
	if len(out) != 1 || out[0].PeakAzimuth != a.PeakAzimuth {
		t.Fatalf("Deduplicate() = %+v, want only the new region", out)
	}
	if out[0].RegionHash == "" || out[0].Status != model.PeakRaw {
		t.Fatalf("deduplicated region missing normalized fields: %+v", out[0])
	}
}

func TestStrongScatterCandidatesExcludeSealedRegions(t *testing.T) {
	main := validPeak(100, 45)
	main.ID = 1
	weak := validPeak(106, 32)
	weak.ID = 2
	sealed := validPeak(112, 20)
	sealed.ID = 3
	sealed.Status = model.PeakExcluded
	pairs := StrongScatterCandidates([]model.PeakRegion{weak, sealed, main})
	if len(pairs) != 1 || pairs[0].Main.ID != main.ID || pairs[0].Cand.ID != weak.ID {
		t.Fatalf("StrongScatterCandidates() = %+v, want one active pair", pairs)
	}
}
