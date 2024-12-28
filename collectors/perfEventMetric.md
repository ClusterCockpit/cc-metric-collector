# `perf_event` collector

This collector uses directly the `perf_event_open` system call to measure events. There is no name to event translation, the configuration has to be as low-level as required by the system call. It allows to aggregate the measurements to topological entities like socket or the whole node.

## Configuration

```json
{
    "events" : [
        {
            "name" : "instructions",
            "unit" : "uncore_imc_0",
            "config": "0x01",
            "scale_file" : "/sys/devices/<unit>/events/<event>.scale",
            "per_hwthread": true,
            "per_socket": true,
            "exclude_kernel": true,
            "exclude_hypervisor": true,
            "tags": {
                "tags": "just_for_the_event"
            },
            "meta": {
                "meta_info": "just_for_the_event"
            },
            "config1": "0x00",
            "config2": "0x00",
        }
    ]
}
```

- `events`: List of events to measure
- `name`: Name for the metric
- `unit`: Unit of the event or `cpu` if not given. The unit type ID is resolved by reading the file `/sys/devices/<unit>/type`. The unit type ID is then written to the `perf_event_attr` struct member `type`.
- `config`: Hex value written to the `perf_event_attr` struct member `config`.
- `config1`: Hex value written to the `perf_event_attr` struct member `config1` (optional).
- `config2`: Hex value written to the `perf_event_attr` struct member `config1` (optional).
- `scale_file`: If a measurement requires scaling, like the `power` unit aka RAPL, it is provided by the kernel in a `.scale` file at `/sys/devices/<unit>/events/<event>.scale`.
- `exclude_kernel`: Exclude the kernel from measurements (default: `true`). It sets the `perf_event_attr` struct member `exclude_kernel`.
- `exclude_hypervisor`: Exclude the hypervisors from measurements (default: `true`). It sets the `perf_event_attr` struct member `exclude_hypervisor`.
- `per_hwthread`: Generate metrics per hardware thread (default: `false`)
- `per_socket`: Generate metrics per hardware thread (default: `false`)
- `tags`: Tags just for the event.
- `meta`: Meta information just for the event, often a `unit`