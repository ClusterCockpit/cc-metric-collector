// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package multiChanTicker

import (
	"fmt"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/v2/ccLogger"
)

type multiChanTicker struct {
	ticker   *time.Ticker
	mutex    sync.Mutex // protects channels, which is appended to while the tick goroutine iterates it
	channels []chan time.Time
	done     chan bool
}

type MultiChanTicker interface {
	Init(duration time.Duration)
	AddChannel(channel chan time.Time)
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
				t.mutex.Lock()
				for i, c := range t.channels {
					// Non-blocking send: a consumer that has not yet read the
					// previous tick must not stall the ticker, otherwise
					// time.Ticker silently drops fires for ALL consumers
					select {
					case c <- ts:
					default:
						cclog.ComponentWarn("MultiChanTicker", fmt.Sprintf("consumer %d did not read previous tick, dropping tick %v", i, ts))
					}
				}
				t.mutex.Unlock()
			}
		}
	}()
}

func (t *multiChanTicker) AddChannel(channel chan time.Time) {
	if cap(channel) == 0 {
		cclog.ComponentWarn("MultiChanTicker", "unbuffered channel registered, ticks may be dropped if the consumer is not ready")
	}
	t.mutex.Lock()
	t.channels = append(t.channels, channel)
	t.mutex.Unlock()
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
