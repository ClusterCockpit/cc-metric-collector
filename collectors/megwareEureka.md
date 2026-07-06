<!--
---
title: Megware Eureka collector
description: Collect power and other metrics of Megware Eureka machines using u20
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/lenovoDensePower.md
---
-->

## `lenovo_dense_power` collector

```json
  "megware_eureka": {
    "u20_path": "/path/to/ipmitool",
    "use_sudo": true
  }
```

The `megware_eureka` collector reads power and other metrics from Megware Eureka machines via `u20`.
If the u20 tool is available for your machines, it's possible that this collector is compatible.
If you don't have access to u20, please contact your Megware sales person.

You can test the collector compatibility by running the following command:

```
$ u20 values --read GET_MPS_POLL_VALUES
```

If this returns a JSON with energy, voltage and current readings, you're fine.

In addition, `u20` typically requires root to run.
In order to run `cc-metric-collector` without root priviliges, you can enable `use_sudo`.
Add a file like this in `/etc/sudoers.d/` to allow `cc-metric-collector` to run the required commands:

```
# Do not log the following sudo commands from monitoring, since this causes a lot of log spam.
# However keep log_denied enabled, to detect failures
Defaults: monitoring !log_allowed, !pam_session

# Allow to use u20 for Megware power readings
monitoring ALL = (root) NOPASSWD:/usr/bin/u20 values --read GET_MPS_POLL_VALUES
```
