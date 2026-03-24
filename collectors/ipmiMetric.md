<!--
---
title: IPMI Metric collector
description: Collect metrics using ipmitool or ipmi-sensors
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/ipmi.md
---
-->

## `ipmistat` collector

```json
  "ipmistat": {
    "ipmitool_path": "/path/to/ipmitool",
    "ipmisensors_path": "/path/to/ipmi-sensors",
    "use_sudo": true
  }
```

The `ipmistat` collector reads data from `ipmitool` (`ipmitool sensor`) or `ipmi-sensors` (`ipmi-sensors --sdr-cache-recreate --comma-separated-output`).

The metrics depend on the output of the underlying tools but contain temperature, power and energy metrics.

ipmitool and ipmi-sensors typically require root to run.
In order to cc-metric-collector without root priviliges, you can enable `use_sudo`.
Add a file like this in /etc/sudoers.d/ to allow cc-metric-collector to run this command:

```
# Do not log the following sudo commands from monitoring, since this causes a lot of log spam.
# However keep log_denied enabled, to detect failures
Defaults: monitoring !log_allowed, !pam_session

# Allow to use ipmitool and ipmi-sensors
monitoring ALL = (root) NOPASSWD:/usr/bin/ipmitool sensor
monitoring ALL = (root) NOPASSWD:/usr/sbin/ipmi-sensors --comma-separated-output --sdr-cache-recreate
```
