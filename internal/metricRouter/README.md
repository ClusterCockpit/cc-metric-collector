# CC Metric Router

The CCMetric router sits in between the collectors and the sinks and can be used to add and remove tags to/from traversing [CCMetrics](../ccMetric/README.md).

# Configuration

```json
{
    "add_tags" : [
        {
            "key" : "cluster",
            "value" : "testcluster",
            "if" : "*"
        },
        {
            "key" : "test",
            "value" : "testing",
            "if" : "name == 'temp_package_id_0'"
        }
    ],
    "delete_tags" : [
        {
            "key" : "unit",
            "value" : "*",
            "if" : "*"
        }
    ],
    "interval_timestamp" : true
}
```

There are three main options `add_tags`, `delete_tags` and `interval_timestamp`. `add_tags` and `delete_tags` are lists consisting of dicts with `key`, `value` and `if`. The `value` can be omitted in the `delete_tags` part as it only uses the `key` for removal. The `interval_timestamp` setting means that a unique timestamp is applied to all metrics traversing the router during an interval.

# Conditional manipulation of tags

The `if` setting allows conditional testing of a single metric like in the example:

```json
{
    "key" : "test",
    "value" : "testing",
    "if" : "name == 'temp_package_id_0'"
}
```

If the CCMetric name is equal to 'temp_package_id_0', it adds an additional tag `test=testing` to the metric.

In order to match all metrics, you can use `*`, so in order to add a flag per default, like the `cluster=testcluster` tag in the example.


