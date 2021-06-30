This folder contains the sinks for the cc-metric-collector.

# `metricSink.go`
The base class/configuration is located in `metricSink.go`.

# Sinks
* `stdoutSink.go`: Writes all metrics to `stdout` in InfluxDB line protocol. The sink does not use https://github.com/influxdata/line-protocol to reduce the executed code for debugging
* `influxSink.go`: Writes all metrics to an InfluxDB database instance using a blocking writer. It uses https://github.com/influxdata/influxdb-client-go . Configuration for the server, port, ssl, password, database name and organisation are in the global configuration file. The 'password' is used for the token and the 'database' for the bucket. It uses the v2 API of Influx.
* `influxAsyncSink.go`: Writes all metrics to an InfluxDB database instance using a non-blocking writer. It uses https://github.com/influxdata/influxdb-client-go . Configuration for the server, port, ssl, password, database name and organisation are in the global configuration file. The 'password' is used for the token and the 'database' for the bucket. It uses the v2 API of Influx.
* `natsSink.go`: Sends all metrics to an NATS server using the InfluxDB line protocol as encoding. It uses https://github.com/nats-io/nats.go . Configuration for the server, port, user, password and database name are in the global configuration file. The database name is used as subject for the NATS messages.

# Installation
Nothing to do, all sinks are pure Go code

# Contributing own sinks
A sink contains three functions and is derived from the type `Sink` (in `metricSink.go`):
* `Init(config SinkConfig) error`
* `Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error`
* `Close()`

The data structures should be set up in `Init()` like opening a file or server connection. The `Write()` function takes a measurement, tags, fields and a timestamp and writes/sends the data. The `Close()` function should tear down anything created in `Init()`.

Finally, the sink needs to be registered in the `metric-collector.go`. There is a list of sinks called `Sinks` which is a map (string -> pointer to sink). Add a new entry with a descriptive name and the new sink.
