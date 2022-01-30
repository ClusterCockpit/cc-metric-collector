# CCMetric sinks

This folder contains the SinkManager and sink implementations for the cc-metric-collector.

# Configuration

The configuration file for the sinks is a list of configurations. The `type` field in each specifies which sink to initialize.

```json
[
  {
    "type" : "stdout",
    "meta_as_tags" : false
  },
  {
    "type" : "http",
    "host" : "localhost",
    "port" : "4123",
    "database" : "ccmetric",
    "password" : "<jwt token>"
  }
]
```

This example initializes two sinks, the `stdout` sink printing all metrics to the STDOUT and the `http` sink with the given `host`, `port`, `database` and `password`.

If `meta_as_tags` is set, all meta information attached to CCMetric are printed out as tags.

## Type `stdout`

```json
{
  "type" : "stdout",
  "meta_as_tags" : <true|false>
}
```

The `stdout` sink dumps all metrics to the STDOUT.

## Type `http`

```json
{
  "type" : "http",
  "host" : "<hostname>",
  "port" : "<portnumber>",
  "database" : "<database name>",
  "password" : "<jwt token>",
  "meta_as_tags" : <true|false>
}
```
The sink uses POST requests to send metrics to `http://<host>:<port>/<database>` using the JWT token as a JWT in the 'Authorization' header.

## Type `nats`

```json
{
  "type" : "nats",
  "host" : "<hostname>",
  "port" : "<portnumber>",
  "user" : "<username>",
  "password" : "<password>",
  "database" : "<database name>"
  "meta_as_tags" : <true|false>
}
```

This sink publishes the CCMetric in a NATS environment using `host`, `port`, `user` and `password` for connecting. The metrics are published using the topic `database`. 

## Type `influxdb`

```json
{
  "type" : "influxdb",
  "host" : "<hostname>",
  "port" : "<portnumber>",
  "user" : "<username>",
  "password" : "<password or API key>",
  "database" : "<database name>"
  "organization": "<InfluxDB v2 organization>",
  "ssl" : <true|false>,
  "meta_as_tags" : <true|false>
}
```

This sink submits the CCMetrics to an InfluxDB time-series database. It uses `host`, `port` and `ssl` for connecting. For authentification, it uses either `user:password` if `user` is set and only `password` as API key. The `organization` and `database` are used for writing to the correct database.

<<<<<<< HEAD
# Sinks
* `stdoutSink.go`: Writes all metrics to `stdout` in InfluxDB line protocol. The sink does not use https://github.com/influxdata/line-protocol to reduce the executed code for debugging
* `influxSink.go`: Writes all metrics to an InfluxDB database instance using a blocking writer. It uses https://github.com/influxdata/influxdb-client-go . Configuration for the server, port, ssl, password, database name and organisation are in the global configuration file. The 'password' is used for the token and the 'database' for the bucket. It uses the v2 API of Influx.
* `influxAsyncSink.go`: Writes all metrics to an InfluxDB database instance using a non-blocking writer. It uses https://github.com/influxdata/influxdb-client-go . Configuration for the server, port, ssl, password, database name and organisation are in the global configuration file. The 'password' is used for the token and the 'database' for the bucket. It uses the v2 API of Influx.
* `natsSink.go`: Sends all metrics to an NATS server using the InfluxDB line protocol as encoding. It uses https://github.com/nats-io/nats.go . Configuration for the server, port, user, password and database name are in the global configuration file. The database name is used as subject for the NATS messages.
* `httpSink.go`: Sends all metrics to an HTTP endpoint `http://<host>:<port>/<database>` using a POST request. The body of the request will consist of lines in the InfluxDB line protocol. In case password is specified, that password is used as a JWT in the 'Authorization' header.
=======
>>>>>>> develop


# Contributing own sinks
A sink contains three functions and is derived from the type `Sink`:
* `Init(config SinkConfig) error`
* `Write(point CCMetric) error`
* `Flush() error`
* `Close()`

The data structures should be set up in `Init()` like opening a file or server connection. The `Write()` function writes/sends the data. For non-blocking sinks, the `Flush()` method tells the sink to drain its internal buffers. The `Close()` function should tear down anything created in `Init()`.

Finally, the sink needs to be registered in the `sinkManager.go`. There is a list of sinks called `AvailableSinks` which is a map (`sink_type_string` -> `pointer to sink interface`). Add a new entry with a descriptive name and the new sink.
