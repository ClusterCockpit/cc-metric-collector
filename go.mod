module github.com/ClusterCockpit/cc-metric-collector

go 1.23.0

toolchain go1.23.2

require (
	github.com/ClusterCockpit/cc-lib v0.0.0-20250224161927-9edac91bf47a
	github.com/ClusterCockpit/cc-units v0.4.0
	github.com/ClusterCockpit/go-rocm-smi v0.3.0
	github.com/NVIDIA/go-nvml v0.12.0-2
	github.com/PaesslerAG/gval v1.2.2
	github.com/expr-lang/expr v1.16.9
	github.com/fsnotify/fsnotify v1.7.0
	github.com/gorilla/mux v1.8.1
	github.com/influxdata/influxdb-client-go/v2 v2.14.0
	github.com/influxdata/line-protocol v0.0.0-20210922203350-b1ad95c89adf
	github.com/influxdata/line-protocol/v2 v2.2.1
	github.com/nats-io/nats.go v1.39.0
	github.com/prometheus/client_golang v1.20.5
	github.com/stmcginnis/gofish v0.15.0
	github.com/tklauser/go-sysconf v0.3.13
	golang.design/x/thread v0.0.0-20210122121316-335e9adffdf1
	golang.org/x/exp v0.0.0-20250215185904-eff6e970281f
	golang.org/x/sys v0.28.0
)

require (
	github.com/ClusterCockpit/cc-backend v1.4.2 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nats-io/nkeys v0.4.9 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/oapi-codegen/runtime v1.1.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/santhosh-tekuri/jsonschema/v5 v5.3.1 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/tklauser/numcpus v0.7.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/net v0.31.0 // indirect
	google.golang.org/protobuf v1.35.2 // indirect
)

replace github.com/ClusterCockpit/cc-lib => ../cc-lib
