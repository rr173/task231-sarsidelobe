package analysis

import (
	"math"
)

// ResponseScore measures how close the measured intensity ratio between the
// suspicious peak and the main peak is to the theoretical first-lobe
// attenuation. A ratio exactly equal to the calibration value scores 1.0;
// deviations beyond ±10 dB score 0.
func ResponseScore(ratioDB, firstLobeDB float64) float64 {
	dev := math.Abs(ratioDB - firstLobeDB)
	score := 1.0 - dev/10.0
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// AzimuthDeltaUnits returns the azimuth separation between two peaks in units
// of the azimuth resolution ρ_a. The SAR image is assumed to be sampled at
// one pixel per resolution cell, so the pixel delta equals the resolution
// units directly.
func AzimuthDeltaUnits(mainAzimuth, candAzimuth int) float64 {
	d := math.Abs(float64(candAzimuth - mainAzimuth))
	if d < 0 {
		return -d
	}
	return d
}

// CorrelationReport bundles the per-pair evidence used by the classifier and
// stored on the candidate.
type CorrelationReport struct {
	OffsetUnits      float64 // 以 ρ_a 为单位
	AzimuthOffsetM   float64 // 米
	IntensityRatioDB float64 // 主峰 - 疑似峰（dB，正值=疑似峰更弱）
	ResponseScore    float64 // 0..1 方位向响应相似度
	OffsetMatched    bool    // 间距与理论旁瓣位置一致
	RatioOK          bool    // 幅度比在旁瓣衰减带内
}
