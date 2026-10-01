# Proposal

## Why

Filesystem and network collectors report one value per mount point, filesystem
or interface, but tag it `type=node` plus a distinguishing tag (`stype`/`stype-id`,
`device` or `filesystem`). The ClusterCockpit metric store ignores those tags
below `type=node`, so all instances of a metric are written into the same node
buffer and overwrite each other. cc-backend's new per-instance scopes
(`filesystem`, `network`) therefore find no data, and node values are whatever
instance was written last. Accelerator metrics already work, because they are
tagged `type=accelerator,type-id=<id>`.

## What Changes

- The netstat, nfsiostat, beegfs_meta, beegfs_storage, gpfs, lustre and
  infiniband collectors tag every per-instance metric with
  `type=filesystem,type-id=<id>` or `type=network,type-id=<id>`, following the
  accelerator convention.
- Each collector uses a stable instance id, so it can match the ids declared in
  cc-backend's `cluster.json` topology:
  - netstat: the canonical interface name
  - nfsiostat and beegfs: the mount point (nfsiostat: the NFS server address
    when `use_server_as_stype` is enabled)
  - gpfs: the GPFS filesystem name
  - lustre: the filesystem name without the per-mount suffix
  - infiniband: `<device>:<port>`
- **BREAKING**: netstat and nfsiostat no longer send `stype`/`stype-id`. All seven
  collectors stop sending `type=node` for per-instance metrics. Consumers that
  select on those tags (other sinks, dashboards) have to switch to
  `type`/`type-id`.
- The informational tags `device`, `filesystem`, `port` and `lid` stay as they are.

Out of scope: the metric store's line decoder (no change needed), other
collectors that tag devices (`diskstat`, `iostat`, `smartmon`), and the
collector documentation, which is updated separately.

## Capabilities

### New Capabilities
- `device-instance-tags`: how collectors tag per-instance filesystem and
  network metrics, and which stable instance id each collector reports.

### Modified Capabilities
<!-- None: no specs exist yet under openspec/specs/. -->

## Impact

- **Code**: `collectors/netstatMetric.go`, `nfsiostatMetric.go`,
  `beegfsmetaMetric.go`, `beegfsstorageMetric.go`, `gpfsMetric.go`,
  `lustreMetric.go`, `infinibandMetric.go`.
- **Consumers**: the cc-backend internal metric store and cc-metric-store store
  each instance at the device level without changes. Other sinks see changed tags.
- **Operations**: cc-backend's `cluster.json` must declare the reported ids in
  `topology.filesystems` / `topology.networks`. Data already stored in a node
  buffer ages out after the change.
