# ClusterCockpit metrics

As described in the [ClusterCockpit specifications](https://github.com/ClusterCockpit/cc-specifications), the whole ClusterCockpit stack uses metrics in the InfluxDB line protocol format. This is also the input and output format for the ClusterCockpit Metric Collector but internally it uses an extended format while processing, named CCMetric.

It is basically a copy of the [InfluxDB line protocol](https://github.com/influxdata/line-protocol) `MutableMetric` interface with one extension. Besides the tags and fields, it contains a list of meta information (re-using the `Tag` structure of the original protocol):

```
type ccMetric struct {
    name   string            // same as
    tags   []*influx.Tag     // original
    fields []*influx.Field   // Influx
    tm     time.Time         // line-protocol
    meta   []*influx.Tag
}

type CCMetric interface {
    influx.MutableMetric        // the same functions as defined by influx.MutableMetric
    RemoveTag(key string)       // this is not published by the original influx.MutableMetric
    Meta() map[string]string
    MetaList() []*lp.Tag
    AddMeta(key, value string)
    HasMeta(key string) bool
    GetMeta(key string) (string, bool)
    RemoveMeta(key string)
}
```

The `CCMetric` interface provides the same functions as the `MutableMetric` like `{Add, Remove, Has}{Tag, Field}` and additionally provides `{Add, Remove, Has}Meta`.

The InfluxDB protocol creates a new metric with `influx.New(name, tags, fields, time)` while CCMetric uses `ccMetric.New(name, tags, meta, fields, time)` where `tags` and `meta` are both of type `map[string]string`.

You can copy a CCMetric with `FromMetric(other CCMetric) CCMetric`. If you get an `influx.Metric` from a function, like the line protocol parser, you can use `FromInfluxMetric(other influx.Metric) CCMetric` to get a CCMetric out of it (see `NatsReceiver` for an example).
