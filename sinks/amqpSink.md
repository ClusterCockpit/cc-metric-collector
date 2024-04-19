## `amqp` sink

The `amqp` sink publishes all metrics into a RabbitMQ network. The publishing key is the queue name in the configuration file

### Configuration structure

```json
{
  "<name>": {
    "type": "amqp",
    "queue_name" : "myqueue",
    "batch_size" : 1000,
    "flush_delay": "4s",
    "publish_timeout": "1s",
    "host": "dbhost.example.com",
    "port": 5672,
    "username": "exampleuser",
    "password" : "examplepw",
    "meta_as_tags" : [],
  }
}
```

- `type`: makes the sink an `amqp` sink, also `rabbitmq` is allowed as alias
- `queue_name`: All metrics are published to this queue
- `host`: Hostname of the RabbitMQ server
- `port`: Port number of the RabbitMQ server
- `username`: Username for basic authentication
- `password`: Password for basic authentication
- `meta_as_tags`: print all meta information as tags in the output (optional)
- `publish_timeout`: Timeout for each publication operation (default `1s`)
- `flush_delay`: Group metrics coming in to a single batch (default `4s`)
- `batch_size`: Maximal batch size. If `batch_size` is reached before the end of `flush_delay`, the metrics are sent without further delay (default: `1000`)