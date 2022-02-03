## `stdout` sink

The `stdout` sink is the most simple sink provided by cc-metric-collector. It writes all metrics in InfluxDB line-procol format to the configurable output file or the common special files `stdout` and `stderr`.


### Configuration structure

```json
{
  "<name>": {
    "type": "stdout",
    "meta_as_tags" : true,
    "output_file" : "mylogfile.log"
  }
}
```

- `type`: makes the sink an `stdout` sink
- `meta_as_tags`: print all meta information as tags in the output (optional)
- `output_file`: Write all data to the selected file (optional). There are two 'special' files: `stdout` and `stderr`. If this option is not provided, the default value is `stdout`


