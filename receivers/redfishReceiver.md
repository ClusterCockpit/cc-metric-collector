## Redfish receiver

The Redfish receiver uses the [Redfish (specification)](https://www.dmtf.org/standards/redfish) to query thermal and power metrics. Thermal metrics may include various fan speeds and temperatures. Power metrics may include the current power consumption of various hardware components. It may also include the minimum, maximum and average power consumption of these components in a given time interval. The receiver will poll each configured redfish device once in a given interval. Multiple devices can be accessed in parallel to increase throughput.

### Configuration structure

```json
{
    "<redfish receiver name>": {
        "type": "redfish",
        "username": "<Username>",
        "password": "<Password>",
        "endpoint": "https://%h-bmc",
        "exclude_metrics": [ "min_consumed_watts" ],
        "client_config": [
            {
                "host_list": "n[1,2-4]"
            },
            {
                "host_list": "n5"
                "disable_power_metrics": true
            },
            {
                "host_list": "n6" ],
                "username": "<Username 2>",
                "password": "<Password 2>",
                "endpoint": "https://%h-BMC",
                "disable_thermal_metrics": true
            }
        ]
    }
}
```

Global settings:

- `fanout`: Maximum number of simultaneous redfish connections (default: 64)
- `interval`: How often the redfish power metrics should be read and send to the sink (default: 30 s)
- `http_insecure`: Control whether a client verifies the server's certificate (default: true == do not verify server's certificate)
- `http_timeout`: Time limit for requests made by this HTTP client (default: 10 s)

Global and per redfish device settings (per redfish device settings overwrite the global settings):

- `disable_power_metrics`: disable collection of power metrics
- `disable_processor_metrics`: disable collection of processor metrics
- `disable_thermal_metrics`: disable collection of thermal metrics
- `exclude_metrics`: list of excluded metrics
- `username`: User name to authenticate with
- `password`: Password to use for authentication
- `endpoint`: URL of the redfish service (placeholder `%h` gets replaced by the hostname)

Per redfish device settings:

- `host_list`: List of hosts with the same client configuration
