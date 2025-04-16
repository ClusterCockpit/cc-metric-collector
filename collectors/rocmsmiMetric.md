<!--
---
title: "ROCm SMI metric collector"
description: Collect metrics for AMD GPUs using the SMI library
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/rocmsmi.md
---
-->


## `rocm_smi` collector

```json
  "rocm_smi": {
    "exclude_devices": [
      "0","1", "0000000:ff:01.0"
    ],
    "exclude_metrics": [
      "rocm_mm_util",
      "rocm_temp_vrsoc"
    ],
    "use_pci_info_as_type_id": true,
    "add_pci_info_tag": false,
    "add_serial_meta": false,
  }
```

The `rocm_smi` collector can be configured to leave out specific devices with the `exclude_devices` option. It takes logical IDs in the list of available devices or the PCI address similar to NVML format (`%08X:%02X:%02X.0`). Metrics (listed below) that should not be sent to the MetricRouter can be excluded with the `exclude_metrics` option. 

The metrics sent by the `rocm_smi` collector use `accelerator` as `type` tag. For the `type-id`, it uses the device handle index by default. With the `use_pci_info_as_type_id` option, the PCI ID is used instead. If both values should be added as tags, activate the `add_pci_info_tag` option. It uses the device handle index as `type-id` and adds the PCI ID as separate `pci_identifier` tag.

Optionally, it is possible to add the serial to the meta informations. They are not sent to the sinks (if not configured otherwise).


Metrics:
* `rocm_gfx_util`
* `rocm_umc_util`
* `rocm_mm_util`
* `rocm_avg_power`
* `rocm_temp_mem`
* `rocm_temp_hotspot`
* `rocm_temp_edge`
* `rocm_temp_vrgfx`
* `rocm_temp_vrsoc`
* `rocm_temp_vrmem`
* `rocm_gfx_clock`
* `rocm_soc_clock`
* `rocm_u_clock`
* `rocm_v0_clock`
* `rocm_v1_clock`
* `rocm_d0_clock`
* `rocm_d1_clock`
* `rocm_temp_hbm`


Some metrics add the additional sub type tag (`stype`) like the `rocm_temp_hbm` metrics set `stype=device,stype-id=<HBM_slice_number>`. 
