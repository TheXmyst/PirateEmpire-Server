package economy

import "math"

// CalculateDistance calculates Euclidean distance between two points
func CalculateDistance(x1, y1, x2, y2 int) float64 {
	return math.Sqrt(math.Pow(float64(x2-x1), 2) + math.Pow(float64(y2-y1), 2))
}
