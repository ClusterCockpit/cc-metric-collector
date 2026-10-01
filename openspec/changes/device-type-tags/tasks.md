# Tasks

## 1. Helpers and tests

- [x] 1.1 Add `lustreFsname(llite string) string` to `collectors/lustreMetric.go` (strip the last `-` segment only when prefix and suffix are non-empty and the suffix is all hexadecimal) and verify with a table test in `collectors/lustreMetric_test.go` covering `scratch-ffff9a6e4c1bc800` → `scratch`, `my-fs` → `my-fs`, `a-b-ffff0001` → `a-b`, `noseparator` → `noseparator`, `-ffff` → `-ffff` and `fs-` → `fs-`; `go test ./collectors/` passes
- [x] 1.2 Add `infinibandTypeID(device, port string) string` to `collectors/infinibandMetric.go` and verify with a test in `collectors/infinibandMetric_test.go` that `("mlx5_0", "1")` yields `mlx5_0:1`; `go test ./collectors/` passes

## 2. Filesystem collectors

- [x] 2.1 nfsiostat: set `m.tags` to `type: filesystem` and replace both `stype`/`stype-id` `AddTag` pairs in `Read` with `AddTag("type-id", mntpoint)`; keep the `use_server_as_stype` option unchanged (it then makes `type-id` the server address); verify by reading the code that no `stype`/`stype-id` tag remains in `collectors/nfsiostatMetric.go` and that `go build ./...` succeeds
- [x] 2.2 beegfs_meta and beegfs_storage: initialise `m.tags` with `type: filesystem` and set `m.tags["type-id"] = mountpoint` next to `m.tags["filesystem"]`; verify that `go build ./...` succeeds and that every message created in the per-mount loop uses `m.tags`
- [x] 2.3 gpfs: initialise `m.tags` with `type: filesystem` and set `m.tags["type-id"] = filesystem` next to `m.tags["filesystem"]`; verify that `go build ./...` succeeds and that all GPFS metrics, including `gpfs_*_total`, are created after that assignment
- [x] 2.4 lustre: set `m.tags` to `type: filesystem` and add `AddTag("type-id", lustreFsname(device))` next to the existing `AddTag("device", device)`; verify that `go build ./...` succeeds

## 3. Network collectors

- [x] 3.1 netstat: build the per-interface tags as `type: network`, `type-id: canonical`, without `stype`/`stype-id`; verify by reading the code that no `stype` remains in `collectors/netstatMetric.go` and that `go build ./...` succeeds
- [x] 3.2 infiniband: build `tagSet` with `type: network` and `type-id: infinibandTypeID(device, port)`, keeping `device`, `port` and `lid`; verify that `go build ./...` succeeds

## 4. Verification

- [x] 4.1 Run `go build ./...`, `make vet` and `go test ./collectors/` and verify all succeed
- [x] 4.2 Verify with `grep -n "stype\|\"type\": *\"node\"" collectors/{netstat,nfsiostat,beegfsmeta,beegfsstorage,gpfs,lustre,infiniband}Metric.go` that none of the seven collectors tags per-instance metrics with `stype` or `type=node` any more
- [ ] 4.3 On a Linux node with the relevant filesystems or HCAs, run `cc-metric-collector -once` with a stdout sink and verify that each enabled collector prints lines tagged `type=filesystem` or `type=network` with the `type-id` the spec requires, and without `stype`
- [ ] 4.4 Optional end to end: declare the reported ids in a cc-backend subcluster's `topology.filesystems` / `topology.networks`, send the collector output to cc-backend (`/api/write` or NATS), and verify that a running job's `jobMetrics` returns one series per id at `filesystem`/`network` scope and their sum at `node` scope
