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

type ccMetric struct {
	name   string
	tags   []*lp.Tag
	fields []*lp.Field
	tm     time.Time
	meta   []*lp.Tag
}

type CCMetric interface {
	lp.MutableMetric
	AddMeta(key, value string)
	MetaList() []*lp.Tag
	RemoveTag(key string)
	GetTag(key string) (string, bool)
	GetMeta(key string) (string, bool)
	GetField(key string) (interface{}, bool)
	HasField(key string) bool
	RemoveField(key string)
}

func (m *ccMetric) Meta() map[string]string {
	meta := make(map[string]string, len(m.meta))
	for _, m := range m.meta {
		meta[m.Key] = m.Value
	}
	return meta
}

func (m *ccMetric) MetaList() []*lp.Tag {
	return m.meta
}

func (m *ccMetric) String() string {
	return fmt.Sprintf("%s %v %v %v %d", m.name, m.Tags(), m.Meta(), m.Fields(), m.tm.UnixNano())
}

func (m *ccMetric) Name() string {
	return m.name
}

func (m *ccMetric) Tags() map[string]string {
	tags := make(map[string]string, len(m.tags))
	for _, tag := range m.tags {
		tags[tag.Key] = tag.Value
	}
	return tags
}

func (m *ccMetric) TagList() []*lp.Tag {
	return m.tags
}

func (m *ccMetric) Fields() map[string]interface{} {
	fields := make(map[string]interface{}, len(m.fields))
	for _, field := range m.fields {
		fields[field.Key] = field.Value
	}

	return fields
}

func (m *ccMetric) FieldList() []*lp.Field {
	return m.fields
}

func (m *ccMetric) Time() time.Time {
	return m.tm
}

func (m *ccMetric) SetTime(t time.Time) {
	m.tm = t
}

func (m *ccMetric) HasTag(key string) bool {
	for _, tag := range m.tags {
		if tag.Key == key {
			return true
		}
	}
	return false
}

func (m *ccMetric) GetTag(key string) (string, bool) {
	for _, tag := range m.tags {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

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

func (m *ccMetric) HasMeta(key string) bool {
	for _, tag := range m.meta {
		if tag.Key == key {
			return true
		}
	}
	return false
}

func (m *ccMetric) GetMeta(key string) (string, bool) {
	for _, tag := range m.meta {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return "", false
}

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

func (m *ccMetric) AddField(key string, value interface{}) {
	for i, field := range m.fields {
		if key == field.Key {
			m.fields[i] = &lp.Field{Key: key, Value: convertField(value)}
			return
		}
	}
	m.fields = append(m.fields, &lp.Field{Key: key, Value: convertField(value)})
}

func (m *ccMetric) GetField(key string) (interface{}, bool) {
	for _, field := range m.fields {
		if field.Key == key {
			return field.Value, true
		}
	}
	return "", false
}

func (m *ccMetric) HasField(key string) bool {
	for _, field := range m.fields {
		if field.Key == key {
			return true
		}
	}
	return false
}

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

	if len(tags) > 0 {
		m.tags = make([]*lp.Tag, 0, len(tags))
		for k, v := range tags {
			m.tags = append(m.tags,
				&lp.Tag{Key: k, Value: v})
		}
		sort.Slice(m.tags, func(i, j int) bool { return m.tags[i].Key < m.tags[j].Key })
	}

	if len(meta) > 0 {
		m.meta = make([]*lp.Tag, 0, len(meta))
		for k, v := range meta {
			m.meta = append(m.meta,
				&lp.Tag{Key: k, Value: v})
		}
		sort.Slice(m.meta, func(i, j int) bool { return m.meta[i].Key < m.meta[j].Key })
	}

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

func FromInfluxMetric(other lp.Metric) CCMetric {
	m := &ccMetric{
		name:   other.Name(),
		tags:   make([]*lp.Tag, len(other.TagList())),
		fields: make([]*lp.Field, len(other.FieldList())),
		meta:   make([]*lp.Tag, 0),
		tm:     other.Time(),
	}

	for i, tag := range other.TagList() {
		m.tags[i] = &lp.Tag{Key: tag.Key, Value: tag.Value}
	}

	for i, field := range other.FieldList() {
		m.fields[i] = &lp.Field{Key: field.Key, Value: field.Value}
	}
	return m
}

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
