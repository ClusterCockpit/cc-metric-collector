## `self` collector

```json
  "self": {
    "read_mem_stats" : true,
    "read_goroutines" : true,
    "read_cgo_calls" : true,
    "read_rusage" : true
  }
```

The `self` collector reads the data from the `runtime` and `syscall` packages, so it monitors the execution of the cc-metric-collector itself.

Metrics:
- If `read_mem_stats == true`:
  * `total_alloc`: The metric reports cumulative bytes allocated for heap objects.
  * `heap_alloc`: The metric reports bytes of allocated heap objects.
  * `heap_sys`: The metric reports bytes of heap memory obtained from the OS.
  * `heap_idle`: The metric reports bytes in idle (unused) spans.
  * `heap_inuse`: The metric reports bytes in in-use spans.
  * `heap_released`: The metric reports bytes of physical memory returned to the OS.
  * `heap_objects`: The metric reports the number of allocated heap objects.
- If `read_goroutines == true`:
  * `num_goroutines`: The metric reports the number of goroutines that currently exist.
- If `read_cgo_calls == true`:
  * `num_cgo_calls`: The metric reports the number of cgo calls made by the current process.
- If `read_rusage == true`:
  * `rusage_user_time`: The metric reports the amount of time that this process has been scheduled in user mode.
  * `rusage_system_time`: The metric reports the amount of time that this process has been scheduled in kernel mode.
  * `rusage_vol_ctx_switch`: The metric reports the amount of voluntary context switches.
  * `rusage_invol_ctx_switch`: The metric reports the amount of involuntary context switches.
  * `rusage_signals`: The metric reports the number of signals received.
  * `rusage_major_pgfaults`: The metric reports the number of major faults the process has made which have required loading a memory page from disk.
  * `rusage_minor_pgfaults`: The metric reports the number of minor faults the process has made which have not required loading a memory page from disk.
