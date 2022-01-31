package ccmetric

import (
	"fmt"
	"sort"
	"time"

	lp "github.com/influxdata/line-protocol" // MIT license
)

// Most functions are derived from github.com/influxdata/line-protocol/metric.go
// The metric type is extended with an extra meta information list re-using the Tag
// type.
//
// See: https://docs.influxdata.com/influxdb/latest/reference/syntax/line-protocol/
type ccMetric struct {
	name   string      // Measurement name
	tags   []*lp.Tag   // ordered list of of tags
	fields []*lp.Field // unordered list of of fields
	tm     time.Time   // timestamp
	meta   []*lp.Tag   // odered list of meta data tags
}

// ccmetric access functions
type CCMetric interface {
	lp.MutableMetric                         // SetTime, AddTag, AddField
	AddMeta(key, value string)               // Add a meta data tag
	MetaList() []*lp.Tag                     // Returns the meta data list
	RemoveTag(key string)                    // Remove a tag addressed by its key
	GetTag(key string) (string, bool)        // Get a tag addressed by its key
	GetMeta(key string) (string, bool)       // Get a meta data tab addressed by its key
	GetField(key string) (interface{}, bool) // Get a field addressed by its key
	HasField(key string) bool                // Check if a field key is present
	RemoveField(key string)                  // Remove a field addressed by its key
}

// Meta returns the list of meta data tags as key-value mapping
func (m *ccMetric) Meta() map[string]string {
	meta := make(map[string]string, len(m.meta))
	for _, m := range m.meta {
		meta[m.Key] = m.Value
	}
	return meta
}

// MetaList returns the list of meta data tags
func (m *ccMetric) MetaList() []*lp.Tag {
	return m.meta
}

// String implements the stringer interface for data type ccMetric
func (m *ccMetric) String() string {
	return fmt.Sprintf("%s %v %v %v %d", m.name, m.Tags(), m.Meta(), m.Fields(), m.tm.UnixNano())
}

// Name returns the metric name
func (m *ccMetric) Name() string {
	return m.name
}

// Tags returns the the list of tags as key-value-mapping
func (m *ccMetric) Tags() map[string]string {
	tags := make(map[string]string, len(m.tags))
	for _, tag := range m.tags {
		tags[tag.Key] = tag.Value
	}
	return tags
}

// TagList returns the list of tags
func (m *ccMetric) TagList() []*lp.Tag {
	return m.tags
}

// Fields returns the list of fields as key-value-mapping
func (m *ccMetric) Fields() map[string]interface{} {
	fields := make(map[string]interface{}, len(m.fields))
	for _, field := range m.fields {
		fields[field.Key] = field.Value
	}

	return fields
}

// FieldList returns the list of fields
func (m *ccMetric) FieldList() []*lp.Field {
	return m.fields
}

// Time returns timestamp
func (m *ccMetric) Time() time.Time {
	return m.tm
}

// SetTime sets the timestamp
func (m *ccMetric) SetTime(t time.Time) {
	m.tm = t
}

// HasTag checks if a tag with key equal to <key> is present in the list of tags
func (m *ccMetric) HasTag(key string) bool {
	for _, tag := range m.tags {
		if tag.Key == key {
			return true
		}
	}
	return false
}

// GetTag returns the tag with tag's key equal to <key>
func (m *ccMetric) GetTag(key string) (string, bool) {
	for _, tag := range m.tags {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

// RemoveTag removes the tag with tag's key equal to <key>
// and keeps the tag list ordered by the keys
func (m *ccMetric) RemoveTag(key string) {
	for i, tag := range m.tags {
		if tag.Key == key {
			copy(m.tags[i:], m.tags[i+1:])
			m.tags[len(m.tags)-1] = nil
			m.tags = m.tags[:len(m.tags)-1]
			return
		}
	}
}

// AddTag adds a tag (consisting of key and value)
// and keeps the tag list ordered by the keys
func (m *ccMetric) AddTag(key, value string) {
	for i, tag := range m.tags {
		if key > tag.Key {
			continue
		}

		if key == tag.Key {
			tag.Value = value
			return
		}

		m.tags = append(m.tags, nil)
		copy(m.tags[i+1:], m.tags[i:])
		m.tags[i] = &lp.Tag{Key: key, Value: value}
		return
	}

	m.tags = append(m.tags, &lp.Tag{Key: key, Value: value})
}

// HasTag checks if a meta data tag with meta data's key equal to <key> is present in the list of meta data tags
func (m *ccMetric) HasMeta(key string) bool {
	for _, tag := range m.meta {
		if tag.Key == key {
			return true
		}
	}
	return false
}

// GetMeta returns the meta data tag with meta data's key equal to <key>
func (m *ccMetric) GetMeta(key string) (string, bool) {
	for _, tag := range m.meta {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

// RemoveMeta removes the meta data tag with tag's key equal to <key>
// and keeps the meta data tag list ordered by the keys
func (m *ccMetric) RemoveMeta(key string) {
	for i, tag := range m.meta {
		if tag.Key == key {
			copy(m.meta[i:], m.meta[i+1:])
			m.meta[len(m.meta)-1] = nil
			m.meta = m.meta[:len(m.meta)-1]
			return
		}
	}
}

// AddMeta adds a meta data tag (consisting of key and value)
// and keeps the meta data list ordered by the keys
func (m *ccMetric) AddMeta(key, value string) {
	for i, tag := range m.meta {
		if key > tag.Key {
			continue
		}

		if key == tag.Key {
			tag.Value = value
			return
		}

		m.meta = append(m.meta, nil)
		copy(m.meta[i+1:], m.meta[i:])
		m.meta[i] = &lp.Tag{Key: key, Value: value}
		return
	}

	m.meta = append(m.meta, &lp.Tag{Key: key, Value: value})
}

// AddField adds a field (consisting of key and value) to the unordered list of fields
func (m *ccMetric) AddField(key string, value interface{}) {
	for i, field := range m.fields {
		if key == field.Key {
			m.fields[i] = &lp.Field{Key: key, Value: convertField(value)}
			return
		}
	}
	m.fields = append(m.fields, &lp.Field{Key: key, Value: convertField(value)})
}

// GetField returns the field with field's key equal to <key>
func (m *ccMetric) GetField(key string) (interface{}, bool) {
	for _, field := range m.fields {
		if field.Key == key {
			return field.Value, true
		}
	}
	return "", false
}

// HasField checks if a field with field's key equal to <key> is present in the list of fields
func (m *ccMetric) HasField(key string) bool {
	for _, field := range m.fields {
		if field.Key == key {
			return true
		}
	}
	return false
}

// RemoveField removes the field with field's key equal to <key>
// from the unordered list of fields
func (m *ccMetric) RemoveField(key string) {
	for i, field := range m.fields {
		if field.Key == key {
			copy(m.fields[i:], m.fields[i+1:])
			m.fields[len(m.fields)-1] = nil
			m.fields = m.fields[:len(m.fields)-1]
			return
		}
	}
}

// New creates a new measurement point
func New(
	name string,
	tags map[string]string,
	meta map[string]string,
	fields map[string]interface{},
	tm time.Time,
) (CCMetric, error) {
	m := &ccMetric{
		name:   name,
		tags:   nil,
		fields: nil,
		tm:     tm,
		meta:   nil,
	}

	// Sorted list of tags
	if len(tags) > 0 {
		m.tags = make([]*lp.Tag, 0, len(tags))
		for k, v := range tags {
			m.tags = append(m.tags,
				&lp.Tag{Key: k, Value: v})
		}
		sort.Slice(m.tags, func(i, j int) bool { return m.tags[i].Key < m.tags[j].Key })
	}

	// Sorted list of meta data tags
	if len(meta) > 0 {
		m.meta = make([]*lp.Tag, 0, len(meta))
		for k, v := range meta {
			m.meta = append(m.meta,
				&lp.Tag{Key: k, Value: v})
		}
		sort.Slice(m.meta, func(i, j int) bool { return m.meta[i].Key < m.meta[j].Key })
	}

	// Unsorted list of fields
	if len(fields) > 0 {
		m.fields = make([]*lp.Field, 0, len(fields))
		for k, v := range fields {
			v := convertField(v)
			if v == nil {
				continue
			}
			m.AddField(k, v)
		}
	}

	return m, nil
}

// FromMetric copies the metric <other>
func FromMetric(other CCMetric) CCMetric {
	m := &ccMetric{
		name:   other.Name(),
		tags:   make([]*lp.Tag, len(other.TagList())),
		fields: make([]*lp.Field, len(other.FieldList())),
		meta:   make([]*lp.Tag, len(other.MetaList())),
		tm:     other.Time(),
	}

	for i, tag := range other.TagList() {
		m.tags[i] = &lp.Tag{Key: tag.Key, Value: tag.Value}
	}
	for i, s := range other.MetaList() {
		m.meta[i] = &lp.Tag{Key: s.Key, Value: s.Value}
	}

	for i, field := range other.FieldList() {
		m.fields[i] = &lp.Field{Key: field.Key, Value: field.Value}
	}
	return m
}

// FromInfluxMetric copies the influxDB line protocol metric <other>
func FromInfluxMetric(other lp.Metric) CCMetric {
	m := &ccMetric{
		name:   other.Name(),
		tags:   make([]*lp.Tag, len(other.TagList())),
		fields: make([]*lp.Field, len(other.FieldList())),
		meta:   make([]*lp.Tag, 0),
		tm:     other.Time(),
	}

	for i, otherTag := range other.TagList() {
		m.tags[i] = &lp.Tag{
			Key:   otherTag.Key,
			Value: otherTag.Value,
		}
	}

	for i, otherField := range other.FieldList() {
		m.fields[i] = &lp.Field{
			Key:   otherField.Key,
			Value: otherField.Value,
		}
	}
	return m
}

// convertField converts data types of fields by the following schemata:
//                         *float32, *float64,                      float32, float64 -> float64
//  *int,  *int8,  *int16,   *int32,   *int64,  int,  int8,  int16,   int32,   int64 ->   int64
// *uint, *uint8, *uint16,  *uint32,  *uint64, uint, uint8, uint16,  uint32,  uint64 ->  uint64
// *[]byte, *string,                           []byte, string                        -> string
// *bool,                                      bool                                  -> bool
func convertField(v interface{}) interface{} {
	switch v := v.(type) {
	case float64:
		return v
	case int64:
		return v
	case string:
		return v
	case bool:
		return v
	case int:
		return int64(v)
	case uint:
		return uint64(v)
	case uint64:
		return uint64(v)
	case []byte:
		return string(v)
	case int32:
		return int64(v)
	case int16:
		return int64(v)
	case int8:
		return int64(v)
	case uint32:
		return uint64(v)
	case uint16:
		return uint64(v)
	case uint8:
		return uint64(v)
	case float32:
		return float64(v)
	case *float64:
		if v != nil {
			return *v
		}
	case *int64:
		if v != nil {
			return *v
		}
	case *string:
		if v != nil {
			return *v
		}
	case *bool:
		if v != nil {
			return *v
		}
	case *int:
		if v != nil {
			return int64(*v)
		}
	case *uint:
		if v != nil {
			return uint64(*v)
		}
	case *uint64:
		if v != nil {
			return uint64(*v)
		}
	case *[]byte:
		if v != nil {
			return string(*v)
		}
	case *int32:
		if v != nil {
			return int64(*v)
		}
	case *int16:
		if v != nil {
			return int64(*v)
		}
	case *int8:
		if v != nil {
			return int64(*v)
		}
	case *uint32:
		if v != nil {
			return uint64(*v)
		}
	case *uint16:
		if v != nil {
			return uint64(*v)
		}
	case *uint8:
		if v != nil {
			return uint64(*v)
		}
	case *float32:
		if v != nil {
			return float64(*v)
		}
	default:
		return nil
	}
	return nil
}
