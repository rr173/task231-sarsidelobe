package peak

import (
	"sort"

	"task231-sarsidelobe/internal/model"
)

// Deduplicate filters out regions whose content hash already exists either in
// the incoming batch or in the store's existing set. Registration is
// idempotent: re-uploading the same regions is a no-op instead of an error.
func Deduplicate(in []model.PeakRegion, existing map[string]bool) ([]model.PeakRegion, error) {
	seen := make(map[string]bool, len(existing))
	for h := range existing {
		seen[h] = true
	}
	out := make([]model.PeakRegion, 0, len(in))
	for _, r := range in {
		r.Status = model.PeakRaw
		r.BatchID = 0 // filled by caller
		if err := ValidateRegion(&r); err != nil {
			return nil, err
		}
		h := HashRegion(&r)
		if seen[h] {
			continue
		}
		seen[h] = true
		r.RegionHash = h
		out = append(out, r)
	}
	return out, nil
}

// Pair is an ordered (main, candidate) peak pair used by analysis.
type Pair struct {
	Main model.PeakRegion
	Cand model.PeakRegion
}

// StrongScatterCandidates returns the ordered list of peak pairs that could
// be a strong-scatterer/sidelobe pair: both peaks are unresolved (raw or
// candidate), the suspicious peak is weaker than the main peak, and the
// azimuth offset is positive (sidelobes appear on both sides, so the
// absolute offset is used downstream). The list is sorted by main peak id
// then candidate id for deterministic output.
func StrongScatterCandidates(regions []model.PeakRegion) []Pair {
	var active []model.PeakRegion
	for _, r := range regions {
		if r.Status == model.PeakRaw || r.Status == model.PeakCandidate {
			active = append(active, r)
		}
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].PeakIntensityDB != active[j].PeakIntensityDB {
			return active[i].PeakIntensityDB > active[j].PeakIntensityDB
		}
		return active[i].ID < active[j].ID
	})
	var out []Pair
	for i := 0; i < len(active); i++ {
		for j := 0; j < len(active); j++ {
			if i == j {
				continue
			}
			// The suspicious peak must be weaker than the main peak.
			if active[j].PeakIntensityDB >= active[i].PeakIntensityDB {
				continue
			}
			out = append(out, Pair{Main: active[i], Cand: active[j]})
		}
	}
	return out
}
