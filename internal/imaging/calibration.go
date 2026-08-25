package imaging

import "task231-sarsidelobe/internal/model"

// Calibration is the domain value object for the active calibration version.
// It carries the first-lobe attenuation and the acceptance bands used by the
// classifier.
type Calibration struct {
	FirstLobeDB     float64
	OffsetTolerance float64
	RatioMinDB      float64
	RatioMaxDB      float64
}

// DefaultCalibration returns the built-in calibration used when no version
// has been activated yet. The first-lobe attenuation matches the theoretical
// sinc first sidelobe (~ -13.26 dB).
func DefaultCalibration() Calibration {
	return Calibration{
		FirstLobeDB:     13.26,
		OffsetTolerance: 0.25,
		RatioMinDB:      6.0,
		RatioMaxDB:      20.0,
	}
}

// FromVersion converts a stored calibration version into the domain object.
func FromVersion(v *model.CalibrationVersion) Calibration {
	if v == nil {
		return DefaultCalibration()
	}
	return Calibration{
		FirstLobeDB:     v.FirstLobeDB,
		OffsetTolerance: v.OffsetTolerance,
		RatioMinDB:      v.RatioMinDB,
		RatioMaxDB:      v.RatioMaxDB,
	}
}
