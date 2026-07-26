package mathutil

import "math"

// LTTB performs Largest-Triangle-Three-Buckets downsampling on a data series.
// It reduces n data points to threshold points while preserving the visual shape.
// dataX and dataY must have the same length. Returns indices of the selected points.
func LTTB(dataX, dataY []float64, threshold int) []int {
	n := len(dataX)
	if n <= threshold || threshold < 3 {
		indices := make([]int, n)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	// Always keep first and last points
	indices := make([]int, 0, threshold)
	indices = append(indices, 0)

	bucketSize := float64(n-2) / float64(threshold-2)

	a := 0 // index of the previous selected point
	for i := 0; i < threshold-2; i++ {
		// Calculate the average point in the next bucket
		bucketStart := int(float64(i+1)*bucketSize) + 1
		bucketEnd := int(float64(i+2)*bucketSize) + 1
		if bucketEnd > n-1 {
			bucketEnd = n - 1
		}

		avgX, avgY := 0.0, 0.0
		count := 0
		for j := bucketStart; j < bucketEnd; j++ {
			avgX += dataX[j]
			avgY += dataY[j]
			count++
		}
		if count > 0 {
			avgX /= float64(count)
			avgY /= float64(count)
		}

		// Find the point in the current bucket with the largest triangle area
		curStart := int(float64(i)*bucketSize) + 1
		curEnd := int(float64(i+1)*bucketSize) + 1
		if curEnd > n-1 {
			curEnd = n - 1
		}

		maxArea := -1.0
		maxIdx := curStart

		for j := curStart; j < curEnd; j++ {
			// Triangle area between point a, point j, and the average of the next bucket
			area := math.Abs(
				(dataX[a]-avgX)*(dataY[j]-dataY[a])-
					(dataX[a]-dataX[j])*(avgY-dataY[a]),
			) * 0.5
			if area > maxArea {
				maxArea = area
				maxIdx = j
			}
		}

		indices = append(indices, maxIdx)
		a = maxIdx
	}

	indices = append(indices, n-1)
	return indices
}

// DownsampleFloat64 applies LTTB to a Y-only data series (X = index).
// Returns the downsampled Y values and their original indices.
func DownsampleFloat64(data []float64, threshold int) ([]float64, []int) {
	n := len(data)
	dataX := make([]float64, n)
	for i := range dataX {
		dataX[i] = float64(i)
	}

	indices := LTTB(dataX, data, threshold)
	result := make([]float64, len(indices))
	for i, idx := range indices {
		result[i] = data[idx]
	}
	return result, indices
}
