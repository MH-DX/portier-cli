package rtt

import (
	"math"
)

// TCPStats holds TCP RTT related statistics.
type TCPStats struct {
	SRTT      float64 // Smoothed RTT
	RTTVAR    float64 // RTT variance
	minRTTVAR float64 // Minimum RTT variance
	RTO       float64 // Retransmission timeout
	alpha     float64 // EWMA alpha
	beta      float64 // EWMA beta
	minRTO    float64 // Minimum RTO
	maxRTO    float64 // Maximum RTO
	K         float64 // Factor to multiply with RTTVAR
	minRTT    float64 // Minimum RTT
	hist      SlidingWindowHistogram
}

// NewTCPStats initializes the TCPStats with the first RTT measurement.
func NewTCPStats(initialRTO float64, minRTTVAR float64, alpha float64, beta float64, minRTO float64, maxRTO float64, K float64, histSize int) TCPStats {
	stats := TCPStats{
		SRTT:      10_000_000,
		RTTVAR:    minRTTVAR,
		minRTTVAR: minRTTVAR,
		alpha:     alpha,
		beta:      beta,
		RTO:       initialRTO,
		minRTO:    minRTO,
		maxRTO:    maxRTO,
		K:         K,
		minRTT:    100_000_000.0,
		hist:      *NewSlidingWindowHistogram(histSize),
	}
	return stats
}

// UpdateRTT updates the sRTT and RTTVAR with a new RTT measurement.
func (t *TCPStats) UpdateRTT(rtt float64) {
	// Update sRTT
	t.SRTT = (1-t.alpha)*t.SRTT + t.alpha*rtt

	// Update RTTVAR
	rttdiff := math.Abs(t.SRTT - rtt)
	t.RTTVAR = (1-t.beta)*t.RTTVAR + t.beta*rttdiff

	t.updateRTO()
	if rtt < t.minRTT {
		t.minRTT = rtt
	}
}

func (t *TCPStats) UpdateHistory() {
	t.hist.Add(t.minRTT)
	t.minRTT = 100_000_000.0
}

// GetBaseRTT returns the base RTT.
func (t *TCPStats) GetBaseRTT() float64 {
	return t.hist.Min()
}

// updateRTO updates the RTO based on sRTT and RTTVAR.
func (t *TCPStats) updateRTO() {
	rto := math.Max(t.minRTO, t.SRTT+t.K*math.Max(t.RTTVAR, t.minRTTVAR))
	rto = math.Min(rto, t.maxRTO)
	t.RTO = rto
}
