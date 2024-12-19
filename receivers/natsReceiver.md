## `nats` receiver

The `nats` receiver can be used receive metrics from the NATS network. The `nats` receiver subscribes to the topic `database` and listens on `address` and `port` for metrics in the InfluxDB line protocol.

### Configuration structure

```json
{
  "<name>": {
    "type": "nats",
    "address" : "nats-server.example.org",
    "port" : "4222",
    "subject" : "subject",
    "user": "natsuser",
    "password": "natssecret",
    "nkey_file": "/path/to/nkey_file"
  }
}
```

- `type`: makes the receiver a `nats` receiver
- `address`: Address of the NATS control server
- `port`: Port of the NATS control server
- `subject`: Subscribes to this subject and receive metrics
- `user`: Connect to nats using this user
- `password`: Connect to nats using this password
- `nkey_file`: Path to credentials file with NKEY

### Debugging

- Install NATS server and command line client
- Start NATS server

  ```bash
  nats-server --net nats-server.example.org --port 4222
  ```

- Check NATS server works as expected

  ```bash
  nats --server=nats-server-db.example.org:4222 server check
  ```

- Use NATS command line client to subscribe to all messages

  ```bash
  nats --server=nats-server-db.example.org:4222 sub ">"
  ```

- Use NATS command line client to send message to NATS receiver

  ```bash
  nats --server=nats-server-db.example.org:4222 pub subject \
  "myMetric,hostname=myHost,type=hwthread,type-id=0,unit=Hz value=400000i 1694777161164284635
  myMetric,hostname=myHost,type=hwthread,type-id=1,unit=Hz value=400001i 1694777161164284635"
  ```
