package analysis

import (
	"testing"

	"task231-sarsidelobe/internal/imaging"
	"task231-sarsidelobe/internal/model"
)

// testParams mirrors the smoke-test scene: X-band, azimuth resolution 3 m.
func testParams() model.ImagingParams {
	return model.ImagingParams{
		WavelengthM:    0.031,
		SlantRangeM:    600000.0,
		ApertureLenM:   3100.0,
		Polarization:   "HH",
		OrbitDirection: "descending",
		LookAngleDeg:   35,
		AttitudeErrDeg: 0.2,
	}
}

func TestAzimuthResolution(t *testing.T) {
	g := imaging.Compute(testParams())
	// ρ_a = λR/2L = 0.031*600000/(2*3100) = 3.0 m
	if g.AzimuthResolutionM < 2.99 || g.AzimuthResolutionM > 3.01 {
		t.Fatalf("azimuth resolution = %v, want ~3.0", g.AzimuthResolutionM)
	}
	if g.FirstLobeSpacingM < 4.49 || g.FirstLobeSpacingM > 4.51 {
		t.Fatalf("first lobe spacing = %v, want ~4.5", g.FirstLobeSpacingM)
	}
}

func TestMatchesLobeOffset(t *testing.T) {
	g := imaging.Compute(testParams())
	cal := imaging.DefaultCalibration()
	// A suspicious peak 18 m away is exactly 4.0 lobe units -> match.
	if _, ok := imaging.MatchesLobeOffset(18.0, g, cal.OffsetTolerance); !ok {
		t.Fatal("18 m offset should match the 4th sidelobe")
	}
	// 3 m away is 0.667 lobe units -> below the first sidelobe, no match.
	if _, ok := imaging.MatchesLobeOffset(3.0, g, cal.OffsetTolerance); ok {
		t.Fatal("3 m offset should not match any sidelobe")
	}
	// 20 m away is 4.44 lobe units -> |4.44-4|=0.44 > 0.25 tolerance.
	if _, ok := imaging.MatchesLobeOffset(20.0, g, cal.OffsetTolerance); ok {
		t.Fatal("20 m offset should exceed the tolerance")
	}
}

func TestClassifySidelobe(t *testing.T) {
	a := New(testParams(), imaging.DefaultCalibration())
	main := model.PeakRegion{ID: 1, BatchID: 1, PeakAzimuth: 500, PeakIntensityDB: 45.0,
		Status: model.PeakRaw}
	cand := model.PeakRegion{ID: 2, BatchID: 1, PeakAzimuth: 506, PeakIntensityDB: 31.8,
		Status: model.PeakRaw}
	rep := a.Correlate(main, cand)
	cls := Classify(rep, a.Params, a.Cal)
	if cls.Source != SourceSidelobe {
		t.Fatalf("source = %s, want sidelobe (rep=%+v)", cls.Source, rep)
	}
	if rep.ResponseScore < 0.9 {
		t.Fatalf("response score = %v, want >= 0.9", rep.ResponseScore)
	}
}

func TestClassifyAttitude(t *testing.T) {
	p := testParams()
	p.AttitudeErrDeg = 1.2 // exceeds the 0.5° threshold
	a := New(p, imaging.DefaultCalibration())
	main := model.PeakRegion{ID: 1, BatchID: 1, PeakAzimuth: 500, PeakIntensityDB: 45.0,
		Status: model.PeakRaw}
	// Offset of 3 m does not match any sidelobe; ratio 12 dB is in band but
	// the geometry gate fails -> attitude path (with attitude error set).
	cand := model.PeakRegion{ID: 2, BatchID: 1, PeakAzimuth: 501, PeakIntensityDB: 33.0,
		Status: model.PeakRaw}
	rep := a.Correlate(main, cand)
	cls := Classify(rep, a.Params, a.Cal)
	if cls.Source != SourceAttitude {
		t.Fatalf("source = %s, want attitude", cls.Source)
	}
}

func TestClassifyScatter(t *testing.T) {
	a := New(testParams(), imaging.DefaultCalibration())
	main := model.PeakRegion{ID: 1, BatchID: 1, PeakAzimuth: 500, PeakIntensityDB: 45.0,
		Status: model.PeakRaw}
	// Far away, in-band ratio but no geometry match and no attitude error.
	cand := model.PeakRegion{ID: 3, BatchID: 1, PeakAzimuth: 705, PeakIntensityDB: 22.0,
		Status: model.PeakRaw}
	rep := a.Correlate(main, cand)
	cls := Classify(rep, a.Params, a.Cal)
	if cls.Source != SourceScatter {
		t.Fatalf("source = %s, want scatter", cls.Source)
	}
}

func TestRunParallelCandidates(t *testing.T) {
	a := New(testParams(), imaging.DefaultCalibration())
	regions := []model.PeakRegion{
		{ID: 1, BatchID: 1, PeakAzimuth: 500, PeakIntensityDB: 45.0, Status: model.PeakRaw},
		{ID: 2, BatchID: 1, PeakAzimuth: 506, PeakIntensityDB: 31.8, Status: model.PeakRaw},
		{ID: 3, BatchID: 1, PeakAzimuth: 705, PeakIntensityDB: 22.0, Status: model.PeakRaw},
	}
	cands := a.Run(regions)
	if len(cands) != 1 {
		t.Fatalf("candidates = %d, want 1", len(cands))
	}
	if cands[0].Source != SourceSidelobe {
		t.Fatalf("candidate source = %s, want sidelobe", cands[0].Source)
	}
}

func TestSincEnvelopeSymmetry(t *testing.T) {
	// |sinc|² is symmetric and zero at integer offsets in lobe units.
	pos, neg := imaging.SincEnvelope(1.5), imaging.SincEnvelope(-1.5)
	if diff := pos - neg; diff < -1e-12 || diff > 1e-12 {
		t.Fatalf("envelope not symmetric: %v vs %v", pos, neg)
	}
	if imaging.SincEnvelope(1.0) > 1e-9 {
		t.Fatal("first null at x=1 (in lobe units) should be zero")
	}
}
