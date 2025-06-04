package util

import (
	"fmt"
	"math"
)

// CosineSimilarity calculates the cosine similarity between two float32 vectors.
func CosineSimilarity(vec1 []float32, vec2 []float32) (float64, error) {
	if len(vec1) == 0 || len(vec2) == 0 {
		return 0, fmt.Errorf("input vectors cannot be empty")
	}
	if len(vec1) != len(vec2) {
		return 0, fmt.Errorf("vector dimensions do not match: %d vs %d", len(vec1), len(vec2))
	}

	var dotProduct float64
	var mag1Squared float64
	var mag2Squared float64

	for i := 0; i < len(vec1); i++ {
		dotProduct += float64(vec1[i] * vec2[i])
		mag1Squared += float64(vec1[i] * vec1[i])
		mag2Squared += float64(vec2[i] * vec2[i])
	}

	mag1 := math.Sqrt(mag1Squared)
	mag2 := math.Sqrt(mag2Squared)

	if mag1 == 0 || mag2 == 0 {
		return 0, nil
	}

	similarity := dotProduct / (mag1 * mag2)
	return similarity, nil
}
