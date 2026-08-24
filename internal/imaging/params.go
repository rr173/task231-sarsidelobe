// Package imaging validates SAR acquisition parameters and computes the
// geometric quantities that drive sidelobe diagnosis: azimuth resolution,
// expected first-sidelobe spacing and the sinc response model.
package imaging

import (
	"math"
	"strings"

	"task231-sarsidelobe/internal/model"
)

// Valid polarization modes for SAR acquisitions.
var ValidPolarizations = map[string]bool{
	"HH": true, "VV": true, "HV": true, "VH": true,
}

// Valid orbit directions.
var ValidOrbits = map[string]bool{
	"ascending": true, "descending": true,
}

// ValidateParams checks every acquisition parameter for physical sanity.
// The polarization mode is mandatory (ErrPolarizationMissing) and geometry
// must be positive with a plausible aperture ratio.
func ValidateParams(p *model.ImagingParams) error {
	pol := strings.ToUpper(strings.TrimSpace(p.Polarization))
	if pol == "" {
		return model.ErrPolarizationMissing
	}
	if !ValidPolarizations[pol] {
		return model.ErrBadRequest
	}
	if p.OrbitDirection == "" {
		p.OrbitDirection = "descending"
	}
	if !ValidOrbits[p.OrbitDirection] {
		return model.ErrBadRequest
	}
	if p.WavelengthM <= 0 || p.SlantRangeM <= 0 || p.ApertureLenM <= 0 {
		return model.ErrBadRequest
	}
	if p.LookAngleDeg < 0 || p.LookAngleDeg > 90 {
		return model.ErrBadRequest
	}
	if p.AttitudeErrDeg < 0 || p.AttitudeErrDeg > 5 {
		return model.ErrBadRequest
	}
	// The azimuth resolution must be a positive fraction of the range; a
	// completely degenerate aperture (resolution >= range) is rejected.
	if AzimuthResolutionM(*p) >= p.SlantRangeM {
		return model.ErrBadRequest
	}
	return nil
}

// AzimuthResolutionM returns the theoretical azimuth resolution
// ρ_a = λ·R / (2·L) in metres.
func AzimuthResolutionM(p model.ImagingParams) float64 {
	return p.WavelengthM * p.SlantRangeM / (2.0 * p.ApertureLenM)
}

// FirstLobeSpacingM returns the expected azimuth distance from the main lobe
// to the first sidelobe of a sinc point-target response, which is
// 1.5·ρ_a.
func FirstLobeSpacingM(p model.ImagingParams) float64 {
	return 1.5 * AzimuthResolutionM(p)
}

// SincEnvelope computes the normalised sinc power envelope
// |sinc(x)|² = (sin(πx)/(πx))² at azimuth offset x measured in units of the
// first-lobe spacing (x=1 is the first sidelobe).
func SincEnvelope(x float64) float64 {
	if math.Abs(x) < 1e-9 {
		return 1.0
	}
	s := math.Sin(math.Pi * x) / (math.Pi * x)
	return s * s
}

// LobePeakRatioDB returns the theoretical peak-power ratio (dB) between the
// main lobe and the n-th sidelobe (n=1 is the first sidelobe, ~ -13.26 dB).
func LobePeakRatioDB(n int) float64 {
	if n < 1 {
		return 0
	}
	x := float64(n) + 0.5 // zeros at integers, peaks at n+0.5
	return 10 * math.Log10(SincEnvelope(x))
}
