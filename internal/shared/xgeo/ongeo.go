package xgeo

import (
	"fmt"
	"math"
)

// Coordinates is a geographic point. Values must be finite and non-NaN for the
// struct to be safe as a map key — use CanonicalizeCoordinates to obtain a
// guaranteed-stable instance from raw float64 inputs.
type Coordinates struct {
	Latitude  float64
	Longitude float64
}

// CanonicalizeCoordinates validates lat/lon and returns a Coordinates with
// negative zero normalized to positive zero, making it stable as a map key.
func CanonicalizeCoordinates(lat, lon float64) (Coordinates, error) {
	if err := ValidateCoordinates(lat, lon); err != nil {
		return Coordinates{}, err
	}
	if lat == 0 {
		lat = math.Abs(lat)
	}
	if lon == 0 {
		lon = math.Abs(lon)
	}
	return Coordinates{Latitude: lat, Longitude: lon}, nil
}

// ValidateCoordinates returns an error if lat or lon is not a finite number
// within the standard geographic range (lat ∈ [-90, 90], lon ∈ [-180, 180]).
func ValidateCoordinates(lat, lon float64) error {
	if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
		return fmt.Errorf("latitude must be a finite number in [-90, 90], got %g", lat)
	}
	if math.IsNaN(lon) || math.IsInf(lon, 0) || lon < -180 || lon > 180 {
		return fmt.Errorf("longitude must be a finite number in [-180, 180], got %g", lon)
	}
	return nil
}
