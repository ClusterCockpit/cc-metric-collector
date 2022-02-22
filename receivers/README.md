# CCMetric receivers

This folder contains the ReceiveManager and receiver implementations for the cc-metric-collector.

# Configuration

The configuration file for the receivers is a list of configurations. The `type` field in each specifies which receiver to initialize.

```json
{
  "myreceivername" : {
    <receiver-specific configuration>
  }
}
```


## Type `nats`

```json
{
  "type": "nats",
  "address": "<nats-URI or hostname>",
  "port" : "<portnumber>",
  "subject": "<subscribe topic>"
}
```

The `nats` receiver subscribes to the topic `database` and listens on `address` and `port` for metrics in the InfluxDB line protocol.

# Contributing own receivers
A receiver contains a few functions and is derived from the type `Receiver` (in `metricReceiver.go`):
* `Init(name string, config json.RawMessage) error`
* `Start() error`
* `Close()`
* `Name() string`
* `SetSink(sink chan lp.CCMetric)`
* `New<Typename>(name string, config json.RawMessage)`

The data structures should be set up in `Init()` like opening a file or server connection. The `Start()` function should either start a go routine or issue some other asynchronous mechanism for receiving metrics. The `Close()` function should tear down anything created in `Init()`.

Finally, the receiver needs to be registered in the `receiveManager.go`. There is a list of receivers called `AvailableReceivers` which is a map (`receiver_type_string` -> `pointer to NewReceiver function`). Add a new entry with a descriptive name and the new receiver.
