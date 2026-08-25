package hier

// abs returns the absolute value of v
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// absf32 returns the absolute value of v
func absf32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// minf32 returns the smaller of a and b
func minf32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// maxf32 returns the larger of a and b
func maxf32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// clampf32 limits v to the range [min, max]
func clampf32(v, min, max float32) float32 {
	if v < min {
		return min
	} else if v > max {
		return max
	}
	return v
}
