# SNMP Receiver

```json
  "<name>": {
    "type": "snmp",
    "read_interval": "30s",
    "targets" : [{
        "hostname" : "host1.example.com",
        "port" : 161,
        "community": "public",
        "timeout" : 1,
    }],
    "metrics" : [
        {
            "name": "sensor1",
            "value": "1.3.6.1.2.1.1.4.0",
            "unit": "1.3.6.1.2.1.1.7.0",
        },
        {
            "name": "1.3.6.1.2.1.1.2.0",
            "value": "1.3.6.1.2.1.1.4.0",
            "unit": "mb/s",
        }
    ]
  }
```

The `snmp` receiver uses [gosnmp](https://github.com/gosnmp/gosnmp) to read metrics from network-attached devices.

The configuration of SNMP is quite extensive due to it's flexibility.

## Configuration

- `type` has to be `snmp`
- `read_interval` as duration like '1s' or '20s' (default '30s')

For the receiver, the configuration is split in two parts:
### Target configuration

Each network-attached device that should be queried. A target consits of
- `hostname`
- `port` (default 161)
- `community` (default `public`)
- `timeout` as duration like '1s' or '20s' (default '1s')
- `version` SNMP version `X` (`X` in `1`, `2c`, `3`) (default `2c`)
- `type` to specify `type` tag for the target (default `node`)
- `type-id` to specify `type-id` tag for the target
- `stype` to specify `stype` tag (sub type) for the target
- `stype-id` to specify `stype-id` tag for the target

### Metric configuration
- `name` can be an OID or a user-given string
- `value` has to be an OID
- `unit` can be empty, an OID or a user-given string

If a OID is used for `name` or `unit`, the receiver will use the returned values to create the output metric. If there are any issues with the returned values, it uses the `OID`.

## Testing

For testing an SNMP endpoint and OIDs, you can use [`scripts/snmpReceiverTest`](../scripts/snmpReceiverTest)