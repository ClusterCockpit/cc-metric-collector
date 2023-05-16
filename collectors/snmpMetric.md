
## `snmpstat` collector

```json
  "snmpstat": {
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

The `snmpstat` collector uses [gosnmp](https://github.com/gosnmp/gosnmp) to read metrics from network-attached devices.

The configuration of SNMP is quite extensive due to it's flexibility. For the collector, the configuration is split in two parts:

### Target configuration

Each network-attached device that should be queried. A target consits of
- `hostname`
- `port` (default 161)
- `community` (default 'public')
- `timeout` in seconds (default 1 for 1 second)

### Metric configuration
- `name` can be an OID or a user-given string
- `value` has to be an OID
- `unit` can be empty, an OID or a user-given string

