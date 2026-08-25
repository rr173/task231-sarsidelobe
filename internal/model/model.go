package model

// Batch is a SAR imaging acquisition that receives antenna parameters and
// peak-region summaries before sidelobe diagnosis.
type Batch struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Sensor    string `json:"sensor"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ImagingParams holds the antenna / acquisition geometry that determines the
// theoretical azimuth resolution and therefore the expected sidelobe spacing.
type ImagingParams struct {
	ID             int64   `json:"id"`
	BatchID        int64   `json:"batch_id"`
	WavelengthM    float64 `json:"wavelength_m"`    // λ
	SlantRangeM    float64 `json:"slant_range_m"`   // R
	ApertureLenM   float64 `json:"aperture_len_m"`  // L（合成孔径长度）
	Polarization   string  `json:"polarization"`    // HH / VV / HV / VH，必填
	OrbitDirection string  `json:"orbit_direction"` // ascending / descending
	LookAngleDeg   float64 `json:"look_angle_deg"`
	AttitudeErrDeg float64 `json:"attitude_err_deg"` // 姿态误差先验
	CalibrationID  int64   `json:"calibration_id"`   // 生效校准版本
	CreatedAt      string  `json:"created_at"`
}

// CalibrationVersion is an immutable calibration parameter set. Only one
// version can be active at a time.
type CalibrationVersion struct {
	ID              int64   `json:"id"`
	Version         int     `json:"version"`
	Name            string  `json:"name"`
	Active          bool    `json:"active"`
	FirstLobeDB     float64 `json:"first_lobe_db"`    // 第一旁瓣相对主瓣强度（dB）
	OffsetTolerance float64 `json:"offset_tolerance"` // 旁瓣间距匹配容差（倍率）
	RatioMinDB      float64 `json:"ratio_min_db"`     // 幅度比下界
	RatioMaxDB      float64 `json:"ratio_max_db"`     // 幅度比上界
	CreatedAt       string  `json:"created_at"`
}

// PeakRegion is a local maximum of the SAR image with its range/azimuth
// bounding box and peak intensity. Region identity is content-hashed so the
// same region cannot be registered twice per batch.
type PeakRegion struct {
	ID              int64   `json:"id"`
	BatchID         int64   `json:"batch_id"`
	RegionHash      string  `json:"region_hash"`
	RangeStart      int     `json:"range_start"`
	RangeEnd        int     `json:"range_end"`
	AzimuthStart    int     `json:"azimuth_start"`
	AzimuthEnd      int     `json:"azimuth_end"`
	PeakAzimuth     int     `json:"peak_azimuth"`
	PeakIntensityDB float64 `json:"peak_intensity_db"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
}

// Candidate is a contamination hypothesis: the analyzer paired a strong
// scatterer (main peak) with a suspicious peak located near a theoretical
// sidelobe position. Evidence is attached during review.
type Candidate struct {
	ID               int64   `json:"id"`
	BatchID          int64   `json:"batch_id"`
	MainPeakID       int64   `json:"main_peak_id"`
	SidelobePeakID   int64   `json:"sidelobe_peak_id"`
	AzimuthOffsetM   float64 `json:"azimuth_offset_m"`
	OffsetUnits      float64 `json:"offset_units"` // 以方位分辨率 ρ_a 为单位
	IntensityRatioDB float64 `json:"intensity_ratio_db"`
	ResponseScore    float64 `json:"response_score"` // 方位向响应相似度 0..1
	Source           string  `json:"source"`         // sidelobe / attitude / scatter
	Status           string  `json:"status"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// Evidence is a review-time attachment, e.g. an attitude calibration report.
type Evidence struct {
	ID          int64  `json:"id"`
	CandidateID int64  `json:"candidate_id"`
	Kind        string `json:"kind"` // attitude_calibration / geometry_override / operator_note
	Note        string `json:"note"`
	CreatedAt   string `json:"created_at"`
}

// Snapshot freezes a diagnosis outcome for a batch. Once published it cannot
// change; a newer version supersedes it.
type Snapshot struct {
	ID        int64  `json:"id"`
	BatchID   int64  `json:"batch_id"`
	Version   int    `json:"version"`
	Status    string `json:"status"`
	Content   string `json:"content"` // JSON blob of frozen diagnosis
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// AnalysisRun records one execution of the sidelobe analysis for a batch.
type AnalysisRun struct {
	ID              int64  `json:"id"`
	BatchID         int64  `json:"batch_id"`
	Status          string `json:"status"`
	CandidatesFound int    `json:"candidates_found"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at"`
}

// ToStatus maps sentinel errors to HTTP status codes.
func ToStatus(err error) int {
	switch err {
	case ErrNotFound:
		return 404
	case ErrDuplicate, ErrRepeatedRegion, ErrSnapshotFrozen:
		return 409
	case ErrStateTransition, ErrArchivedMutation, ErrPeakSealed, ErrAnalyzeLocked:
		return 409
	case ErrPolarizationMissing, ErrCoordinateRange, ErrBadRequest:
		return 400
	default:
		return 500
	}
}
