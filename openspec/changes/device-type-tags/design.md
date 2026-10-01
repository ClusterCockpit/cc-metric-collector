# Design

## Context

See `proposal.md` for motivation and `specs/device-instance-tags/spec.md` for the
required tags and ids. Relevant facts:

- **The decoder needs no change.** Both ClusterCockpit metric stores (cc-backend's
  internal store and cc-metric-store, which imports the same decoder) build the
  storage path below the host as `<type><type-id>` for every type except `node`.
  With `type=node`, `stype`/`stype-id` are dropped. Unknown tags (`device`,
  `filesystem`, `port`, `lid`) are always ignored. Tagging instances as
  `type=filesystem|network` therefore needs no decoder change and works with
  the cc-backend version cc-metric-store pins today (v1.5.3).
- cc-backend's queries read `filesystem<id>` / `network<id>` for the ids declared
  in a subcluster's `topology.filesystems|networks[].id`, so ids must be stable.
- `lp.NewMessage` / `lp.NewMetric` copy the tag map, so collectors that mutate a
  shared tag map between messages (gpfs, beegfs) stay correct.
- Current tagging, per collector:

| Collector | Where tags are built | Today | Instance id source |
|---|---|---|---|
| netstat | `setup()`, per included interface, stored in `m.matches[canonical]` | `type=node,stype=network,stype-id=<raw>` | `raw` (system name); `canonical` from `getCanonicalName` |
| nfsiostat | `m.tags = {type: node}` plus `AddTag` per message in `Read` | `+ stype=filesystem,stype-id=<mntpoint>` | mount point |
| beegfs_meta, beegfs_storage | `m.tags = {type: node, filesystem: ""}`, `m.tags["filesystem"] = mountpoint` in `Read` | `type=node,filesystem=<mnt>` | mount point |
| gpfs | same pattern, `m.tags["filesystem"] = filesystem` (`_fs_`) | `type=node,filesystem=<fs>` | GPFS filesystem name |
| lustre | `m.tags = {type: node}`, `y.AddTag("device", device)` per message | `type=node,device=<llite>` | llite instance `<fsname>-<hex>` |
| infiniband | `tagSet` per device/port in `Init` | `type=node,device,port,lid` | device and port |

## Goals / Non-Goals

**Goals:**
- Every per-instance filesystem or network message carries
  `type=filesystem|network,type-id=<stable id>` and no `stype` tags.
- The changes are local: tag maps are set in the place where each collector
  builds them today.

**Non-Goals:**
- `diskstat`, `iostat`, `smartmon` (block devices, not filesystem/network scopes).
- GPU sub-devices (`nvidia`/`rocm` `stype=mig|nvlink|device` below
  `type=accelerator`), which already work.
- A configurable id format, or translating instances to mount points or netdevs.
- Documentation of the collectors (handled separately).

## Decisions

### D1: Follow the accelerator convention (`type`/`type-id`)

Use `type=filesystem|network,type-id=<id>`, the form the store already keeps per
instance. *Alternative:* teach the decoder to accept `type=node` + device
`stype`. Rejected: it needs changes in cc-backend plus a dependency bump in
cc-metric-store. lustre, gpfs, beegfs and infiniband would have to change their
tags anyway.

### D2: Drop `stype`/`stype-id`, keep informational tags

With a non-node `type`, the decoder appends `stype<stype-id>` as a second level
(`filesystem/x` → `filesystem/x/filesystem/x`), so the sub-type tags must go.
`device`, `filesystem`, `port` and `lid` are ignored by the store and may be used
by other sinks, so they stay.

### D3: Per-collector ids

- **netstat**: build the tags with `type-id` = `canonical`. The configured name
  stays the same even when a node reports an alias.
- **nfsiostat**: `m.tags = {type: filesystem}`. Replace the two `stype`/`stype-id`
  `AddTag` pairs with `AddTag("type-id", mntpoint)`. The existing option
  `use_server_as_stype` stays with its name: it keys the data by NFS server
  address, so `type-id` is then the server address. *Alternatives:* remove or
  rename the option. Rejected: the config decoder rejects unknown keys, so
  existing configs that set it would fail at startup.
- **beegfs_meta, beegfs_storage, gpfs**: initialise `m.tags` with
  `type: filesystem` and set `m.tags["type-id"]` right next to the existing
  `m.tags["filesystem"]` assignment.
- **lustre**: `m.tags = {type: filesystem}`. Per message, keep
  `AddTag("device", device)` and add `AddTag("type-id", lustreFsname(device))`.
  `lustreFsname` is a pure helper: split at the last `-`, and return the prefix
  only when the prefix is non-empty and the suffix is non-empty and entirely
  hexadecimal. Otherwise return the name unchanged.
  *Alternative:* map to the mount point via `/proc/mounts`. Rejected as an extra
  lookup with its own failure modes; the fsname is stable.
- **infiniband**: `tagSet` gets `type: network` and `type-id:
  infinibandTypeID(device, port)` = `device + ":" + port`. Built once in `Init`.
  *Alternative:* device only. Rejected: two ports of one HCA would overwrite each
  other.

Both helpers live in their collector files. Unit tests sit next to them. This
adds the first `_test.go` files in `collectors/`, which needs nothing beyond the
standard `go test`.

## Risks / Trade-offs

- **Breaking change for other sinks and dashboards** that select on `stype` or
  `type=node`. → Called out as BREAKING in the proposal (the release
  announcement is part of the separate documentation work). Informational tags
  stay.
- **Ids have to match the ids declared in cc-backend's `cluster.json`**: interface
  names, mount points, GPFS names, Lustre fsnames and `mlx5_0:1`-style ids. →
  The ids are fixed by the spec, and operators declare exactly these.
- **netstat and infiniband both report `type=network` with different ids**
  (`ib0` vs `mlx5_0:1`). Both appear in `topology.networks`. A query for a
  metric at an id the metric doesn't have returns no data for that id, which is
  harmless.
- **Lustre fsnames that end in a hexadecimal-looking segment** (e.g. `fs-beef`)
  would be truncated. → Lustre always appends the superblock suffix, so the
  helper strips exactly one segment. A real fsname that ends in `-<hex>` loses
  that segment consistently on every node, so the id is still stable.
- **Transition**: values already in a node buffer stay until they age out.
  Until then, node-scope reads in the store use that old buffer instead of
  summing the new per-instance data.

## Migration Plan

1. Release the collector change, marked BREAKING.
2. Operators declare the reported ids in `topology.filesystems` /
   `topology.networks` in cc-backend's `cluster.json`.
3. Rollback: deploy the previous collector. The store then writes into the node
   buffer again; per-instance data written in between ages out.
