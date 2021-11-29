This folder contains the sinks for the cc-metric-collector.

# `metricSink.go`
The base class/configuration is located in `metricSink.go`.

# Sinks
* `stdoutSink.go`: Writes all metrics to `stdout` in InfluxDB line protocol. The sink does not use https://github.com/influxdata/line-protocol to reduce the executed code for debugging
* `influxSink.go`: Writes all metrics to an InfluxDB database instance using a blocking writer. It uses https://github.com/influxdata/influxdb-client-go . Configuration for the server, port, ssl, password, database name and organisation are in the global configuration file. The 'password' is used for the token and the 'database' for the bucket. It uses the v2 API of Influx.
* `natsSink.go`: Sends all metrics to an NATS server using the InfluxDB line protocol as encoding. It uses https://github.com/nats-io/nats.go . Configuration for the server, port, user, password and database name are in the global configuration file. The database name is used as subject for the NATS messages.
* `httpSink.go`: Sends all metrics to an HTTP endpoint `http://<host>:<port>/<database>` using a POST request. The body of the request will consist of lines in the InfluxDB line protocol. In case password is specified, that password is used as a JWT in the 'Authorization' header.

# Installation
Nothing to do, all sinks are pure Go code

# Sink configuration

```json
  "sink": {
    "user": "testuser",
    "password": "testpass",
    "host": "127.0.0.1",
    "port": "9090",
    "database": "testdb",
    "organization": "testorg",
    "ssl": false
    "type": "stdout"
  }
```

## `stdout`
When configuring `type = stdout`, all metrics are printed to stdout. No further configuration is required or touched, so you can leave your other-sink-config in there and just change the `type` for debugging purposes

## `influxdb`
The InfluxDB sink uses blocking write operations to write to an InfluxDB database using the v2 API. It uses the following configuration options:
* `host`: Hostname of the database instance
* `port`: Portnumber (as string) of the database
* `database`: Name of the database, called 'bucket' in InfluxDB v2
* `organization`: The InfluxDB v2 API uses organizations to separate database instances running on the same host
* `ssl`: Boolean to activate SSL/TLS
* `user`: Although the v2 API uses API keys instead of username and password, this field can be used if the sink should authentificate with `username:password`. If you want to use an API key, leave this field empty.
* `password`: API key for the InfluxDB v2 API or password if `user` is set

## `nats`
* `host`: Hostname of the NATS server
* `port`: Portnumber (as string) of the NATS server
* `user`: Username for authentification in the NATS transport system
* `password`: Password for authentification in the NATS transport system

## `http`
* `host`: Hostname of the HTTP server
* `port`: Portnumber (as string) of the HTTP server
* `database`: Endpoint to write to. HTTP POST requests are performed on `http://<host>:<port>/<database>`
* `password`: JSON Web token used for authentification


# Contributing own sinks
A sink contains three functions and is derived from the type `Sink` (in `metricSink.go`):
* `Init(config SinkConfig) error`
* `Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error`
* `Flush() error`
* `Close()`

The data structures should be set up in `Init()` like opening a file or server connection. The `Write()` function takes a measurement, tags, fields and a timestamp and writes/sends the data. For non-blocking sinks, the `Flush()` method tells the sink to drain its internal buffers. The `Close()` function should tear down anything created in `Init()`.

Finally, the sink needs to be registered in the `metric-collector.go`. There is a list of sinks called `Sinks` which is a map (sink_type_string -> pointer to sink). Add a new entry with a descriptive name and the new sink.
