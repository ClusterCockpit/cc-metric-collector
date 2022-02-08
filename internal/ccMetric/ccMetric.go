package ccmetric

import (
	"fmt"
	"sort"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	write "github.com/influxdata/influxdb-client-go/v2/api/write"
	lp "github.com/influxdata/line-protocol" // MIT license
)

// Most functions are derived from github.com/influxdata/line-protocol/metric.go
// The metric type is extended with an extra meta information list re-using the Tag
// type.
//
// See: https://docs.influxdata.com/influxdb/latest/reference/syntax/line-protocol/
type ccMetric struct {
	name   string                 // Measurement name
	meta   map[string]string      // map of meta data tags
	tags   map[string]string      // map of of tags
	fields map[string]interface{} // map of of fields
	tm     time.Time              // timestamp
}

// ccmetric access functions
type CCMetric interface {
	lp.Metric                              // Time(), Name(), TagList(), FieldList()
	ToLineProtocol(metaAsTags bool) string // Generate influxDB line protocol for data type ccMetric
	ToPoint(metaAsTags bool) *write.Point  // Generate influxDB point for data type ccMetric

	SetName(name string) // Set metric name
	SetTime(t time.Time) // Set timestamp

	Tags() map[string]string                   // Map of tags
	TagList() []*lp.Tag                        // Ordered list of tags
	AddTag(key, value string)                  // Add a tag
	GetTag(key string) (value string, ok bool) // Get a tag by its key
	HasTag(key string) (ok bool)               // Check a tag
	RemoveTag(key string)                      // Remove a tag by its key

	Meta() map[string]string                    // Map of meta data tags
	MetaList() []*lp.Tag                        // Ordered list of meta data
	AddMeta(key, value string)                  // Add a meta data tag
	GetMeta(key string) (value string, ok bool) // Get a meta data tab addressed by its key
	HasMeta(key string) (ok bool)               // Check a meta data tag
	RemoveMeta(key string)                      // Remove a meta data tag by its key

	Fields() map[string]interface{}                   // Map of fields
	FieldList() []*lp.Field                           // Ordered list of fields
	AddField(key string, value interface{})           // Add a field
	GetField(key string) (value interface{}, ok bool) // Get a field addressed by its key
	HasField(key string) (ok bool)                    // Check if a field key is present
	RemoveField(key string)                           // Remove a field addressed by its key
}

// Meta returns the meta data tags as key-value mapping
func (m *ccMetric) Meta() map[string]string {
	return m.meta
}

// MetaList returns the the list of meta data tags as sorted list of key value tags
func (m *ccMetric) MetaList() []*lp.Tag {

	ml := make([]*lp.Tag, 0, len(m.meta))
	for key, value := range m.meta {
		ml = append(ml, &lp.Tag{Key: key, Value: value})
	}
	sort.Slice(ml, func(i, j int) bool { return ml[i].Key < ml[j].Key })
	return ml
}

// String implements the stringer interface for data type ccMetric
func (m *ccMetric) String() string {
	return fmt.Sprintf("Name: %s, Tags: %+v, Meta: %+v, fields: %+v, Timestamp: %d", m.name, m.tags, m.meta, m.fields, m.tm.UnixNano())
}

// ToLineProtocol generates influxDB line protocol for data type ccMetric
func (m *ccMetric) ToPoint(metaAsTags bool) (p *write.Point) {

	if !metaAsTags {
		p = influxdb2.NewPoint(m.name, m.tags, m.fields, m.tm)
	} else {
		tags := make(map[string]string, len(m.tags)+len(m.meta))
		for key, value := range m.tags {
			tags[key] = value
		}
		for key, value := range m.meta {
			tags[key] = value
		}
		p = influxdb2.NewPoint(m.name, tags, m.fields, m.tm)
	}
	return
}

// ToLineProtocol generates influxDB line protocol for data type ccMetric
func (m *ccMetric) ToLineProtocol(metaAsTags bool) string {

	return write.PointToLineProtocol(
		m.ToPoint(metaAsTags),
		time.Nanosecond,
	)
}

// Name returns the measurement name
func (m *ccMetric) Name() string {
	return m.name
}

func (m *ccMetric) SetName(name string) {
	m.name = name
}

// Tags returns the the list of tags as key-value-mapping
func (m *ccMetric) Tags() map[string]string {
	return m.tags
}

// TagList returns the the list of tags as sorted list of key value tags
func (m *ccMetric) TagList() []*lp.Tag {
	tl := make([]*lp.Tag, 0, len(m.tags))
	for key, value := range m.tags {
		tl = append(tl, &lp.Tag{Key: key, Value: value})
	}
	sort.Slice(tl, func(i, j int) bool { return tl[i].Key < tl[j].Key })
	return tl
}

// Fields returns the list of fields as key-value-mapping
func (m *ccMetric) Fields() map[string]interface{} {
	return m.fields
}

// FieldList returns the list of fields
func (m *ccMetric) FieldList() []*lp.Field {
	fieldList := make([]*lp.Field, 0, len(m.fields))
	for key, value := range m.fields {
		fieldList = append(fieldList, &lp.Field{Key: key, Value: value})
	}
	return fieldList
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
	_, ok := m.tags[key]
	return ok
}

// GetTag returns the tag with tag's key equal to <key>
func (m *ccMetric) GetTag(key string) (string, bool) {
	value, ok := m.tags[key]
	return value, ok
}

// RemoveTag removes the tag with tag's key equal to <key>
// and keeps the tag list ordered by the keys
func (m *ccMetric) RemoveTag(key string) {
	delete(m.tags, key)
}

// AddTag adds a tag (consisting of key and value)
// and keeps the tag list ordered by the keys
func (m *ccMetric) AddTag(key, value string) {
	m.tags[key] = value
}

// HasTag checks if a meta data tag with meta data's key equal to <key> is present in the list of meta data tags
func (m *ccMetric) HasMeta(key string) bool {
	_, ok := m.meta[key]
	return ok
}

// GetMeta returns the meta data tag with meta data's key equal to <key>
func (m *ccMetric) GetMeta(key string) (string, bool) {
	value, ok := m.meta[key]
	return value, ok
}

// RemoveMeta removes the meta data tag with tag's key equal to <key>
// and keeps the meta data tag list ordered by the keys
func (m *ccMetric) RemoveMeta(key string) {
	delete(m.meta, key)
}

// AddMeta adds a meta data tag (consisting of key and value)
// and keeps the meta data list ordered by the keys
func (m *ccMetric) AddMeta(key, value string) {
	m.meta[key] = value
}

// AddField adds a field (consisting of key and value) to the unordered list of fields
func (m *ccMetric) AddField(key string, value interface{}) {
	m.fields[key] = value
}

// GetField returns the field with field's key equal to <key>
func (m *ccMetric) GetField(key string) (interface{}, bool) {
	v, ok := m.fields[key]
	return v, ok
}

// HasField checks if a field with field's key equal to <key> is present in the list of fields
func (m *ccMetric) HasField(key string) bool {
	_, ok := m.fields[key]
	return ok
}

// RemoveField removes the field with field's key equal to <key>
// from the unordered list of fields
func (m *ccMetric) RemoveField(key string) {
	delete(m.fields, key)
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
		tags:   make(map[string]string, len(tags)),
		meta:   make(map[string]string, len(meta)),
		fields: make(map[string]interface{}, len(fields)),
		tm:     tm,
	}

	// deep copy tags
	for k, v := range tags {
		m.tags[k] = v
	}

	// deep copy meta data tags
	for k, v := range meta {
		m.meta[k] = v
	}

	// Unsorted list of fields
	for k, v := range fields {
		v := convertField(v)
		if v == nil {
			continue
		}
		m.fields[k] = v
	}

	return m, nil
}

// FromMetric copies the metric <other>
func FromMetric(other ccMetric) CCMetric {
	m := &ccMetric{
		name:   other.Name(),
		tags:   make(map[string]string),
		meta:   make(map[string]string),
		fields: make(map[string]interface{}),
		tm:     other.Time(),
	}

	for key, value := range other.tags {
		m.tags[key] = value
	}
	for key, value := range other.meta {
		m.meta[key] = value
	}
	for key, value := range other.fields {
		m.fields[key] = value
	}
	return m
}

// FromInfluxMetric copies the influxDB line protocol metric <other>
func FromInfluxMetric(other lp.Metric) CCMetric {
	m := &ccMetric{
		name:   other.Name(),
		tags:   make(map[string]string),
		meta:   make(map[string]string),
		fields: make(map[string]interface{}),
		tm:     other.Time(),
	}

	for _, otherTag := range other.TagList() {
		m.tags[otherTag.Key] = otherTag.Value
	}
	for _, otherField := range other.FieldList() {
		m.fields[otherField.Key] = otherField.Value
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
