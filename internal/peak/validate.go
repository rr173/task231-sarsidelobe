// Package peak validates and identifies SAR peak regions. Each region is a
// range/azimuth bounding box around a local maximum; identity is derived from
// a content hash so duplicate registration is impossible.
package peak

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"

	"task231-sarsidelobe/internal/model"
)

// MaxCoordinate is the inclusive upper bound for range/azimuth indices.
const MaxCoordinate = 1 << 20

// ValidateRegion checks a peak region's coordinates and intensity.
// Rejects inverted boxes, out-of-range indices and non-finite intensities.
func ValidateRegion(b *model.PeakRegion) error {
	if b.RangeStart < 0 || b.RangeEnd < 0 || b.AzimuthStart < 0 || b.AzimuthEnd < 0 {
		return model.ErrCoordinateRange
	}
	if b.RangeEnd >= MaxCoordinate || b.AzimuthEnd >= MaxCoordinate {
		return model.ErrCoordinateRange
	}
	if b.RangeStart >= b.RangeEnd || b.AzimuthStart >= b.AzimuthEnd {
		return model.ErrCoordinateRange
	}
	if b.PeakAzimuth < b.AzimuthStart || b.PeakAzimuth >= b.AzimuthEnd {
		return model.ErrCoordinateRange
	}
	if math.IsNaN(b.PeakIntensityDB) || math.IsInf(b.PeakIntensityDB, 0) {
		return model.ErrBadRequest
	}
	return nil
}

// HashRegion derives a stable content hash for a region within a batch. Two
// regions with identical geometry and intensity hash identically; this is the
// idempotency key used by registration.
func HashRegion(b *model.PeakRegion) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%d|%d|%d|%d|%.6f",
		b.RangeStart, b.RangeEnd, b.AzimuthStart, b.AzimuthEnd, b.PeakAzimuth, b.PeakIntensityDB)
	return hex.EncodeToString(h.Sum(nil))
}

// ClassifyTo resolves a raw region into its final conclusion. The returned
// (status, ok) pair is used by the review layer when a candidate is
// confirmed / rejected / excluded.
func ClassifyTo(current, target string) (string, bool) {
	if !model.CanPeakTransition(current, target) {
		return "", false
	}
	return target, true
}
