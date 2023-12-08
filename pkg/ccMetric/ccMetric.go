package ccmetric

import (
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	write "github.com/influxdata/influxdb-client-go/v2/api/write"
	lp "github.com/influxdata/line-protocol" // MIT license
	"golang.org/x/exp/maps"
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

// ccMetric access functions
type CCMetric interface {
	ToPoint(metaAsTags map[string]bool) *write.Point  // Generate influxDB point for data type ccMetric
	ToLineProtocol(metaAsTags map[string]bool) string // Generate influxDB line protocol for data type ccMetric

	Name() string        // Get metric name
	SetName(name string) // Set metric name

	Time() time.Time     // Get timestamp
	SetTime(t time.Time) // Set timestamp

	Tags() map[string]string                   // Map of tags
	AddTag(key, value string)                  // Add a tag
	GetTag(key string) (value string, ok bool) // Get a tag by its key
	HasTag(key string) (ok bool)               // Check if a tag key is present
	RemoveTag(key string)                      // Remove a tag by its key

	Meta() map[string]string                    // Map of meta data tags
	AddMeta(key, value string)                  // Add a meta data tag
	GetMeta(key string) (value string, ok bool) // Get a meta data tab addressed by its key
	HasMeta(key string) (ok bool)               // Check if a meta data key is present
	RemoveMeta(key string)                      // Remove a meta data tag by its key

	Fields() map[string]interface{}                   // Map of fields
	AddField(key string, value interface{})           // Add a field
	GetField(key string) (value interface{}, ok bool) // Get a field addressed by its key
	HasField(key string) (ok bool)                    // Check if a field key is present
	RemoveField(key string)                           // Remove a field addressed by its key
	String() string                                   // Return line-protocol like string
}

// String implements the stringer interface for data type ccMetric
func (m *ccMetric) String() string {
	return fmt.Sprintf(
		"Name: %s, Tags: %+v, Meta: %+v, fields: %+v, Timestamp: %d",
		m.name, m.tags, m.meta, m.fields, m.tm.UnixNano(),
	)
}

// ToLineProtocol generates influxDB line protocol for data type ccMetric
func (m *ccMetric) ToPoint(metaAsTags map[string]bool) (p *write.Point) {
	p = influxdb2.NewPoint(m.name, m.tags, m.fields, m.tm)
	for key, use_as_tag := range metaAsTags {
		if use_as_tag {
			if value, ok := m.GetMeta(key); ok {
				p.AddTag(key, value)
			}
		}
	}
	return p
}

// ToLineProtocol generates influxDB line protocol for data type ccMetric
func (m *ccMetric) ToLineProtocol(metaAsTags map[string]bool) string {

	return write.PointToLineProtocol(
		m.ToPoint(metaAsTags),
		time.Nanosecond,
	)
}

// Name returns the measurement name
func (m *ccMetric) Name() string {
	return m.name
}

// SetName sets the measurement name
func (m *ccMetric) SetName(name string) {
	m.name = name
}

// Time returns timestamp
func (m *ccMetric) Time() time.Time {
	return m.tm
}

// SetTime sets the timestamp
func (m *ccMetric) SetTime(t time.Time) {
	m.tm = t
}

// Tags returns the the list of tags as key-value-mapping
func (m *ccMetric) Tags() map[string]string {
	return m.tags
}

// AddTag adds a tag (consisting of key and value) to the map of tags
func (m *ccMetric) AddTag(key, value string) {
	m.tags[key] = value
}

// GetTag returns the tag with tag's key equal to <key>
func (m *ccMetric) GetTag(key string) (string, bool) {
	value, ok := m.tags[key]
	return value, ok
}

// HasTag checks if a tag with key equal to <key> is present in the list of tags
func (m *ccMetric) HasTag(key string) bool {
	_, ok := m.tags[key]
	return ok
}

// RemoveTag removes the tag with tag's key equal to <key>
func (m *ccMetric) RemoveTag(key string) {
	delete(m.tags, key)
}

// Meta returns the meta data tags as key-value mapping
func (m *ccMetric) Meta() map[string]string {
	return m.meta
}

// AddMeta adds a meta data tag (consisting of key and value) to the map of meta data tags
func (m *ccMetric) AddMeta(key, value string) {
	m.meta[key] = value
}

// GetMeta returns the meta data tag with meta data's key equal to <key>
func (m *ccMetric) GetMeta(key string) (string, bool) {
	value, ok := m.meta[key]
	return value, ok
}

// HasMeta checks if a meta data tag with meta data's key equal to <key> is present in the map of meta data tags
func (m *ccMetric) HasMeta(key string) bool {
	_, ok := m.meta[key]
	return ok
}

// RemoveMeta removes the meta data tag with tag's key equal to <key>
func (m *ccMetric) RemoveMeta(key string) {
	delete(m.meta, key)
}

// Fields returns the list of fields as key-value-mapping
func (m *ccMetric) Fields() map[string]interface{} {
	return m.fields
}

// AddField adds a field (consisting of key and value) to the map of fields
func (m *ccMetric) AddField(key string, value interface{}) {
	m.fields[key] = value
}

// GetField returns the field with field's key equal to <key>
func (m *ccMetric) GetField(key string) (interface{}, bool) {
	v, ok := m.fields[key]
	return v, ok
}

// HasField checks if a field with field's key equal to <key> is present in the map of fields
func (m *ccMetric) HasField(key string) bool {
	_, ok := m.fields[key]
	return ok
}

// RemoveField removes the field with field's key equal to <key>
// from the map of fields
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
		tags:   maps.Clone(tags),
		meta:   maps.Clone(meta),
		fields: make(map[string]interface{}, len(fields)),
		tm:     tm,
	}

	// deep copy fields
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
func FromMetric(other CCMetric) CCMetric {

	return &ccMetric{
		name:   other.Name(),
		tags:   maps.Clone(other.Tags()),
		meta:   maps.Clone(other.Meta()),
		fields: maps.Clone(other.Fields()),
		tm:     other.Time(),
	}
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

	// deep copy tags and fields
	for _, otherTag := range other.TagList() {
		m.tags[otherTag.Key] = otherTag.Value
	}
	for _, otherField := range other.FieldList() {
		m.fields[otherField.Key] = otherField.Value
	}
	return m
}

// convertField converts data types of fields by the following schemata:
//
//	                       *float32, *float64,                      float32, float64 -> float64
//	*int,  *int8,  *int16,   *int32,   *int64,  int,  int8,  int16,   int32,   int64 ->   int64
//
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
