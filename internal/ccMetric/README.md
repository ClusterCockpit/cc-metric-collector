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
```

The `CCMetric` interface provides the same functions as the `MutableMetric` like `{Add, Remove, Has}{Tag, Field}` and additionally provides `{Add, Remove, Has}Meta`

