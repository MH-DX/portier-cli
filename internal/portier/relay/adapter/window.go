package adapter

import (
	"context"
	"errors"
	"log"
	"math"
	"sync"
	"time"

	"github.com/mh-dx/portier-cli/internal/portier/relay/adapter/rto_heap"
	"github.com/mh-dx/portier-cli/internal/portier/relay/adapter/rtt"
	"github.com/mh-dx/portier-cli/internal/portier/relay/encoder"
	"github.com/mh-dx/portier-cli/internal/portier/relay/messages"
	"github.com/mh-dx/portier-cli/internal/portier/relay/uplink"
	windowitem "github.com/mh-dx/portier-cli/internal/portier/relay/window_item"
	"gopkg.in/eapache/queue.v1"
)

type WindowOptions struct {
	// initial size of the window in bytes
	InitialCap float64

	// minimum rtt variance of the window in microseconds
	MinRTTVAR float64

	// minimum rto of the window in microseconds
	MinRTO float64

	// maximum rto of the window in microseconds
	MaxRTO float64

	// initial rto of the window in microseconds
	InitialRTO float64

	// rtt factor for calculating the rto
	RTTFactor float64 // 4.0

	// ewma alpha
	EWMAAlpha float64 // 0.125

	// ewma beta
	EWMABeta float64 // 0.25

	// MaxCap is the maximum size of the window in bytes
	MaxCap float64

	// WindowDownscaleFactor is the factor by which the window is downscaled when a retransmission is detected
	WindowDownscaleFactor float64

	// WindowUpscaleFactor is the factor by which the window is upscaled when a retransmission is not detected
	WindowUpscaleFactor float64

	// HistSize is the size of the sliding window histogram
	RTTHistSize int
}

type Window interface {
	// add is called when a message is added to the window
	// seq is the sequence number of the message
	// this function will block until there is enough space in the window
	add(msg messages.Message, seq uint64) error

	// ack is called when a message has been ack'ed by peer
	// seq is the sequence number of the ack'ed message
	// retransmitted indicates if the message rtt was a retransmission
	// returns the rtt of the message, and a flag indicating if the message was a retransmission or influrnced by a retransmission (i.e. rtt is not accurate)
	ack(seq uint64, retransmitted bool) error
}

type window struct {
	options        WindowOptions
	currentSize    int
	currentCap     float64
	currentBaseRTT float64
	queue          *queue.Queue
	mutex          *sync.Mutex
	cond           *sync.Cond
	uplink         uplink.Uplink
	stats          *rtt.TCPStats
	rtoHeap        rto_heap.RtoHeap
	baseRTTTicker  *time.Ticker
}

func NewDefaultWindowOptions() WindowOptions {
	return WindowOptions{
		InitialCap:            32768 * 4,
		MinRTTVAR:             5_000_000.0,
		MinRTO:                50_000_000.0,
		MaxRTO:                500_000_000.0,
		InitialRTO:            100_000_000.0,
		RTTFactor:             10.0,
		EWMAAlpha:             0.125,
		EWMABeta:              0.25,
		MaxCap:                32768 * 32,
		WindowDownscaleFactor: 0.995,
		WindowUpscaleFactor:   1.0005,
		RTTHistSize:           10,
	}
}

func NewWindow(ctx context.Context, options WindowOptions, uplink uplink.Uplink, encoderDecoder encoder.EncoderDecoder) Window {
	rtoHeap := rto_heap.NewRtoHeap(ctx, rto_heap.NewDefaultRtoHeapOptions(), uplink, encoderDecoder)
	return newWindow(ctx, options, uplink, rtoHeap)
}

func newWindow(ctx context.Context, options WindowOptions, uplink uplink.Uplink, rtoHeap rto_heap.RtoHeap) Window {
	mutex := sync.Mutex{}

	stats := rtt.NewTCPStats(options.InitialRTO, options.MinRTTVAR, options.EWMAAlpha, options.EWMABeta, options.MinRTO, options.MaxRTO, options.RTTFactor, options.RTTHistSize)

	baseRTTTicker := time.NewTicker(1 * time.Minute)
	window := &window{
		options:        options,
		currentSize:    0,
		currentCap:     options.InitialCap,
		currentBaseRTT: 100_000_000,
		queue:          queue.New(),
		mutex:          &mutex,
		cond:           sync.NewCond(&mutex),
		uplink:         uplink,
		stats:          &stats,
		rtoHeap:        rtoHeap,
		baseRTTTicker:  baseRTTTicker,
	}

	go func() {
		for {
			// update the base rtt
			stats.UpdateHistory()
			window.currentBaseRTT = stats.GetBaseRTT()
			log.Printf("updated base rtt: %fms\n", window.currentBaseRTT/1_000_000.0)
			select {
			case <-baseRTTTicker.C:
				continue
			case <-ctx.Done():
				return
			}
		}
	}()

	return window
}

func (w *window) add(msg messages.Message, seq uint64) error {
	w.mutex.Lock()
	defer func() { w.mutex.Unlock() }()

	for w.currentSize+len(msg.Message) > int(w.currentCap) {
		// wait until there is enough space in the window
		w.cond.Wait()
	}
	w.currentSize += len(msg.Message)
	now := time.Now()
	rtoDuration := time.Duration(w.stats.RTO) * time.Nanosecond
	item := &windowitem.WindowItem{
		Msg:           msg,
		Seq:           seq,
		Time:          now,
		Rto:           now.Add(rtoDuration),
		Acked:         false,
		Retransmitted: false,
		RtoDuration:   rtoDuration,
	}
	w.queue.Add(item)
	_ = w.rtoHeap.Add(item)
	_ = w.uplink.Send(msg)
	return nil
}

func (w *window) ack(seq uint64, retransmitted bool) error {
	w.mutex.Lock()
	defer func() { w.mutex.Unlock() }()

	if w.queue.Length() == 0 {
		return nil
	}

	// determine the index of the message in the queue using the sequence number
	first := w.queue.Peek().(*windowitem.WindowItem).Seq
	index := int(seq - first)
	if index < 0 || index >= w.queue.Length() {
		// the message is not in the window anymore - it has already been ack'ed
		return nil
	}
	// get the message from the queue
	item := w.queue.Get(index).(*windowitem.WindowItem)
	if item.Acked {
		// the message has already been ack'ed
		return errors.New("message_already_acked")
	}

	// mark the message as ack'ed
	item.Acked = true
	defer func() { w.cond.Signal() }()
	item.Retransmitted = retransmitted
	if !retransmitted {
		rtt := float64(time.Since(item.Time))
		w.stats.UpdateRTT(rtt)
		if w.currentBaseRTT < w.stats.SRTT-w.stats.RTTVAR {
			newCap := math.Max(w.currentCap*w.options.WindowDownscaleFactor, w.options.InitialCap)
			if newCap < w.currentCap {
				w.currentCap = newCap
			}
		} else {
			newCap := math.Min(w.currentCap*w.options.WindowUpscaleFactor, w.options.MaxCap)
			if newCap > w.currentCap {
				w.currentCap = newCap
			}

		}
	}
	// remove all messages from the queue that have been ack'ed
	for w.queue.Length() > 0 {
		item := w.queue.Peek().(*windowitem.WindowItem)
		if item.Acked {
			w.queue.Remove()
			w.currentSize -= len(item.Msg.Message)
		} else {
			break
		}
	}
	return nil
}
