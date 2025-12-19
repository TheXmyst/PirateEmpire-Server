package economy

import (
	"math"
	"math/rand"
	"time"
)

// WeatherSystem manages global weather conditions (Wind)
type WeatherSystem struct {
	WindDirection float64   // 0-360 degrees
	ValuesLast    time.Time // When was it last updated
	NextChange    time.Time // When will it change next
}

var GlobalWeather *WeatherSystem

func InitWeather() {
	GlobalWeather = &WeatherSystem{
		WindDirection: rand.Float64() * 360,
		ValuesLast:    time.Now(),
		NextChange:    time.Now().Add(time.Minute * time.Duration(30+rand.Intn(31))), // 30-60 mins
	}
}

// Update checks if wind needs changing
func (w *WeatherSystem) Update() {
	if time.Now().After(w.NextChange) {
		w.ChangeWind()
	}
}

func (w *WeatherSystem) ChangeWind() {
	// New random direction
	w.WindDirection = rand.Float64() * 360
	// Schedule next change (30-60 min)
	w.NextChange = time.Now().Add(time.Minute * time.Duration(30+rand.Intn(31)))
	w.ValuesLast = time.Now()
}

// GetWindFactor returns the speed multiplier (e.g. 1.2 for tailwind, 0.5 for headwind)
// travelAngle is in degrees (0-360)
func (w *WeatherSystem) GetWindFactor(travelAngle float64) float64 {
	// Calculate difference
	diff := math.Abs(travelAngle - w.WindDirection)
	if diff > 180 {
		diff = 360 - diff
	}

	// Logic:
	// Diff 0 (Tailwind) -> +50% Speed (1.5)
	// Diff 180 (Headwind) -> -50% Speed (0.5)
	// Linear interpolation? Or curve?
	// User said: "bonus or malus"

	// Cosine interpolation for smooth transition
	// Cos(0) = 1, Cos(180) = -1
	// Map to factor: Base + (Cos(diff) * Strength)
	// Let's say max bonus is 30% (+0.3) and max penalty is 30% (-0.3)
	// Factor = 1.0 + (Cos(diff_rad) * 0.3)

	rad := diff * (math.Pi / 180.0)
	factor := 1.0 + (math.Cos(rad) * 0.3)

	return factor
}
