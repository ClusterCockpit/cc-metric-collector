# Spec Delta

## Purpose

Defines how collectors tag metrics that are reported once per filesystem or
network instance, so that metric stores keep each instance separate and
consumers can match it to a statically declared instance id.

## ADDED Requirements

### Requirement: Device type tags for filesystem instances
Every metric that a collector reports per filesystem instance SHALL carry the
tags `type=filesystem` and `type-id=<id>`, where `<id>` identifies the instance.
Such a metric SHALL NOT carry `type=node`. This applies to the nfsiostat,
beegfs_meta, beegfs_storage, gpfs and lustre collectors.

#### Scenario: Two mounted filesystems
- **WHEN** the nfsiostat collector reports `nfsio_nread` for the mount points
  `/home` and `/scratch`
- **THEN** it emits two messages, one tagged `type=filesystem,type-id=/home` and
  one tagged `type=filesystem,type-id=/scratch`

#### Scenario: No node type on filesystem metrics
- **WHEN** any of these collectors reports a per-filesystem metric
- **THEN** the message has no `type=node` tag

### Requirement: Device type tags for network instances
Every metric that a collector reports per network instance SHALL carry the tags
`type=network` and `type-id=<id>`, where `<id>` identifies the instance. Such a
metric SHALL NOT carry `type=node`. This applies to the netstat and infiniband
collectors.

#### Scenario: Two network interfaces
- **WHEN** the netstat collector reports `net_bytes_in` for the included
  interfaces `eth0` and `ib0`
- **THEN** it emits one message tagged `type=network,type-id=eth0` and one tagged
  `type=network,type-id=ib0`

### Requirement: No sub-type tags on device instance metrics
Per-instance filesystem and network metrics SHALL NOT carry `stype` or
`stype-id` tags.

#### Scenario: Former sub-type tags are gone
- **WHEN** the netstat or nfsiostat collector reports a per-instance metric
- **THEN** the message has neither an `stype` nor an `stype-id` tag

### Requirement: Stable instance ids
The `type-id` of a per-instance metric SHALL be stable across nodes and across
remounts or reboots, so that it can match a statically declared id. Each
collector SHALL use the following id:

| Collector | `type-id` |
|---|---|
| netstat | the canonical interface name, i.e. the `include_devices` entry, also when the system reports an alias |
| nfsiostat | the mount point, or the NFS server address when `use_server_as_stype` is enabled |
| beegfs_meta, beegfs_storage | the mount point |
| gpfs | the GPFS filesystem name |
| lustre | the filesystem name, i.e. the llite instance name without its trailing `-<hexadecimal>` suffix |
| infiniband | `<device>:<port>`, e.g. `mlx5_0:1` |

#### Scenario: Interface reported under an alias
- **WHEN** `include_devices` lists `ib0`, `interface_aliases` lists `ibp65s0` as
  an alias of `ib0`, and the system reports the interface as `ibp65s0`
- **THEN** netstat metrics for that interface carry `type-id=ib0`

#### Scenario: NFS keyed by server address
- **WHEN** the nfsiostat collector is configured with `use_server_as_stype: true`
  and reports a metric for the mount `/home` served by `nfs1:/export/home`
- **THEN** the message carries `type=filesystem,type-id=nfs1:/export/home`

#### Scenario: Lustre mounted again
- **WHEN** the Lustre filesystem `scratch` appears as llite instance
  `scratch-ffff9a6e4c1bc800` and, after a remount, as `scratch-ffff8b2d1a0e4000`
- **THEN** its metrics carry `type-id=scratch` in both cases

#### Scenario: Lustre name without a hexadecimal suffix
- **WHEN** the llite instance name is `my-fs` (its last `-` segment is not
  hexadecimal)
- **THEN** its metrics carry `type-id=my-fs`

#### Scenario: Two ports of one HCA
- **WHEN** the infiniband collector reports metrics for ports 1 and 2 of device
  `mlx5_0`
- **THEN** the metrics carry `type-id=mlx5_0:1` and `type-id=mlx5_0:2`
  respectively

### Requirement: Informational tags are kept
Collectors SHALL keep the informational tags they send today (`device`,
`filesystem`, `port`, `lid`), in addition to `type`/`type-id`.

#### Scenario: Infiniband tags
- **WHEN** the infiniband collector reports a metric for port 1 of `mlx5_0`
  with LID `0x12`
- **THEN** the message carries `type=network,type-id=mlx5_0:1`,
  `device=mlx5_0`, `port=1` and `lid=0x12`
