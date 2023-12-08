# CCMetric receivers

This folder contains the ReceiveManager and receiver implementations for the cc-metric-collector.

## Configuration

The configuration file for the receivers is a list of configurations. The `type` field in each specifies which receiver to initialize.

```json
{
  "myreceivername" : {
    "type": "receiver-type",
    <receiver-specific configuration>
  }
}
```

This allows to specify

## Available receivers

- [`nats`](./natsReceiver.md): Receive metrics from the NATS network
- [`prometheus`](./prometheusReceiver.md): Scrape data from a Prometheus client
- [`http`](./httpReceiver.md): Listen for HTTP Post requests transporting metrics in InfluxDB line protocol
- [`ipmi`](./ipmiReceiver.md): Read IPMI sensor readings
- [`redfish`](redfishReceiver.md) Use the Redfish (specification) to query thermal and power metrics

## Contributing own receivers

A receiver contains a few functions and is derived from the type `Receiver` (in `metricReceiver.go`):

For an example, check the [sample receiver](./sampleReceiver.go)
