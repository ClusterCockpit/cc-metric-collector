This folder contains the sinks for the cc-metric-collector.

# `sink.go`
The base class/configuration is located in `sink.go`.

# Sinks
There are currently two sinks shipped with the cc-metric-collector:
* `stdoutSink.go`: Writes all metrics to `stdout` in InfluxDB line protocol. The sink does not use https://github.com/influxdata/line-protocol to reduce the executed code for debugging
* `influxSink.go`: Writes all metrics to an InfluxDB database instance using a blocking writer. It uses https://github.com/influxdata/influxdb-client-go . Configuration for the server, port, user, password and database name are in the global configuration file

# Installation
Nothing to do, all sinks are pure Go code
