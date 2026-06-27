package xgeo_test

import (
	"math"
	"testing"

	"github.com/raphoester/movie-reservation-system/internal/shared/xgeo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCoordinates(t *testing.T) {
	t.Run("valid coordinates are accepted", func(t *testing.T) {
		require.NoError(t, xgeo.ValidateCoordinates(48.8584, 2.2945))
	})

	t.Run("boundary values are accepted", func(t *testing.T) {
		require.NoError(t, xgeo.ValidateCoordinates(-90, -180))
		require.NoError(t, xgeo.ValidateCoordinates(90, 180))
	})

	t.Run("latitude below -90 is rejected", func(t *testing.T) {
		assert.Error(t, xgeo.ValidateCoordinates(-90.0001, 0))
	})

	t.Run("latitude above 90 is rejected", func(t *testing.T) {
		assert.Error(t, xgeo.ValidateCoordinates(90.0001, 0))
	})

	t.Run("longitude below -180 is rejected", func(t *testing.T) {
		assert.Error(t, xgeo.ValidateCoordinates(0, -180.0001))
	})

	t.Run("longitude above 180 is rejected", func(t *testing.T) {
		assert.Error(t, xgeo.ValidateCoordinates(0, 180.0001))
	})

	t.Run("NaN latitude is rejected", func(t *testing.T) {
		assert.Error(t, xgeo.ValidateCoordinates(math.NaN(), 0))
	})

	t.Run("NaN longitude is rejected", func(t *testing.T) {
		assert.Error(t, xgeo.ValidateCoordinates(0, math.NaN()))
	})

	t.Run("infinite latitude is rejected", func(t *testing.T) {
		require.Error(t, xgeo.ValidateCoordinates(math.Inf(1), 0))
		require.Error(t, xgeo.ValidateCoordinates(math.Inf(-1), 0))
	})

	t.Run("infinite longitude is rejected", func(t *testing.T) {
		require.Error(t, xgeo.ValidateCoordinates(0, math.Inf(1)))
		require.Error(t, xgeo.ValidateCoordinates(0, math.Inf(-1)))
	})
}

func TestHaversineKm(t *testing.T) {
	t.Run("identical points return zero distance", func(t *testing.T) {
		lat, lon := 48.8566, 2.3522
		got := xgeo.HaversineKm(lat, lon, lat, lon)
		assert.InDelta(t, 0.0, got, 1e-9)
	})

	t.Run("Paris to London is approximately 340 km", func(t *testing.T) {
		parisLat, parisLon := 48.8566, 2.3522
		londonLat, londonLon := 51.5074, -0.1278
		got := xgeo.HaversineKm(parisLat, parisLon, londonLat, londonLon)
		assert.InDelta(t, 340.0, got, 5.0)
	})

	t.Run("is commutative", func(t *testing.T) {
		parisLat, parisLon := 48.8566, 2.3522
		londonLat, londonLon := 51.5074, -0.1278
		d1 := xgeo.HaversineKm(parisLat, parisLon, londonLat, londonLon)
		d2 := xgeo.HaversineKm(londonLat, londonLon, parisLat, parisLon)
		assert.InDelta(t, d1, d2, 1e-9)
	})

	t.Run("antipodal points return approximately half Earth circumference", func(t *testing.T) {
		got := xgeo.HaversineKm(0, 0, 0, 180)
		assert.InDelta(t, math.Pi*6371.0, got, 1.0)
	})
}
