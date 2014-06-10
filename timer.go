package udp

import (
	"time"

	"github.com/reusee/closer"
)

type Timer struct {
	closer.Closer
	Now  uint
	Tick chan struct{}
}

func NewTimer(interval time.Duration) *Timer {
	timer := &Timer{
		Tick: make(chan struct{}),
	}
	ticker := time.NewTicker(interval)
	stop := make(chan bool)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				timer.Now++
				timer.Tick <- struct{}{}
			}
		}
	}()
	timer.OnClose(func() {
		close(stop)
	})
	return timer
}
