package multiChanTicker

import (
	"time"
)

type multiChanTicker struct {
	ticker   *time.Ticker
	channels []chan time.Time
}

type MultiChanTicker interface {
	Init(duration time.Duration)
	AddChannel(chan time.Time)
}

func (t *multiChanTicker) Init(duration time.Duration) {
	t.ticker = time.NewTicker(duration)
	go func() {
		for {
			select {
			case ts := <-t.ticker.C:
				for _, c := range t.channels {
					c <- ts
				}
			}
		}
	}()
}

func (t *multiChanTicker) AddChannel(channel chan time.Time) {
	t.channels = append(t.channels, channel)
}

func NewTicker(duration time.Duration) MultiChanTicker {
	t := &multiChanTicker{}
	t.Init(duration)
	return t
}
