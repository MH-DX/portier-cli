package windowitem

import (
	"time"

	"github.com/marinator86/portier-cli/internal/portier/relay/messages"
)

// A windowItem is an item in the window.
type WindowItem struct {
	Msg           messages.Message
	Seq           uint64
	Time          time.Time
	Rto           time.Time
	Acked         bool
	Retransmitted bool
	RtoDuration   time.Duration
}
