## `nats` receiver

The `nats` receiver can be used receive metrics from the NATS network. The `nats` receiver subscribes to the topic `database` and listens on `address` and `port` for metrics in the InfluxDB line protocol.

### Configuration structure

```json
{
  "<name>": {
    "type": "nats",
    "address" : "nats-server.example.org",
    "port" : "4222",
    "subject" : "subject"
  }
}
```

- `type`: makes the receiver a `nats` receiver
- `address`: Address of the NATS control server
- `port`: Port of the NATS control server
- `subject`: Subscribes to this subject and receive metrics
