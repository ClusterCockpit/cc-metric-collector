// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package multiChanTicker

import (
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
)

type multiChanTicker struct {
	ticker   *time.Ticker
	channels []chan time.Time
	done     chan bool
}

type MultiChanTicker interface {
	Init(duration time.Duration)
	AddChannel(chan time.Time)
	Close()
}

func (t *multiChanTicker) Init(duration time.Duration) {
	t.ticker = time.NewTicker(duration)
	t.done = make(chan bool)
	go func() {
		done := func() {
			close(t.done)
			cclog.ComponentDebug("MultiChanTicker", "DONE")
		}
		for {
			select {
			case <-t.done:
				done()
				return
			case ts := <-t.ticker.C:
				cclog.ComponentDebug("MultiChanTicker", "Tick", ts)
				for _, c := range t.channels {
					select {
					case <-t.done:
						done()
						return
					case c <- ts:
					}
				}
			}
		}
	}()
}

func (t *multiChanTicker) AddChannel(channel chan time.Time) {
	t.channels = append(t.channels, channel)
}

func (t *multiChanTicker) Close() {
	cclog.ComponentDebug("MultiChanTicker", "CLOSE")
	t.done <- true
	// wait for close of channel t.done
	<-t.done
}

func NewTicker(duration time.Duration) MultiChanTicker {
	t := &multiChanTicker{}
	t.Init(duration)
	return t
}
