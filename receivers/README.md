This folder contains the receivers for the cc-metric-collector.

# `metricReceiver.go`
The base class/configuration is located in `metricReceiver.go`.

# Receivers
* `natsReceiver.go`: Receives metrics from the Nats transport system in Influx line protocol encoding. The database name is used as subscription subject for the NATS messages. It uses https://github.com/nats-io/nats.go

# Installation
Nothing to do, all receivers are pure Go code

# Contributing own receivers
A receiver contains three functions and is derived from the type `Receiver` (in `metricReceiver.go`):
* `Init(config ReceiverConfig) error`
* `Start() error`
* `Close()`

The data structures should be set up in `Init()` like opening a file or server connection. The `Start()` function should either start a go routine or issue some other asynchronous mechanism for receiving metrics. The `Close()` function should tear down anything created in `Init()`.

Finally, the receiver needs to be registered in the `metric-collector.go`. There is a list of receivers called `Receivers` which is a map (string -> pointer to receiver). Add a new entry with a descriptive name and the new receiver.
