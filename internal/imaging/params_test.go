package imaging

import (
	"errors"
	"testing"

	"task231-sarsidelobe/internal/model"
)

func TestValidateParamsRejectsMissingPolarization(t *testing.T) {
	p := model.ImagingParams{WavelengthM: 0.03, SlantRangeM: 600000, ApertureLenM: 3000}
	if err := ValidateParams(&p); !errors.Is(err, model.ErrPolarizationMissing) {
		t.Fatalf("ValidateParams() error = %v, want %v", err, model.ErrPolarizationMissing)
	}
}

func TestValidateParamsDefaultsOrbitDirection(t *testing.T) {
	p := model.ImagingParams{
		WavelengthM: 0.031, SlantRangeM: 600000, ApertureLenM: 3100,
		Polarization: " hh ", LookAngleDeg: 35,
	}
	if err := ValidateParams(&p); err != nil {
		t.Fatalf("ValidateParams() error = %v", err)
	}
	if p.Polarization != " hh " {
		t.Fatalf("ValidateParams() unexpectedly rewrote polarization = %q", p.Polarization)
	}
	if p.OrbitDirection != "descending" {
		t.Fatalf("OrbitDirection = %q, want descending", p.OrbitDirection)
	}
}

func TestLobePeakRatioRejectsInvalidOrder(t *testing.T) {
	if got := LobePeakRatioDB(0); got != 0 {
		t.Fatalf("LobePeakRatioDB(0) = %v, want 0", got)
	}
	if got := LobePeakRatioDB(1); got >= 0 {
		t.Fatalf("first sidelobe ratio = %v, want negative dB", got)
	}
}
