## IPMI Receiver

The IPMI Receiver uses `ipmi-sensors` from the [FreeIPMI](https://www.gnu.org/software/freeipmi/) project to read IPMI sensor readings and sensor data repository (SDR) information. The available metrics depend on the sensors provided by the hardware vendor but typically contain temperature, fan speed, voltage and power metrics.

### Configuration structure

```json
{
    "<IPMI receiver name>": {
        "type": "ipmi",
        "interval": "30s",
        "fanout": 256,
        "username": "<Username>",
        "password": "<Password>",
        "endpoint": "ipmi-sensors://%h-p",
        "exclude_metrics": [ "fan_speed", "voltage" ],
        "client_config": [
            {
                "host_list": "n[1,2-4]"
            },
            {
                "host_list": "n[5-6]",
                "driver_type": "LAN",
                "cli_options": [ "--workaround-flags=..." ],
                "password": "<Password 2>"
            }
        ]
    }
}
```

Global settings:

- `interval`: How often the IPMI sensor metrics should be read and send to the sink (default: 30 s)

Global and per IPMI device settings (per IPMI device settings overwrite the global settings):

- `exclude_metrics`: list of excluded metrics e.g. fan_speed, power, temperature, utilization, voltage
- `fanout`: Maximum number of simultaneous IPMI connections (default: 64)
- `driver_type`: Out of band IPMI driver (default: LAN_2_0)
- `username`: User name to authenticate with
- `password`: Password to use for authentication
- `endpoint`: URL of the IPMI device (placeholder `%h` gets replaced by the hostname)

Per IPMI device settings:

- `host_list`: List of hosts with the same client configuration
- `cli_options`: Additional command line options for ipmi-sensors
