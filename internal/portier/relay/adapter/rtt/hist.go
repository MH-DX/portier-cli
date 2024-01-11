package rtt

import (
	"gonum.org/v1/gonum/stat"
	"gopkg.in/eapache/queue.v1"
)

// SlidingWindowHistogram maintains a histogram over a sliding window of data.
type SlidingWindowHistogram struct {
	windowSize int
	data       *queue.Queue
}

// NewSlidingWindowHistogram creates a new SlidingWindowHistogram with a given size.
func NewSlidingWindowHistogram(size int) *SlidingWindowHistogram {
	return &SlidingWindowHistogram{
		windowSize: size,
		data:       queue.New(),
	}
}

// Add adds a new value to the histogram, sliding the window if necessary.
func (h *SlidingWindowHistogram) Add(value float64) {
	if h.data.Length() >= h.windowSize {
		h.data.Remove()
	}
	h.data.Add(value)
}

// Mean calculates the mean of the data in the window.
func (h *SlidingWindowHistogram) Mean() float64 {
	return stat.Mean(h.convertToArray(), nil)
}

// StdDev calculates the standard deviation of the data in the window.
func (h *SlidingWindowHistogram) StdDev() float64 {
	return stat.StdDev(h.convertToArray(), nil)
}

// Min calculates the minimum value of the data in the window.
func (h *SlidingWindowHistogram) Min() float64 {
	// iterate over all values in the window and find the minimum
	min := float64(0)
	for i := 0; i < h.data.Length(); i++ {
		value := h.data.Get(i).(float64)
		if i == 0 || value < min {
			min = value
		}
	}
	return min
}

// convertToArray converts the data in the window to an array.
func (h *SlidingWindowHistogram) convertToArray() []float64 {
	data := make([]float64, h.data.Length())
	for i := 0; i < h.data.Length(); i++ {
		data[i] = h.data.Get(i).(float64)
	}
	return data
}
