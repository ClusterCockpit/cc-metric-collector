## `stdout` sink

The `stdout` sink is the most simple sink provided by cc-metric-collector. It writes all metrics in InfluxDB line-procol format to the configurable output file or the common special files `stdout` and `stderr`.


### Configuration structure

```json
{
  "<name>": {
    "type": "stdout",
    "meta_as_tags" : [],
    "output_file" : "mylogfile.log",
    "process_messages" : {
      "see" : "docs of message processor for valid fields"
    },
    "meta_as_tags" : []
  }
}
```

- `type`: makes the sink an `stdout` sink
- `meta_as_tags`: print meta information as tags in the output (optional)
- `output_file`: Write all data to the selected file (optional). There are two 'special' files: `stdout` and `stderr`. If this option is not provided, the default value is `stdout`
- `process_messages`: Process messages with given rules before progressing or dropping, see [here](../pkg/messageProcessor/README.md)  (optional)
- `meta_as_tags`: print all meta information as tags in the output (deprecated, optional)

