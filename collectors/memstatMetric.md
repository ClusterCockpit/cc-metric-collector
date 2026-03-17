<!--
---
title: Memory statistics metric collector
description: Collect metrics from `/proc/meminfo`
categories: [cc-metric-collector]
tags: ['Admin']
weight: 2
hugo_path: docs/reference/cc-metric-collector/collectors/memstat.md
---
-->


## `memstat` collector

```json
  "memstat": {
    "exclude_metrics": [
      "mem_used"
    ]
  }
```

The `memstat` collector reads data from `/proc/meminfo` and outputs a handful **node** metrics. If a metric is not required, it can be excluded from forwarding it to the sink.


Metrics:
* `mem_total`
* `mem_sreclaimable`
* `mem_slab`
* `mem_free`
* `mem_buffers`
* `mem_cached`
* `mem_available`
* `mem_shared`
* `mem_active`
* `mem_inactive`
* `mem_dirty`
* `mem_writeback`
* `mem_anon_pages`
* `mem_mapped`
* `mem_vmalloc_total`
* `mem_anon_hugepages`
* `mem_shared_hugepages`
* `mem_shared_pmd_mapped`
* `mem_hugepages_total`
* `mem_hugepages_free`
* `mem_hugepages_reserved`
* `mem_hugepages_surplus`
* `mem_hugepages_size`
* `mem_direct_mapped_4k`
* `mem_direct_mapped_2m`
* `mem_direct_mapped_4m`
* `mem_direct_mapped_1g`
* `mem_locked`
* `mem_pagetables`
* `mem_kernelstack`
* `swap_total`
* `swap_free`
* `mem_used` = `mem_total` - (`mem_free` + `mem_buffers` + `mem_cached` + `mem_shared`)

