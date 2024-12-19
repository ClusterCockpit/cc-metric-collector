package messageprocessor

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lplegacy "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// Message processor add/delete tag/meta configuration
type messageProcessorTagConfig struct {
	Key       string `json:"key"`             // Tag name
	Value     string `json:"value,omitempty"` // Tag value
	Condition string `json:"if"`              // Condition for adding or removing corresponding tag
}

type messageProcessorConfig struct {
	StageOrder       []string                    `json:"stage_order,omitempty"`        // List of stages to execute them in the specified order and to skip unrequired ones
	DropMessages     []string                    `json:"drop_messages,omitempty"`      // List of metric names to drop. For fine-grained dropping use drop_messages_if
	DropMessagesIf   []string                    `json:"drop_messages_if,omitempty"`   // List of evaluatable terms to drop messages
	RenameMessages   map[string]string           `json:"rename_messages,omitempty"`    // Map of metric names to rename
	RenameMessagesIf map[string]string           `json:"rename_messages_if,omitempty"` // Map to rename metric name based on a condition
	NormalizeUnits   bool                        `json:"normalize_units,omitempty"`    // Check unit meta flag and normalize it using cc-units
	ChangeUnitPrefix map[string]string           `json:"change_unit_prefix,omitempty"` // Add prefix that should be applied to the messages
	AddTagsIf        []messageProcessorTagConfig `json:"add_tags_if"`                  // List of tags that are added when the condition is met
	DelTagsIf        []messageProcessorTagConfig `json:"delete_tags_if"`               // List of tags that are removed when the condition is met
	AddMetaIf        []messageProcessorTagConfig `json:"add_meta_if"`                  // List of meta infos that are added when the condition is met
	DelMetaIf        []messageProcessorTagConfig `json:"delete_meta_if"`               // List of meta infos that are removed when the condition is met
	AddFieldIf       []messageProcessorTagConfig `json:"add_field_if"`                 // List of fields that are added when the condition is met
	DelFieldIf       []messageProcessorTagConfig `json:"delete_field_if"`              // List of fields that are removed when the condition is met
	DropByType       []string                    `json:"drop_by_message_type"`         // List of message types that should be dropped
	MoveTagToMeta    []messageProcessorTagConfig `json:"move_tag_to_meta_if"`
	MoveTagToField   []messageProcessorTagConfig `json:"move_tag_to_field_if"`
	MoveMetaToTag    []messageProcessorTagConfig `json:"move_meta_to_tag_if"`
	MoveMetaToField  []messageProcessorTagConfig `json:"move_meta_to_field_if"`
	MoveFieldToTag   []messageProcessorTagConfig `json:"move_field_to_tag_if"`
	MoveFieldToMeta  []messageProcessorTagConfig `json:"move_field_to_meta_if"`
	AddBaseEnv       map[string]interface{}      `json:"add_base_env"`
}

type messageProcessor struct {

	// For thread-safety
	mutex sync.RWMutex

	// mapping contains all evalables as strings to gval.Evaluable
	// because it is not possible to get the original string out of
	// a gval.Evaluable
	mapping map[string]*vm.Program

	stages           []string                 // order of stage execution
	dropMessages     map[string]struct{}      // internal lookup map
	dropTypes        map[string]struct{}      // internal lookup map
	dropMessagesIf   map[*vm.Program]struct{} // pre-processed dropMessagesIf
	renameMessages   map[string]string        // internal lookup map
	renameMessagesIf map[*vm.Program]string   // pre-processed RenameMessagesIf
	changeUnitPrefix map[*vm.Program]string   // pre-processed ChangeUnitPrefix
	normalizeUnits   bool
	addTagsIf        map[*vm.Program]messageProcessorTagConfig // pre-processed AddTagsIf
	deleteTagsIf     map[*vm.Program]messageProcessorTagConfig // pre-processed DelTagsIf
	addMetaIf        map[*vm.Program]messageProcessorTagConfig // pre-processed AddMetaIf
	deleteMetaIf     map[*vm.Program]messageProcessorTagConfig // pre-processed DelMetaIf
	addFieldIf       map[*vm.Program]messageProcessorTagConfig // pre-processed AddFieldIf
	deleteFieldIf    map[*vm.Program]messageProcessorTagConfig // pre-processed DelFieldIf
	moveTagToMeta    map[*vm.Program]messageProcessorTagConfig // pre-processed MoveTagToMeta
	moveTagToField   map[*vm.Program]messageProcessorTagConfig // pre-processed MoveTagToField
	moveMetaToTag    map[*vm.Program]messageProcessorTagConfig // pre-processed MoveMetaToTag
	moveMetaToField  map[*vm.Program]messageProcessorTagConfig // pre-processed MoveMetaToField
	moveFieldToTag   map[*vm.Program]messageProcessorTagConfig // pre-processed MoveFieldToTag
	moveFieldToMeta  map[*vm.Program]messageProcessorTagConfig // pre-processed MoveFieldToMeta
}

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
	// Processing functions for legacy CCMetric and current CCMessage
	ProcessMetric(m lplegacy.CCMetric) (lp.CCMessage, error)
	ProcessMessage(m lp.CCMessage) (lp.CCMessage, error)
	//EvalToBool(condition string, parameters map[string]interface{}) (bool, error)
	//EvalToFloat64(condition string, parameters map[string]interface{}) (float64, error)
	//EvalToString(condition string, parameters map[string]interface{}) (string, error)
}

const (
	STAGENAME_DROP_BY_NAME       string = "drop_by_name"
	STAGENAME_DROP_BY_TYPE       string = "drop_by_type"
	STAGENAME_DROP_IF            string = "drop_if"
	STAGENAME_ADD_TAG            string = "add_tag"
	STAGENAME_DELETE_TAG         string = "delete_tag"
	STAGENAME_MOVE_TAG_META      string = "move_tag_to_meta"
	STAGENAME_MOVE_TAG_FIELD     string = "move_tag_to_fields"
	STAGENAME_ADD_META           string = "add_meta"
	STAGENAME_DELETE_META        string = "delete_meta"
	STAGENAME_MOVE_META_TAG      string = "move_meta_to_tags"
	STAGENAME_MOVE_META_FIELD    string = "move_meta_to_fields"
	STAGENAME_ADD_FIELD          string = "add_field"
	STAGENAME_DELETE_FIELD       string = "delete_field"
	STAGENAME_MOVE_FIELD_TAG     string = "move_field_to_tags"
	STAGENAME_MOVE_FIELD_META    string = "move_field_to_meta"
	STAGENAME_RENAME_BY_NAME     string = "rename"
	STAGENAME_RENAME_IF          string = "rename_if"
	STAGENAME_CHANGE_UNIT_PREFIX string = "change_unit_prefix"
	STAGENAME_NORMALIZE_UNIT     string = "normalize_unit"
)

var StageNames = []string{
	STAGENAME_DROP_BY_NAME,
	STAGENAME_DROP_BY_TYPE,
	STAGENAME_DROP_IF,
	STAGENAME_ADD_TAG,
	STAGENAME_DELETE_TAG,
	STAGENAME_MOVE_TAG_META,
	STAGENAME_MOVE_TAG_FIELD,
	STAGENAME_ADD_META,
	STAGENAME_DELETE_META,
	STAGENAME_MOVE_META_TAG,
	STAGENAME_MOVE_META_FIELD,
	STAGENAME_ADD_FIELD,
	STAGENAME_DELETE_FIELD,
	STAGENAME_MOVE_FIELD_TAG,
	STAGENAME_MOVE_FIELD_META,
	STAGENAME_RENAME_BY_NAME,
	STAGENAME_RENAME_IF,
	STAGENAME_CHANGE_UNIT_PREFIX,
	STAGENAME_NORMALIZE_UNIT,
}

var paramMapPool = sync.Pool{
	New: func() any {
		return make(map[string]interface{})
	},
}

func sanitizeExprString(key string) string {
	return strings.ReplaceAll(key, "type-id", "typeid")
}

func getParamMap(point lp.CCMetric) map[string]interface{} {
	params := paramMapPool.Get().(map[string]interface{})
	params["message"] = point
	params["msg"] = point
	params["name"] = point.Name()
	params["timestamp"] = point.Time().Unix()
	params["time"] = params["timestamp"]

	fields := paramMapPool.Get().(map[string]interface{})
	for key, value := range point.Fields() {
		fields[key] = value
		switch key {
		case "value":
			params["messagetype"] = "metric"
			params["value"] = value
			params["metric"] = value
		case "event":
			params["messagetype"] = "event"
			params["event"] = value
		case "control":
			params["messagetype"] = "control"
			params["control"] = value
		case "log":
			params["messagetype"] = "log"
			params["log"] = value
		default:
			params["messagetype"] = "unknown"
		}
	}
	params["msgtype"] = params["messagetype"]
	params["fields"] = fields
	params["field"] = fields
	tags := paramMapPool.Get().(map[string]interface{})
	for key, value := range point.Tags() {
		tags[sanitizeExprString(key)] = value
	}
	params["tags"] = tags
	params["tag"] = tags
	meta := paramMapPool.Get().(map[string]interface{})
	for key, value := range point.Meta() {
		meta[sanitizeExprString(key)] = value
	}
	params["meta"] = meta
	return params
}

var baseenv = map[string]interface{}{
	"name":        "",
	"messagetype": "unknown",
	"msgtype":     "unknown",
	"tag": map[string]interface{}{
		"type":     "unknown",
		"typeid":   "0",
		"stype":    "unknown",
		"stypeid":  "0",
		"hostname": "localhost",
		"cluster":  "nocluster",
	},
	"tags": map[string]interface{}{
		"type":     "unknown",
		"typeid":   "0",
		"stype":    "unknown",
		"stypeid":  "0",
		"hostname": "localhost",
		"cluster":  "nocluster",
	},
	"meta": map[string]interface{}{
		"unit":   "invalid",
		"source": "unknown",
	},
	"fields": map[string]interface{}{
		"value":   0,
		"event":   "",
		"control": "",
		"log":     "",
	},
	"field": map[string]interface{}{
		"value":   0,
		"event":   "",
		"control": "",
		"log":     "",
	},
	"timestamp": 1234567890,
	"msg":       lp.EmptyMessage(),
	"message":   lp.EmptyMessage(),
}

func addBaseEnvWalker(values map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range values {
		switch value := v.(type) {
		case int, int32, int64, uint, uint32, uint64, string, float32, float64:
			out[k] = value
		case map[string]interface{}:
			if _, ok := baseenv[k]; !ok {
				out[k] = addBaseEnvWalker(value)
			}
		}
	}
	return out
}

func (mp *messageProcessor) AddBaseEnv(env map[string]interface{}) error {
	for k, v := range env {
		switch value := v.(type) {
		case int, int32, int64, uint, uint32, uint64, string, float32, float64:
			baseenv[k] = value
		case map[string]interface{}:
			if _, ok := baseenv[k]; !ok {
				baseenv[k] = addBaseEnvWalker(value)
			}
		}
	}
	return nil
}

func (mp *messageProcessor) init() error {
	mp.stages = make([]string, 0)
	mp.mapping = make(map[string]*vm.Program)
	mp.dropMessages = make(map[string]struct{})
	mp.dropTypes = make(map[string]struct{})
	mp.dropMessagesIf = make(map[*vm.Program]struct{})
	mp.renameMessages = make(map[string]string)
	mp.renameMessagesIf = make(map[*vm.Program]string)
	mp.changeUnitPrefix = make(map[*vm.Program]string)
	mp.addTagsIf = make(map[*vm.Program]messageProcessorTagConfig)
	mp.addMetaIf = make(map[*vm.Program]messageProcessorTagConfig)
	mp.addFieldIf = make(map[*vm.Program]messageProcessorTagConfig)
	mp.deleteTagsIf = make(map[*vm.Program]messageProcessorTagConfig)
	mp.deleteMetaIf = make(map[*vm.Program]messageProcessorTagConfig)
	mp.deleteFieldIf = make(map[*vm.Program]messageProcessorTagConfig)
	mp.moveFieldToMeta = make(map[*vm.Program]messageProcessorTagConfig)
	mp.moveFieldToTag = make(map[*vm.Program]messageProcessorTagConfig)
	mp.moveMetaToField = make(map[*vm.Program]messageProcessorTagConfig)
	mp.moveMetaToTag = make(map[*vm.Program]messageProcessorTagConfig)
	mp.moveTagToField = make(map[*vm.Program]messageProcessorTagConfig)
	mp.moveTagToMeta = make(map[*vm.Program]messageProcessorTagConfig)
	mp.normalizeUnits = false
	return nil
}

func (mp *messageProcessor) AddDropMessagesByName(name string) error {
	mp.mutex.Lock()
	if _, ok := mp.dropMessages[name]; !ok {
		mp.dropMessages[name] = struct{}{}
	}
	mp.mutex.Unlock()
	return nil
}

func (mp *messageProcessor) RemoveDropMessagesByName(name string) {
	mp.mutex.Lock()
	delete(mp.dropMessages, name)
	mp.mutex.Unlock()
}

func (mp *messageProcessor) AddDropMessagesByType(typestring string) error {
	valid := []string{"metric", "event", "control", "log"}
	isValid := false
	for _, t := range valid {
		if t == typestring {
			isValid = true
			break
		}
	}
	if isValid {
		mp.mutex.Lock()
		if _, ok := mp.dropTypes[typestring]; !ok {
			cclog.ComponentDebug("MessageProcessor", "Adding type", typestring, "for dropping")
			mp.dropTypes[typestring] = struct{}{}
		}
		mp.mutex.Unlock()
	} else {
		return fmt.Errorf("invalid message type %s", typestring)
	}
	return nil
}

func (mp *messageProcessor) RemoveDropMessagesByType(typestring string) {
	mp.mutex.Lock()
	delete(mp.dropTypes, typestring)
	mp.mutex.Unlock()
}

func (mp *messageProcessor) addTagConfig(condition, key, value string, config *map[*vm.Program]messageProcessorTagConfig) error {
	var err error
	evaluable, err := expr.Compile(sanitizeExprString(condition), expr.Env(baseenv), expr.AsBool())
	if err != nil {
		return fmt.Errorf("failed to create condition evaluable of '%s': %v", condition, err.Error())
	}
	mp.mutex.Lock()
	if _, ok := (*config)[evaluable]; !ok {
		mp.mapping[condition] = evaluable
		(*config)[evaluable] = messageProcessorTagConfig{
			Condition: condition,
			Key:       key,
			Value:     value,
		}
	}
	mp.mutex.Unlock()
	return nil
}

func (mp *messageProcessor) removeTagConfig(condition string, config *map[*vm.Program]messageProcessorTagConfig) {
	mp.mutex.Lock()
	if e, ok := mp.mapping[condition]; ok {
		delete(mp.mapping, condition)
		delete(*config, e)
	}
	mp.mutex.Unlock()
}

func (mp *messageProcessor) AddAddTagsByCondition(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.addTagsIf)
}

func (mp *messageProcessor) RemoveAddTagsByCondition(condition string) {
	mp.removeTagConfig(condition, &mp.addTagsIf)
}

func (mp *messageProcessor) AddDeleteTagsByCondition(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.deleteTagsIf)
}

func (mp *messageProcessor) RemoveDeleteTagsByCondition(condition string) {
	mp.removeTagConfig(condition, &mp.deleteTagsIf)
}

func (mp *messageProcessor) AddAddMetaByCondition(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.addMetaIf)
}

func (mp *messageProcessor) RemoveAddMetaByCondition(condition string) {
	mp.removeTagConfig(condition, &mp.addMetaIf)
}

func (mp *messageProcessor) AddDeleteMetaByCondition(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.deleteMetaIf)
}

func (mp *messageProcessor) RemoveDeleteMetaByCondition(condition string) {
	mp.removeTagConfig(condition, &mp.deleteMetaIf)
}

func (mp *messageProcessor) AddAddFieldByCondition(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.addFieldIf)
}

func (mp *messageProcessor) RemoveAddFieldByCondition(condition string) {
	mp.removeTagConfig(condition, &mp.addFieldIf)
}

func (mp *messageProcessor) AddDeleteFieldByCondition(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.deleteFieldIf)
}

func (mp *messageProcessor) RemoveDeleteFieldByCondition(condition string) {
	mp.removeTagConfig(condition, &mp.deleteFieldIf)
}

func (mp *messageProcessor) AddDropMessagesByCondition(condition string) error {

	var err error
	evaluable, err := expr.Compile(sanitizeExprString(condition), expr.Env(baseenv), expr.AsBool())
	if err != nil {
		return fmt.Errorf("failed to create condition evaluable of '%s': %v", condition, err.Error())
	}
	mp.mutex.Lock()
	if _, ok := mp.dropMessagesIf[evaluable]; !ok {
		mp.mapping[condition] = evaluable
		mp.dropMessagesIf[evaluable] = struct{}{}
	}
	mp.mutex.Unlock()
	return nil
}

func (mp *messageProcessor) RemoveDropMessagesByCondition(condition string) {
	mp.mutex.Lock()
	if e, ok := mp.mapping[condition]; ok {
		delete(mp.mapping, condition)
		delete(mp.dropMessagesIf, e)
	}
	mp.mutex.Unlock()
}

func (mp *messageProcessor) AddRenameMetricByCondition(condition string, name string) error {

	var err error
	evaluable, err := expr.Compile(sanitizeExprString(condition), expr.Env(baseenv), expr.AsBool())
	if err != nil {
		return fmt.Errorf("failed to create condition evaluable of '%s': %v", condition, err.Error())
	}
	mp.mutex.Lock()
	if _, ok := mp.renameMessagesIf[evaluable]; !ok {
		mp.mapping[condition] = evaluable
		mp.renameMessagesIf[evaluable] = name
	} else {
		mp.renameMessagesIf[evaluable] = name
	}
	mp.mutex.Unlock()
	return nil
}

func (mp *messageProcessor) RemoveRenameMetricByCondition(condition string) {
	mp.mutex.Lock()
	if e, ok := mp.mapping[condition]; ok {
		delete(mp.mapping, condition)
		delete(mp.renameMessagesIf, e)
	}
	mp.mutex.Unlock()
}

func (mp *messageProcessor) SetNormalizeUnits(setting bool) {
	mp.normalizeUnits = setting
}

func (mp *messageProcessor) AddChangeUnitPrefix(condition string, prefix string) error {

	var err error
	evaluable, err := expr.Compile(sanitizeExprString(condition), expr.Env(baseenv), expr.AsBool())
	if err != nil {
		return fmt.Errorf("failed to create condition evaluable of '%s': %v", condition, err.Error())
	}
	mp.mutex.Lock()
	if _, ok := mp.changeUnitPrefix[evaluable]; !ok {
		mp.mapping[condition] = evaluable
		mp.changeUnitPrefix[evaluable] = prefix
	} else {
		mp.changeUnitPrefix[evaluable] = prefix
	}
	mp.mutex.Unlock()
	return nil
}

func (mp *messageProcessor) RemoveChangeUnitPrefix(condition string) {
	mp.mutex.Lock()
	if e, ok := mp.mapping[condition]; ok {
		delete(mp.mapping, condition)
		delete(mp.changeUnitPrefix, e)
	}
	mp.mutex.Unlock()
}

func (mp *messageProcessor) AddRenameMetricByName(from, to string) error {
	mp.mutex.Lock()
	if _, ok := mp.renameMessages[from]; !ok {
		mp.renameMessages[from] = to
	}
	mp.mutex.Unlock()
	return nil
}

func (mp *messageProcessor) RemoveRenameMetricByName(from string) {
	mp.mutex.Lock()
	delete(mp.renameMessages, from)
	mp.mutex.Unlock()
}

func (mp *messageProcessor) AddMoveTagToMeta(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.moveTagToMeta)
}

func (mp *messageProcessor) RemoveMoveTagToMeta(condition string) {
	mp.removeTagConfig(condition, &mp.moveTagToMeta)
}

func (mp *messageProcessor) AddMoveTagToFields(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.moveTagToField)
}

func (mp *messageProcessor) RemoveMoveTagToFields(condition string) {
	mp.removeTagConfig(condition, &mp.moveTagToField)
}

func (mp *messageProcessor) AddMoveMetaToTags(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.moveMetaToTag)
}

func (mp *messageProcessor) RemoveMoveMetaToTags(condition string) {
	mp.removeTagConfig(condition, &mp.moveMetaToTag)
}

func (mp *messageProcessor) AddMoveMetaToFields(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.moveMetaToField)
}

func (mp *messageProcessor) RemoveMoveMetaToFields(condition string) {
	mp.removeTagConfig(condition, &mp.moveMetaToField)
}

func (mp *messageProcessor) AddMoveFieldToTags(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.moveFieldToTag)
}

func (mp *messageProcessor) RemoveMoveFieldToTags(condition string) {
	mp.removeTagConfig(condition, &mp.moveFieldToTag)
}

func (mp *messageProcessor) AddMoveFieldToMeta(condition, key, value string) error {
	return mp.addTagConfig(condition, key, value, &mp.moveFieldToMeta)
}

func (mp *messageProcessor) RemoveMoveFieldToMeta(condition string) {
	mp.removeTagConfig(condition, &mp.moveFieldToMeta)
}

func (mp *messageProcessor) SetStages(stages []string) error {
	newstages := make([]string, 0)
	if len(stages) == 0 {
		mp.mutex.Lock()
		mp.stages = newstages
		mp.mutex.Unlock()
		return nil
	}
	for i, s := range stages {
		valid := false
		for _, v := range StageNames {
			if s == v {
				valid = true
			}
		}
		if valid {
			newstages = append(newstages, s)
		} else {
			return fmt.Errorf("invalid stage %s at index %d", s, i)
		}
	}
	mp.mutex.Lock()
	mp.stages = newstages
	mp.mutex.Unlock()
	return nil
}

func (mp *messageProcessor) DefaultStages() []string {
	return StageNames
}

func (mp *messageProcessor) FromConfigJSON(config json.RawMessage) error {
	var c messageProcessorConfig

	err := json.Unmarshal(config, &c)
	if err != nil {
		return fmt.Errorf("failed to process config JSON: %v", err.Error())
	}

	if len(c.StageOrder) > 0 {
		err = mp.SetStages(c.StageOrder)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	} else {
		err = mp.SetStages(mp.DefaultStages())
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}

	for _, m := range c.DropMessages {
		err = mp.AddDropMessagesByName(m)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, m := range c.DropByType {
		err = mp.AddDropMessagesByType(m)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, m := range c.DropMessagesIf {
		err = mp.AddDropMessagesByCondition(m)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for k, v := range c.RenameMessagesIf {
		err = mp.AddRenameMetricByCondition(k, v)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for k, v := range c.RenameMessages {
		err = mp.AddRenameMetricByName(k, v)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for k, v := range c.ChangeUnitPrefix {
		err = mp.AddChangeUnitPrefix(k, v)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.AddTagsIf {
		err = mp.AddAddTagsByCondition(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.AddMetaIf {
		err = mp.AddAddMetaByCondition(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.AddFieldIf {
		err = mp.AddAddFieldByCondition(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.DelTagsIf {
		err = mp.AddDeleteTagsByCondition(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.DelMetaIf {
		err = mp.AddDeleteMetaByCondition(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.DelFieldIf {
		err = mp.AddDeleteFieldByCondition(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.MoveTagToMeta {
		err = mp.AddMoveTagToMeta(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.MoveTagToField {
		err = mp.AddMoveTagToFields(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.MoveMetaToTag {
		err = mp.AddMoveMetaToTags(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.MoveMetaToField {
		err = mp.AddMoveMetaToFields(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.MoveFieldToTag {
		err = mp.AddMoveFieldToTags(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, c := range c.MoveFieldToMeta {
		err = mp.AddMoveFieldToMeta(c.Condition, c.Key, c.Value)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	for _, m := range c.DropByType {
		err = mp.AddDropMessagesByType(m)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	if len(c.AddBaseEnv) > 0 {
		err = mp.AddBaseEnv(c.AddBaseEnv)
		if err != nil {
			return fmt.Errorf("failed to process config JSON: %v", err.Error())
		}
	}
	mp.SetNormalizeUnits(c.NormalizeUnits)
	return nil
}

func (mp *messageProcessor) ProcessMetric(metric lplegacy.CCMetric) (lp.CCMessage, error) {
	m, err := lp.NewMessage(
		metric.Name(),
		metric.Tags(),
		metric.Meta(),
		metric.Fields(),
		metric.Time(),
	)
	if err != nil {
		return m, fmt.Errorf("failed to parse metric to message: %v", err.Error())
	}
	return mp.ProcessMessage(m)

}

func (mp *messageProcessor) ProcessMessage(m lp.CCMessage) (lp.CCMessage, error) {
	var err error = nil
	var out lp.CCMessage = lp.FromMessage(m)

	name := out.Name()

	if len(mp.stages) == 0 {
		mp.SetStages(mp.DefaultStages())
	}

	mp.mutex.RLock()
	defer mp.mutex.RUnlock()

	params := getParamMap(out)

	defer func() {
		params["field"] = nil
		params["tag"] = nil
		paramMapPool.Put(params["fields"])
		paramMapPool.Put(params["tags"])
		paramMapPool.Put(params["meta"])
		paramMapPool.Put(params)
	}()

	for _, s := range mp.stages {
		switch s {
		case STAGENAME_DROP_BY_NAME:
			if len(mp.dropMessages) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Dropping by message name ", name)
				if _, ok := mp.dropMessages[name]; ok {
					//cclog.ComponentDebug("MessageProcessor", "Drop")
					return nil, nil
				}
			}
		case STAGENAME_DROP_BY_TYPE:
			if len(mp.dropTypes) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Dropping by message type")
				if _, ok := mp.dropTypes[params["messagetype"].(string)]; ok {
					//cclog.ComponentDebug("MessageProcessor", "Drop")
					return nil, nil
				}
			}
		case STAGENAME_DROP_IF:
			if len(mp.dropMessagesIf) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Dropping by condition")
				drop, err := dropMessagesIf(&params, &mp.dropMessagesIf)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
				if drop {
					//cclog.ComponentDebug("MessageProcessor", "Drop")
					return nil, nil
				}
			}
		case STAGENAME_RENAME_BY_NAME:
			if len(mp.renameMessages) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Renaming by name match")
				if newname, ok := mp.renameMessages[name]; ok {
					//cclog.ComponentDebug("MessageProcessor", "Rename to", newname)
					out.SetName(newname)
					//cclog.ComponentDebug("MessageProcessor", "Add old name as 'oldname' to meta", name)
					out.AddMeta("oldname", name)
				}
			}
		case STAGENAME_RENAME_IF:
			if len(mp.renameMessagesIf) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Renaming by condition")
				_, err := renameMessagesIf(out, &params, &mp.renameMessagesIf)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_ADD_TAG:
			if len(mp.addTagsIf) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Adding tags")
				_, err = addTagIf(out, &params, &mp.addTagsIf)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_DELETE_TAG:
			if len(mp.deleteTagsIf) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Delete tags")
				_, err = deleteTagIf(out, &params, &mp.deleteTagsIf)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_ADD_META:
			if len(mp.addMetaIf) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Adding meta information")
				_, err = addMetaIf(out, &params, &mp.addMetaIf)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_DELETE_META:
			if len(mp.deleteMetaIf) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Delete meta information")
				_, err = deleteMetaIf(out, &params, &mp.deleteMetaIf)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_ADD_FIELD:
			if len(mp.addFieldIf) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Adding fields")
				_, err = addFieldIf(out, &params, &mp.addFieldIf)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_DELETE_FIELD:
			if len(mp.deleteFieldIf) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Delete fields")
				_, err = deleteFieldIf(out, &params, &mp.deleteFieldIf)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_MOVE_TAG_META:
			if len(mp.moveTagToMeta) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Move tag to meta")
				_, err := moveTagToMeta(out, &params, &mp.moveTagToMeta)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_MOVE_TAG_FIELD:
			if len(mp.moveTagToField) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Move tag to fields")
				_, err := moveTagToField(out, &params, &mp.moveTagToField)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_MOVE_META_TAG:
			if len(mp.moveMetaToTag) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Move meta to tags")
				_, err := moveMetaToTag(out, &params, &mp.moveMetaToTag)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_MOVE_META_FIELD:
			if len(mp.moveMetaToField) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Move meta to fields")
				_, err := moveMetaToField(out, &params, &mp.moveMetaToField)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_MOVE_FIELD_META:
			if len(mp.moveFieldToMeta) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Move field to meta")
				_, err := moveFieldToMeta(out, &params, &mp.moveFieldToMeta)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_MOVE_FIELD_TAG:
			if len(mp.moveFieldToTag) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Move field to tags")
				_, err := moveFieldToTag(out, &params, &mp.moveFieldToTag)
				if err != nil {
					return out, fmt.Errorf("failed to evaluate: %v", err.Error())
				}
			}
		case STAGENAME_NORMALIZE_UNIT:
			if mp.normalizeUnits {
				//cclog.ComponentDebug("MessageProcessor", "Normalize units")
				if lp.IsMetric(out) {
					_, err := normalizeUnits(out)
					if err != nil {
						return out, fmt.Errorf("failed to evaluate: %v", err.Error())
					}
				} else {
					cclog.ComponentDebug("MessageProcessor", "skipped, no metric")
				}
			}

		case STAGENAME_CHANGE_UNIT_PREFIX:
			if len(mp.changeUnitPrefix) > 0 {
				//cclog.ComponentDebug("MessageProcessor", "Change unit prefix")
				if lp.IsMetric(out) {
					_, err := changeUnitPrefix(out, &params, &mp.changeUnitPrefix)
					if err != nil {
						return out, fmt.Errorf("failed to evaluate: %v", err.Error())
					}
				} else {
					cclog.ComponentDebug("MessageProcessor", "skipped, no metric")
				}
			}
		}

	}

	return out, nil
}

// Get a new instace of a message processor.
func NewMessageProcessor() (MessageProcessor, error) {
	mp := new(messageProcessor)
	err := mp.init()
	if err != nil {
		err := fmt.Errorf("failed to create MessageProcessor: %v", err.Error())
		cclog.ComponentError("MessageProcessor", err.Error())
		return nil, err
	}
	return mp, nil
}
