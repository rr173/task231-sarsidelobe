package analysis

import (
	"math"

	"task231-sarsidelobe/internal/imaging"
)

// TheoreticalLobeProfile builds the theoretical azimuth response of a strong
// point target: the main lobe at offset 0 with the first sidelobe at
// ±1.5·ρ_a. Intensity is expressed in dB relative to the main-lobe peak.
func TheoreticalLobeProfile(firstLobeDB float64) []ProfilePoint {
	// Sample offsets in units of ρ_a: -3..3 with step 0.5 covers the main
	// lobe (0) and the first sidelobes (±1.5) plus the first nulls (±1).
	var out []ProfilePoint
	for x := -3.0; x <= 3.0+1e-9; x += 0.5 {
		// Normalised sinc power envelope at offset x (in ρ_a units).
		s := imaging.SincEnvelope(x / 1.5)
		var db float64
		if s < 1e-12 {
			db = -40
		} else {
			db = 10 * math.Log10(s)
		}
		out = append(out, ProfilePoint{OffsetUnits: x, IntensityDB: db})
	}
	return out
}

// ProfilePoint is one sample of an azimuth response profile.
type ProfilePoint struct {
	OffsetUnits float64 `json:"offset_units"`
	IntensityDB float64 `json:"intensity_db"`
}

// NormalizeToMain converts a profile so the main lobe is the given peak
// intensity: theoretical dB values are relative, so we simply reference them
// against the actual main-peak intensity supplied by the caller.
func NormalizeToMain(profile []ProfilePoint, mainPeakDB float64) []ProfilePoint {
	out := make([]ProfilePoint, len(profile))
	for i, p := range profile {
		out[i] = ProfilePoint{OffsetUnits: p.OffsetUnits, IntensityDB: mainPeakDB + p.IntensityDB}
	}
	return out
}
