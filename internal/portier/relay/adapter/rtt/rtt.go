package rtt

import (
	"math"
)

// TCPStats holds TCP RTT related statistics.
type TCPStats struct {
	SRTT   float64 // Smoothed RTT
	RTTVAR float64 // RTT variance
	RTO    float64 // Retransmission timeout
	alpha  float64 // EWMA alpha
	beta   float64 // EWMA beta
	minRTO float64 // Minimum RTO
	maxRTO float64 // Maximum RTO
	K      float64 // Factor to multiply with RTTVAR
	hist   SlidingWindowHistogram
}

// NewTCPStats initializes the TCPStats with the first RTT measurement.
func NewTCPStats(initialRTO float64, alpha float64, beta float64, minRTO float64, maxRTO float64, K float64, histSize int) TCPStats {
	stats := TCPStats{
		SRTT:   0,
		RTTVAR: 0,
		alpha:  alpha,
		beta:   beta,
		RTO:    initialRTO,
		minRTO: minRTO,
		maxRTO: maxRTO,
		K:      K,
		hist:   *NewSlidingWindowHistogram(histSize),
	}
	return stats
}

// IsInitialized returns true if the TCPStats have been initialized with an RTT measurement.
func (t *TCPStats) IsInitialized() bool {
	return t.hist.data.Length() > 0
}

// Init initializes the TCPStats with the first RTT measurement.
func (t *TCPStats) Init(initialRTT float64) {
	t.SRTT = initialRTT
	t.RTTVAR = initialRTT / 3.0
	t.hist.Add(initialRTT)
}

// UpdateRTT updates the sRTT and RTTVAR with a new RTT measurement.
func (t *TCPStats) UpdateRTT(rtt float64) {
	// Update sRTT
	t.SRTT = (1-t.alpha)*t.SRTT + t.alpha*rtt

	// Update RTTVAR
	rttdiff := math.Abs(t.SRTT - rtt)
	t.RTTVAR = (1-t.beta)*t.RTTVAR + t.beta*rttdiff

	t.updateRTO()
	t.hist.Add(rtt)
}

// GetBaseRTT returns the base RTT.
func (t *TCPStats) GetBaseRTT() float64 {
	return t.hist.Min()
}

// updateRTO updates the RTO based on sRTT and RTTVAR.
func (t *TCPStats) updateRTO() {
	rto := math.Max(t.minRTO, t.SRTT+t.K*t.RTTVAR)
	rto = math.Min(rto, t.maxRTO)
	t.RTO = rto
}
