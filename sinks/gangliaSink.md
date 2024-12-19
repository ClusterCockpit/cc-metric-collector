## `ganglia` sink

The `ganglia` sink uses the `gmetric` tool of the [Ganglia Monitoring System](http://ganglia.info/) to submit the metrics

### Configuration structure

```json
{
  "<name>": {
    "type": "ganglia",
    "gmetric_path" : "/path/to/gmetric",
    "add_ganglia_group" : true,
    "process_messages" : {
      "see" : "docs of message processor for valid fields"
    },
    "meta_as_tags" : []
  }
}
```

- `type`: makes the sink an `ganglia` sink
- `gmetric_path`: Path to `gmetric` executable (optional). If not given, the sink searches in `$PATH` for `gmetric`.
- `add_ganglia_group`: Add `--group=X` based on meta information to the `gmetric` call. Some old versions of `gmetric` do not support the `--group` option.
- `process_messages`: Process messages with given rules before progressing or dropping, see [here](../pkg/messageProcessor/README.md) (optional)
- `meta_as_tags`: print all meta information as tags in the output (deprecated, optional)