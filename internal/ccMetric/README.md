# ClusterCockpit metrics

As described in the [ClusterCockpit specifications](https://github.com/ClusterCockpit/cc-specifications), the whole ClusterCockpit stack uses metrics in the InfluxDB line protocol format. This is also the input and output format for the ClusterCockpit Metric Collector but internally it uses an extended format while processing, named CCMetric.

It is basically a copy of the [InfluxDB line protocol](https://github.com/influxdata/line-protocol) `MutableMetric` interface with one extension. Besides the tags and fields, it contains a list of meta information (re-using the `Tag` structure of the original protocol):

```golang
type ccMetric struct {
    name   string                 // Measurement name
    meta   map[string]string      // map of meta data tags
    tags   map[string]string      // map of of tags
    fields map[string]interface{} // map of of fields
    tm     time.Time              // timestamp
}

type CCMetric interface {
    ToPoint(metaAsTags map[string]bool) *write.Point  // Generate influxDB point for data type ccMetric
    ToLineProtocol(metaAsTags map[string]bool) string // Generate influxDB line protocol for data type ccMetric
    String() string                                   // Return line-protocol like string

    Name() string        // Get metric name
    SetName(name string) // Set metric name

    Time() time.Time     // Get timestamp
    SetTime(t time.Time) // Set timestamp

    Tags() map[string]string                   // Map of tags
    AddTag(key, value string)                  // Add a tag
    GetTag(key string) (value string, ok bool) // Get a tag by its key
    HasTag(key string) (ok bool)               // Check if a tag key is present
    RemoveTag(key string)                      // Remove a tag by its key

    Meta() map[string]string                    // Map of meta data tags
    AddMeta(key, value string)                  // Add a meta data tag
    GetMeta(key string) (value string, ok bool) // Get a meta data tab addressed by its key
    HasMeta(key string) (ok bool)               // Check if a meta data key is present
    RemoveMeta(key string)                      // Remove a meta data tag by its key

    Fields() map[string]interface{}                   // Map of fields
    AddField(key string, value interface{})           // Add a field
    GetField(key string) (value interface{}, ok bool) // Get a field addressed by its key
    HasField(key string) (ok bool)                    // Check if a field key is present
    RemoveField(key string)                           // Remove a field addressed by its key
}

func New(name string, tags map[string]string, meta map[string]string, fields map[string]interface{}, tm time.Time) (CCMetric, error)
func FromMetric(other CCMetric) CCMetric
func FromInfluxMetric(other lp.Metric) CCMetric
```

The `CCMetric` interface provides the same functions as the `MutableMetric` like `{Add, Get, Remove, Has}{Tag, Field}` and additionally provides `{Add, Get, Remove, Has}Meta`.

The InfluxDB protocol creates a new metric with `influx.New(name, tags, fields, time)` while CCMetric uses `ccMetric.New(name, tags, meta, fields, time)` where `tags` and `meta` are both of type `map[string]string`.

You can copy a CCMetric with `FromMetric(other CCMetric) CCMetric`. If you get an `influx.Metric` from a function, like the line protocol parser, you can use `FromInfluxMetric(other influx.Metric) CCMetric` to get a CCMetric out of it (see `NatsReceiver` for an example).

Although the [cc-specifications](https://github.com/ClusterCockpit/cc-specifications/blob/master/interfaces/lineprotocol/README.md) defines that there is only a `value` field for the metric value, the CCMetric still can have multiple values similar to the InfluxDB line protocol.
