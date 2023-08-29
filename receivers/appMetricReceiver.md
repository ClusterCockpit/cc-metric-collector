## `appmetrics` receiver

The `appmetrics` receiver can be used to submit metrics from an application into the monitoring system. It listens for incoming connections on a UNIX socket.

### Configuration structure

```json
{
  "<name>": {
    "type": "appmetrics",
    "socket_file" : "/tmp/cc.sock",
  }
}
```

- `type`: makes the receiver a `appmetrics` receiver
- `socket_file`: Listen UNIX socket

### Inputs from applications

Applcations can connect to the `appmetrics` socket and provide metric in the [InfluxDB line protocol](https://github.com/influxdata/line-protocol). It is currently not possible to submit meta information as the Influx line protocol does not know them.


