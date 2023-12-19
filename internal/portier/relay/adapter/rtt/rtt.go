package rtt

import (
	"math"
)

// TCPStats holds TCP RTT related statistics
type TCPStats struct {
	SRTT   float64 // Smoothed RTT
	RTTVAR float64 // RTT variance
	RTO    float64 // Retransmission timeout
	alpha  float64 // EWMA alpha
	beta   float64 // EWMA beta
	minRTO float64 // Minimum RTO
	K      float64 // Factor to multiply with RTTVAR
}

// NewTCPStats initializes the TCPStats with the first RTT measurement.
func NewTCPStats(initialRTT float64, alpha float64, beta float64, minRTO float64, K float64) TCPStats {
	stats := TCPStats{
		SRTT:   initialRTT,
		RTTVAR: initialRTT / 2,
		alpha:  alpha,
		beta:   beta,
		minRTO: minRTO,
		K:      K,
	}
	stats.updateRTO()
	return stats
}

// UpdateRTT updates the sRTT and RTTVAR with a new RTT measurement.
func (t *TCPStats) UpdateRTT(rtt float64) {
	// Update sRTT
	t.SRTT = (1-t.alpha)*t.SRTT + t.alpha*rtt

	// Update RTTVAR
	rttdiff := math.Abs(t.SRTT - rtt)
	t.RTTVAR = (1-t.beta)*t.RTTVAR + t.beta*rttdiff

	// Update RTO
	t.updateRTO()
}

// updateRTO updates the RTO based on sRTT and RTTVAR.
func (t *TCPStats) updateRTO() {
	rto := t.SRTT + math.Max(t.minRTO, t.K*t.RTTVAR)
	t.RTO = rto
}
