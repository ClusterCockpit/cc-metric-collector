<!--
---
title: Lenovo Dense Power collector
description: Collect power of Lenovo Dense machines using ipmitool raw
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/lenovoDensePower.md
---
-->

## `lenovo_dense_power` collector

```json
  "lenovo_dense_power": {
    "ipmitool_path": "/path/to/ipmitool",
    "use_sudo": true,
    "include_metrics" : []
  }
```

The `lenovo_dense_power` collector reads power from Lenovo Dense machines via `ipmitool` (`ipmitool raw ...`). This collector is known to only work on the following machines. Others are not supported, so use at your *OWN* risk:

System                                    | Compatibility             | Power measured
---                                       | ---                       | ---
Lenovo ThinkSystem SD650 V2/V3            | untested, but should work | node + riser1 + riser2
Lenovo ThinkSystem SD650-N V2/ SD650-I V3 | untested, but should work | node + riser1 + riser2 + gpus
Lenovo ThinkSystem SD665-N V3             | tested, known to work     | node + riser1 + riser2 + gpus

To test if your system *may* be supported, you can run the following command *at your own risk*:

```
ipmitool raw 58 50 4 2 0 0 0
```

`ipmitool` typically require root to run.
In order to run `cc-metric-collector` without root priviliges, you can enable `use_sudo`.
Add a file like this in `/etc/sudoers.d/` to allow `cc-metric-collector` to run the required commands:

```
# Do not log the following sudo commands from monitoring, since this causes a lot of log spam.
# However keep log_denied enabled, to detect failures
Defaults: monitoring !log_allowed, !pam_session

# Allow to use ipmitool for Lenovo power readings
monitoring ALL = (root) NOPASSWD:/usr/bin/ipmitool raw 58 50 4 2 0 0 0
```
