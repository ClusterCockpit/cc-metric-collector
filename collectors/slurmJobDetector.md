# `slurm` collector

```json
  "slurm": {
    "interval" : "1s",
    "send_job_events" : true,
    "send_job_metrics" : true,
    "send_step_events": false,
    "send_step_metrics" : false,
    "cgroup_version" : "v1"
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
* `cgroup_version`: Which cgroup version is in use. Possible values are `v1` and `v2`. `v1` is the default
* `sysfs_base`: (Testing only) Set the base path for lookups, default `/sys/fs/cgroup`.

For `cgroup_version = v2`, the collector searches for jobs at `<sysfs_base>/system.slice/slurmstepd.scope`, by default with `<sysfs_base>=/sys/fs/cgroup`. If the cgroup folders are created below `/sys/fs/cgroup/unified`, adjust the `sysfs_base` option to `/sys/fs/cgroup/unified`.

## Testing
For testing the collector, you can specifiy a different base directory that should be checked for new events. The default is `/sys/fs/cgroup/`. By specifying a `sysfs_base` in the configuration, this can be changed. Moreover, with the `slurmJobDetector_dummy.sh`, you can create and delete "jobs" for testing. Use the same directory with `--basedir`. It generates only cgroup/v1 directory structures at the moment.

```sh
$ slurmJobDetector_dummy.sh -h

Usage: slurmJobDetector_dummy.sh <opts>
       [ -h | --help ]
       [ -v | --verbosity ]
       [ -u | --uid <UID> (default: XXXX) ]
       [ -j | --jobid <JOBID> (default: random) ]
       [ -b | --basedir <JOBID> (default: ./slurm-test) ]
       [ -d | --delete ]
       [ -l | --list ]
```

With no options, it creates a job with the executing user's UID and a random JOBID. For deletion, use `-d -j JOBID`, deletion requires a JOBID. If you want to get a list of all UIDs and JOBIDs that currently exist, you can get the list with `-l`.