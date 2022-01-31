# CCMetric receivers

This folder contains the ReceiveManager and receiver implementations for the cc-metric-collector.

# Configuration

The configuration file for the receivers is a list of configurations. The `type` field in each specifies which receiver to initialize.

```json
[
  {
    "type": "nats",
    "address": "nats://my-url",
    "port" : "4222",
    "database": "testcluster"
  }
]
```


## Type `nats`

```json
{
  "type": "nats",
  "address": "<nats-URI or hostname>",
  "port" : "<portnumber>",
  "database": "<subscribe topic>"
}
```

The `nats` receiver subscribes to the topic `database` and listens on `address` and `port` for metrics in the InfluxDB line protocol.

# Contributing own receivers
A receiver contains three functions and is derived from the type `Receiver` (in `metricReceiver.go`):
* `Init(config ReceiverConfig) error`
* `Start() error`
* `Close()`
* `Name() string`
* `SetSink(sink chan ccMetric.CCMetric)`

The data structures should be set up in `Init()` like opening a file or server connection. The `Start()` function should either start a go routine or issue some other asynchronous mechanism for receiving metrics. The `Close()` function should tear down anything created in `Init()`.

Finally, the receiver needs to be registered in the `receiveManager.go`. There is a list of receivers called `AvailableReceivers` which is a map (`receiver_type_string` -> `pointer to Receiver interface`). Add a new entry with a descriptive name and the new receiver.
