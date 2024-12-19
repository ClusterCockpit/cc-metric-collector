# Message Processor Component

Multiple parts of in the ClusterCockit ecosystem require the processing of CCMessages.
The main CC application using it is `cc-metric-collector`. The processing part there was originally in the metric router, the central
hub connecting collectors (reading local data), receivers (receiving remote data) and sinks (sending data). Already in early stages, the
lack of flexibility caused some trouble:

> The sysadmins wanted to keep operating their Ganglia based monitoring infrastructure while we developed the CC stack. Ganglia wants the core metrics with
> a specific name and resolution (right unit prefix) but there was no conversion of the data in the CC stack, so CC frontend developers wanted a different
> resolution for some metrics. The issue was basically the `mem_used` metric showing the currently used memory of the node. Ganglia wants it in `kByte` as provided
> by the Linux operating system but CC wanted it in `GByte`.

With the message processor, the Ganglia sinks can apply the unit prefix changes individually and name the metrics as required by Ganglia.

## For developers

Whenever you receive or are about to send a message out, you should provide some processing.

### Configuration of component

New operations can be added to the message processor at runtime. Of course, they can also be removed again. For the initial setup, having a configuration file
or some fields in a configuration file for the processing.

The message processor uses the following configuration

```json
{
	"drop_messages": [
		"name_of_message_to_drop"
	],
	"drop_messages_if": [
		"condition_when_to_drop_message",
		"name == 'drop_this'",
		"tag.hostname == 'this_host'",
		"meta.unit != 'MB'"
	],
	"rename_messages" : {
		"old_message_name" : "new_message_name"
	},
	"rename_messages_if": {
		"condition_when_to_rename_message" : "new_name"
	},
	"add_tags_if": [
		{
			"if" : "condition_when_to_add_tag",
			"key": "name_for_new_tag",
			"value": "new_tag_value"
		}
	],
	"delete_tags_if": [
		{
			"if" : "condition_when_to_delete_tag",
			"key": "name_of_tag"
		}
	],
	"add_meta_if": [
		{
			"if" : "condition_when_to_add_meta_info",
			"key": "name_for_new_meta_info",
			"value": "new_meta_info_value"
		}
	],
	"delete_meta_if": [
		{
			"if" : "condition_when_to_delete_meta_info",
			"key": "name_of_meta_info"
		}
	],
	"add_field_if": [
		{
			"if" : "condition_when_to_add_field",
			"key": "name_for_new_field",
			"value": "new_field_value_but_only_string_at_the_moment"
		}
	],
	"delete_field_if": [
		{
			"if" : "condition_when_to_delete_field",
			"key": "name_of_field"
		}
	],
	"move_tag_to_meta_if": [
		{
			"if" : "condition_when_to_move_tag_to_meta_info_including_its_value",
			"key": "name_of_tag",
			"value": "name_of_meta_info"
		}
	],
	"move_tag_to_field_if": [
		{
			"if" : "condition_when_to_move_tag_to_fields_including_its_value",
			"key": "name_of_tag",
			"value": "name_of_field"
		}
	],
	"move_meta_to_tag_if": [
		{
			"if" : "condition_when_to_move_meta_info_to_tags_including_its_value",
			"key": "name_of_meta_info",
			"value": "name_of_tag"
		}
	],
	"move_meta_to_field_if": [
		{
			"if" : "condition_when_to_move_meta_info_to_fields_including_its_value",
			"key": "name_of_tag",
			"value": "name_of_meta_info"
		}
	],
	"move_field_to_tag_if": [
		{
			"if" : "condition_when_to_move_field_to_tags_including_its_stringified_value",
			"key": "name_of_field",
			"value": "name_of_tag"
		}
	],
	"move_field_to_meta_if": [
		{
			"if" : "condition_when_to_move_field_to_meta_info_including_its_stringified_value",
			"key": "name_of_field",
			"value": "name_of_meta_info"
		}
	],
	"drop_by_message_type": [
		"metric",
		"event",
		"log",
		"control"
	],
	"change_unit_prefix": {
		"name == 'metric_with_wrong_unit_prefix'" : "G",
		"only_if_messagetype == 'metric'": "T"
	},
	"normalize_units": true,
	"add_base_env": {
		"MY_CONSTANT_FOR_CUSTOM_CONDITIONS": 1.0,
		"output_value_for_test_metrics": 42.0,
	},
	"stage_order": [
		"rename_messages_if",
		"drop_messages"
	]
}
```

The options `change_unit_prefix` and `normalize_units` are only applied to CCMetrics. It is not possible to delete the field related to each message type as defined in [cc-specification](https://github.com/ClusterCockpit/cc-specifications/tree/master/interfaces/lineprotocol). In short:
- CCMetrics always have to have a field named `value`
- CCEvents always have to have a field named `event`
- CCLogs always have to have a field named `log`
- CCControl messages always have to have a field named `control`

With `add_base_env`, one can specifiy mykey=myvalue pairs that can be used in conditions like `tag.type == mykey`.

The order in which each message is processed, can be specified with the `stage_order` option. The stage names are the keys in the JSON configuration, thus `change_unit_prefix`, `move_field_to_meta_if`, etc. Stages can be listed multiple times.

### Using the component
In order to load the configuration from a `json.RawMessage`:
```golang
mp, err := NewMessageProcessor()
if err != nil {
	log.Error("failed to create new message processor")
}
mp.FromConfigJSON(configJson)
```

After initialization and adding the different operations, the `ProcessMessage()` function applies all operations and returns whether the message should be dropped.

```golang
m := lp.CCMetric{}

x, err := mp.ProcessMessage(m)
if err != nil {
	// handle error
}
if x != nil {
    // process x further
} else {
	// this message got dropped
}
```

Single operations can be added and removed at runtime
```golang
type MessageProcessor interface {
	// Functions to set the execution order of the processing stages
	SetStages([]string) error
	DefaultStages() []string
	// Function to add variables to the base evaluation environment
	AddBaseEnv(env map[string]interface{}) error
	// Functions to add and remove rules
	AddDropMessagesByName(name string) error
	RemoveDropMessagesByName(name string)
	AddDropMessagesByCondition(condition string) error
	RemoveDropMessagesByCondition(condition string)
	AddRenameMetricByCondition(condition string, name string) error
	RemoveRenameMetricByCondition(condition string)
	AddRenameMetricByName(from, to string) error
	RemoveRenameMetricByName(from string)
	SetNormalizeUnits(settings bool)
	AddChangeUnitPrefix(condition string, prefix string) error
	RemoveChangeUnitPrefix(condition string)
	AddAddTagsByCondition(condition, key, value string) error
	RemoveAddTagsByCondition(condition string)
	AddDeleteTagsByCondition(condition, key, value string) error
	RemoveDeleteTagsByCondition(condition string)
	AddAddMetaByCondition(condition, key, value string) error
	RemoveAddMetaByCondition(condition string)
	AddDeleteMetaByCondition(condition, key, value string) error
	RemoveDeleteMetaByCondition(condition string)
	AddMoveTagToMeta(condition, key, value string) error
	RemoveMoveTagToMeta(condition string)
	AddMoveTagToFields(condition, key, value string) error
	RemoveMoveTagToFields(condition string)
	AddMoveMetaToTags(condition, key, value string) error
	RemoveMoveMetaToTags(condition string)
	AddMoveMetaToFields(condition, key, value string) error
	RemoveMoveMetaToFields(condition string)
	AddMoveFieldToTags(condition, key, value string) error
	RemoveMoveFieldToTags(condition string)
	AddMoveFieldToMeta(condition, key, value string) error
	RemoveMoveFieldToMeta(condition string)
	// Read in a JSON configuration
	FromConfigJSON(config json.RawMessage) error
	ProcessMessage(m lp2.CCMessage) (lp2.CCMessage, error)
	// Processing functions for legacy CCMetric and current CCMessage
	ProcessMetric(m lp.CCMetric) (lp2.CCMessage, error)
}
```


### Syntax for evaluatable terms

The message processor uses `gval` for evaluating the terms. It provides a basic set of operators like string comparison and arithmetic operations.

Accessible for operations are
- `name` of the message
- `timestamp` or `time` of the message
- `type`, `type-id` of the message (also `tag_type`, `tag_type-id` and `tag_typeid`)
- `stype`, `stype-id` of the message (if message has theses tags, also `tag_stype`, `tag_stype-id` and `tag_stypeid`)
- `value` for a CCMetric message (also `field_value`)
- `event` for a CCEvent message (also `field_event`)
- `control` for a CCControl message (also `field_control`)
- `log` for a CCLog message (also `field_log`)
- `messagetype` or `msgtype`. Possible values `event`, `metric`, `log` and `control`.

Generally, all tags are accessible with `tag_<tagkey>`, `tags_<tagkey>` or `tags.<tagkey>`. Similarly for all fields with `field[s]?[_.]<fieldkey>`. For meta information `meta[_.]<metakey>` (there is no `metas[_.]<metakey>`).

The [syntax of `expr`](https://expr-lang.org/docs/language-definition) is accepted with some additions:
- Comparing strings: `==`, `!=`, `str matches regex` (use `%` instead of `\`!)
- Combining conditions: `&&`, `||`
- Comparing numbers: `==`, `!=`, `<`, `>`, `<=`, `>=`
- Test lists: `<value> in <list>`
- Topological tests: `tag_type-id in getCpuListOfType("socket", "1")` (test if the metric belongs to socket 1 in local node topology)

Often the operations are written in JSON files for loading them at startup. In JSON, some characters are not allowed. Therefore, the term syntax reflects that:
- use `''` instead of `""` for strings
- for the regexes, use `%` instead of `\`


For operations that should be applied on all messages, use the condition `true`.

### Overhead

The operations taking conditions are pre-processed, which is commonly the time consuming part but, of course, with each added operation, the time to process a message
increases. Moreover, the processing creates a copy of the message.

