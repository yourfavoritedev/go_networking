package heartbeats

import (
	"context"
	"io"
	"time"
)

const defaultPingInterval = 30 * time.Second

// Pinger writes ping messages to a given writer at regular intervals
func Pinger(ctx context.Context, w io.Writer, reset <-chan time.Duration) {
	var interval time.Duration

	select {
	case <-ctx.Done():
		return
	case interval = <-reset: // pulls iniital interval off reset channel
	default:
	}

	if interval <= 0 {
		interval = defaultPingInterval
	}

	// initialize timer to the interval
	timer := time.NewTimer(interval)
	// read from timer C at end of execution to prevent leaking if necessary
	defer func() {
		if !timer.Stop() {
			<-timer.C
		}
	}()

	for {
		select {
		// context is cancelled, exit pinger
		case <-ctx.Done():
			return
		// signal to reset the timer is received
		case newInterval := <-reset:
			if !timer.Stop() {
				<-timer.C
			}
			// adjust interval
			if newInterval > 0 {
				interval = newInterval
			}
		// timer expires, write ping message
		case <-timer.C:
			// write ping message to the writer
			if _, err := w.Write([]byte("ping")); err != nil {
				// track and act on consecutive timeouts here
				return
			}
		}
		// reset timer for new interval
		_ = timer.Reset(interval)
	}
}
