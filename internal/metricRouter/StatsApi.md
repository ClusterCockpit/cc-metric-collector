# Stats API

The Stats API can be used for debugging. It publishes counts at an HTTP endpoint as JSON from different componenets of the CC Metric Collector.

# Configuration

The Stats API has an own configuration file to specify the listen host and port. The defaults are `localhost` and `8080`.

```json
{
  "bindhost" : "",
  "port" : "8080",
  "publish_collectorstate" : true
}
```

The `bindhost` and `port` can be used to specify the listen host and port. The `publish_collectorstate` needs to be `true`, otherwise nothing is presented. This option is for future use if we need to publish more infos using different domains.