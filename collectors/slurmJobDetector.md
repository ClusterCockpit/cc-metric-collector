# `slurm` collector

```json
  "slurm": {
    "interval" : "1s",
    "send_job_events" : true,
    "send_job_metrics" : true,
    "send_step_events": false,
    "send_step_metrics" : false,
  }
```

The `slurm` collector reads the data from `/sys/fs/cgroup/` to detect the creation and deletion of SLURM jobs on the node. Then detecting an event, it collects some event related information and sends the event. The event detection happens every `interval`.

Additionally, for all running jobs, is can collect metrics and send them out. This collection is done in the global collector interval.

Options:
* `interval`: Time interval in which the folders are checked for new or vanished SLURM jobs
* `send_job_events`: Send events when a job starts or ends
* `send_job_metrics`: Send metrics of each running job with the global collector interval
* `send_step_events`: Send events when a job step starts
* `send_step_metrics`: Send metrics of each job step with the global collector interval

Testing options:
For testing the collector, you can specifiy a different base directory that should be checked for new events. The default is `/sys/fs/cgroup/`. By specifying a `sysfs_base` in the configuration, this can be changed. Moreover, with the `slurmJobDetector_dummy.sh`, you can create and delete "jobs" for testing.