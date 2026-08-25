package imaging

import (
	"math"

	"task231-sarsidelobe/internal/model"
)

// Geometry is a precomputed set of sidelobe geometry facts for one batch.
type Geometry struct {
	AzimuthResolutionM float64 `json:"azimuth_resolution_m"`
	FirstLobeSpacingM  float64 `json:"first_lobe_spacing_m"`
}

// Compute derives the geometry facts from validated imaging parameters.
func Compute(p model.ImagingParams) Geometry {
	return Geometry{
		AzimuthResolutionM: AzimuthResolutionM(p),
		FirstLobeSpacingM:  FirstLobeSpacingM(p),
	}
}

// MatchesLobeOffset reports whether the azimuth offset between two peaks (in
// metres) coincides with the expected n-th sidelobe position within the
// given relative tolerance. The offset is normalised by ρ_a; an integer
// multiple of 1.5 units (with the tolerance applied in units of ρ_a) is
// accepted.
func MatchesLobeOffset(offsetM float64, g Geometry, tolerance float64) (units float64, ok bool) {
	rho := g.AzimuthResolutionM
	if rho <= 0 {
		return 0, false
	}
	units = math.Abs(offsetM) / rho
	// Expected sidelobe positions are at 1.5·n·ρ_a. Normalise to "lobe
	// units" of 1.5·ρ_a and test against the nearest integer.
	lobeUnits := units / 1.5
	nearest := math.Round(lobeUnits)
	if nearest < 1 {
		return units, false
	}
	delta := math.Abs(lobeUnits - nearest)
	if delta <= tolerance {
		return units, true
	}
	return units, false
}

// IntensityRatioOK reports whether the measured peak-intensity difference
// (main minus suspicious, in dB, so positive means the suspicious peak is
// weaker) falls inside the calibration band expected for a genuine sidelobe.
func IntensityRatioOK(ratioDB, minDB, maxDB float64) bool {
	return ratioDB >= minDB && ratioDB <= maxDB
}

// TheoreticalSidelobeRatioDB returns the calibration-consistent reference
// first-lobe ratio in dB (normally 13.26, overridable by calibration).
func TheoreticalSidelobeRatioDB(calFirstLobeDB float64) float64 {
	if calFirstLobeDB <= 0 {
		return LobePeakRatioDB(1)
	}
	return calFirstLobeDB
}

// AttitudeSuspicious reports whether the recorded attitude error exceeds the
// 0.5° threshold used to flag attitude-induced contamination.
func AttitudeSuspicious(attitudeErrDeg float64) bool {
	return attitudeErrDeg > 0.5
}
