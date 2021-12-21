# MultiChanTicker

The idea of this ticker is to multiply the output channels. The original Golang `time.Ticker` provides only a single output channel, so the signal can only be received by a single other class. This ticker allows to add multiple channels which get all notified about the time tick.

```golang
type MultiChanTicker interface {
	Init(duration time.Duration)
	AddChannel(chan time.Time)
}
```

The MultiChanTicker is created similarly to the common `time.Ticker`:

```golang
NewTicker(duration time.Duration) MultiChanTicker
```

Afterwards, you can add channels:

```golang
t := MultiChanTicker(duration)
c1 := make(chan time.Time)
c2 := make(chan time.Time)
t.AddChannel(c1)
t.AddChannel(c2)

for {
    select {
    case t1 :<- c1:
        log.Print(t1)
    case t2 :<- c2:
        log.Print(t2)
    }
}
```

The result should be the same `time.Time` output in both channels, notified "simultaneously".
