# Message Processor Component

Multiple parts of in the ClusterCockit ecosystem require the processing of CCMessages.
The main CC application using it is `cc-metric-collector`. The processing part there was originally in the metric router, the central
hub connecting collectors (reading local data), receivers (receiving remote data) and sinks (sending data). Already in early stages, the
lack of flexibility caused some trouble:

> The sysadmins wanted to keep operating their Ganglia based monitoring infrastructure while we developed the CC stack. Ganglia wants the core metrics with
> a specific name and resolution (right unit prefix) but there was no conversion of the data in the CC stack, so CC frontend developers wanted a different
> resolution for some metrics. The issue was basically the `mem_used` metric showing the currently used memory of the node. Ganglia wants it in `kByte` as provided
> by the Linux operating system but CC wanted it in `GByte`.

## For developers

Whenever you receive or are about to send a message out, you should provide some processing.

### Configuration of component

New operations can be added to the message processor at runtime. Of course, they can also be removed again. For the initial setup, having a configuration file
or some fields in a configuration file for the processing.

The message processor uses the following configuration
```golang
type messageProcessorConfig struct {
	DropMessages     []string          `json:"drop_messages"`      // List of metric names to drop. For fine-grained dropping use drop_messages_if
	DropMessagesIf   []string          `json:"drop_messages_if"`   // List of evaluatable terms to drop messages
	RenameMessages   map[string]string `json:"rename_messages"`    // Map to rename metric name from key to value
	NormalizeUnits   bool              `json:"normalize_units"`    // Check unit meta flag and normalize it using cc-units
	ChangeUnitPrefix map[string]string `json:"change_unit_prefix"` // Add prefix that should be applied to the messages
}
```

In order to load the configuration from a `json.RawMessage`:
```golang
mp, _ := NewMessageProcessor()

mp.FromConfigJSON(configJson)
```

### Using the component
After initialization and adding the different operations, the `ProcessMessage()` function applies all operations and returns whether the message should be dropped.

```golang
m := lp.CCMetric{}

drop, err := mp.ProcessMessage(m)
if !drop {
    // process further
}
```

#### Overhead

The operations taking conditions are pre-processed, which is commonly the time consuming part but, of course, with each added operation, the time to process a message
increases.

## For users

### Syntax for evaluatable terms

The message processor uses `gval` for evaluating the terms. It provides a basic set of operators like string comparison and arithmetic operations.

Accessible for operations are
- `name` of the message
- `timestamp` of the message
- `type`, `type-id` of the message (also `tag_type` and `tag_type-id`)
- `stype`, `stype-id` of the message (if message has theses tags, also `tag_stype` and `tag_stype-id`)
- `value` for a CCMetric message (also `field_value`)
- `event` for a CCEvent message (also `field_event`)
- `control` for a CCControl message (also `field_control`)
- `log` for a CCLog message (also `field_log`)

Generally, all tags are accessible with `tag_<tagkey>`, all meta information with `meta_<metakey>` and fields with `field_<fieldkey>`.

- Comparing strings: `==`, `!=`, `match(str, regex)` (use `%` instead of `\`!)
- Combining conditions: `&&`, `||`
- Comparing numbers: `==`, `!=`, `<`, `>`, `<=`, `>=`
- Test lists: `<value> in <list>`
- Topological tests: `tag_type-id in getCpuListOfType("socket", "1")` (test if the metric belongs to socket 1 in local node topology)

Often the operations are written in JSON files for loading them at startup. In JSON, some characters are not allowed. Therefore, the term syntax reflects that:
- use `''` instead of `""` for strings
- for the regexes, use `%` instead of `\`

