package messageprocessor

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
)

func generate_message_lists(num_lists, num_entries int) ([][]lp.CCMessage, error) {
	mlist := make([][]lp.CCMessage, 0)
	for j := 0; j < num_lists; j++ {
		out := make([]lp.CCMessage, 0)
		for i := 0; i < num_entries; i++ {
			var x lp.CCMessage
			var err error = nil
			switch {
			case i%4 == 0:
				x, err = lp.NewEvent("myevent", map[string]string{"type": "socket", "type-id": "0"}, map[string]string{}, "nothing happend", time.Now())
			case i%4 == 1:
				x, err = lp.NewMetric("mymetric", map[string]string{"type": "socket", "type-id": "0"}, map[string]string{"unit": "kByte"}, 12.145, time.Now())
			case i%4 == 2:
				x, err = lp.NewLog("mylog", map[string]string{"type": "socket", "type-id": "0"}, map[string]string{}, "disk status: OK", time.Now())
			case i%4 == 3:
				x, err = lp.NewGetControl("mycontrol", map[string]string{"type": "socket", "type-id": "0"}, map[string]string{}, time.Now())
			}
			if err == nil {
				x.AddTag("hostname", "myhost")
				out = append(out, x)
			} else {
				return nil, errors.New("failed to create message")
			}
		}
		mlist = append(mlist, out)
	}
	return mlist, nil
}

func TestNewMessageProcessor(t *testing.T) {
	_, err := NewMessageProcessor()
	if err != nil {
		t.Error(err.Error())
	}
}

type Configs struct {
	name   string
	config json.RawMessage
	drop   bool
	errors bool
	pre    func(msg lp.CCMessage) error
	check  func(msg lp.CCMessage) error
}

var test_configs = []Configs{
	{
		name:   "single_dropif_nomatch",
		config: json.RawMessage(`{"drop_messages_if": [ "name == 'testname' && tags.type == 'socket' && tags.typeid % 2 == 1"]}`),
	},
	{
		name:   "drop_by_name",
		config: json.RawMessage(`{"drop_messages": [ "net_bytes_in"]}`),
		drop:   true,
	},
	{
		name:   "drop_by_type_match",
		config: json.RawMessage(`{"drop_by_message_type": [ "metric"]}`),
		drop:   true,
	},
	{
		name:   "drop_by_type_nomatch",
		config: json.RawMessage(`{"drop_by_message_type": [ "event"]}`),
	},
	{
		name:   "single_dropif_match",
		config: json.RawMessage(`{"drop_messages_if": [ "name == 'net_bytes_in' && tags.type == 'node'"]}`),
		drop:   true,
	},
	{
		name:   "double_dropif_match_nomatch",
		config: json.RawMessage(`{"drop_messages_if": [ "name == 'net_bytes_in' && tags.type == 'node'", "name == 'testname' && tags.type == 'socket' && tags.typeid % 2 == 1"]}`),
		drop:   true,
	},
	{
		name:   "rename_simple",
		config: json.RawMessage(`{"rename_messages": { "net_bytes_in" : "net_bytes_out", "rapl_power": "cpu_power"}}`),
		check: func(msg lp.CCMessage) error {
			if msg.Name() != "net_bytes_out" {
				return errors.New("expected name net_bytes_out but still have net_bytes_in")
			}
			return nil
		},
	},
	{
		name:   "rename_match",
		config: json.RawMessage(`{"rename_messages_if": { "name == 'net_bytes_in'" : "net_bytes_out", "name == 'rapl_power'": "cpu_power"}}`),
		check: func(msg lp.CCMessage) error {
			if msg.Name() != "net_bytes_out" {
				return errors.New("expected name net_bytes_out but still have net_bytes_in")
			}
			return nil
		},
	},
	{
		name:   "rename_nomatch",
		config: json.RawMessage(`{"rename_messages_if": { "name == 'net_bytes_out'" : "net_bytes_in", "name == 'rapl_power'": "cpu_power"}}`),
		check: func(msg lp.CCMessage) error {
			if msg.Name() != "net_bytes_in" {
				return errors.New("expected name net_bytes_in but still have net_bytes_out")
			}
			return nil
		},
	},
	{
		name:   "add_tag",
		config: json.RawMessage(`{"add_tags_if": [{"if": "name == 'net_bytes_in'", "key" : "cluster", "value" : "mycluster"}]}`),
		check: func(msg lp.CCMessage) error {
			if !msg.HasTag("cluster") {
				return errors.New("expected new tag 'cluster' but not present")
			}
			return nil
		},
	},
	{
		name:   "del_tag",
		config: json.RawMessage(`{"delete_tags_if": [{"if": "name == 'net_bytes_in'", "key" : "type"}]}`),
		check: func(msg lp.CCMessage) error {
			if msg.HasTag("type") {
				return errors.New("expected to have no 'type' but still present")
			}
			return nil
		},
	},
	{
		name:   "add_meta",
		config: json.RawMessage(`{"add_meta_if": [{"if": "name == 'net_bytes_in'", "key" : "source", "value" : "example"}]}`),
		check: func(msg lp.CCMessage) error {
			if !msg.HasMeta("source") {
				return errors.New("expected new tag 'source' but not present")
			}
			return nil
		},
	},
	{
		name:   "del_meta",
		config: json.RawMessage(`{"delete_meta_if": [{"if": "name == 'net_bytes_in'", "key" : "unit"}]}`),
		check: func(msg lp.CCMessage) error {
			if msg.HasMeta("unit") {
				return errors.New("expected to have no 'unit' but still present")
			}
			return nil
		},
	},
	{
		name:   "add_field",
		config: json.RawMessage(`{"add_fields_if": [{"if": "name == 'net_bytes_in'", "key" : "myfield", "value" : "example"}]}`),
		check: func(msg lp.CCMessage) error {
			if !msg.HasField("myfield") {
				return errors.New("expected new tag 'source' but not present")
			}
			return nil
		},
	},
	{
		name:   "delete_fields_if_protected",
		config: json.RawMessage(`{"delete_fields_if": [{"if": "name == 'net_bytes_in'", "key" : "value"}]}`),
		errors: true,
		check: func(msg lp.CCMessage) error {
			if !msg.HasField("value") {
				return errors.New("expected to still have 'value' field because it is a protected field key")
			}
			return nil
		},
	},
	{
		name:   "delete_fields_if_unprotected",
		config: json.RawMessage(`{"delete_fields_if": [{"if": "name == 'net_bytes_in'", "key" : "testfield"}]}`),
		check: func(msg lp.CCMessage) error {
			if msg.HasField("testfield") {
				return errors.New("expected to still have 'testfield' field but should be deleted")
			}
			return nil
		},
		pre: func(msg lp.CCMessage) error {
			msg.AddField("testfield", 4.123)
			return nil
		},
	},
	{
		name:   "single_change_prefix_match",
		config: json.RawMessage(`{"change_unit_prefix": {"name == 'net_bytes_in' && tags.type == 'node'": "M"}}`),
		check: func(msg lp.CCMessage) error {
			if u, ok := msg.GetMeta("unit"); ok {
				if u != "MB" {
					return fmt.Errorf("expected unit MB but have %s", u)
				}
			} else if u, ok := msg.GetTag("unit"); ok {
				if u != "MB" {
					return fmt.Errorf("expected unit MB but have %s", u)
				}
			}
			return nil
		},
	},
	{
		name:   "normalize_units",
		config: json.RawMessage(`{"normalize_units": true}`),
		check: func(msg lp.CCMessage) error {
			if u, ok := msg.GetMeta("unit"); ok {
				if u != "B" {
					return fmt.Errorf("expected unit B but have %s", u)
				}
			} else if u, ok := msg.GetTag("unit"); ok {
				if u != "B" {
					return fmt.Errorf("expected unit B but have %s", u)
				}
			}
			return nil
		},
	},
	{
		name:   "move_tag_to_meta",
		config: json.RawMessage(`{"move_tag_to_meta_if": [{"if": "name == 'net_bytes_in'", "key" : "type-id", "value": "typeid"}]}`),
		check: func(msg lp.CCMessage) error {
			if msg.HasTag("type-id") || !msg.HasMeta("typeid") {
				return errors.New("moving tag 'type-id' to meta 'typeid' failed")
			}
			return nil
		},
		pre: func(msg lp.CCMessage) error {
			msg.AddTag("type-id", "0")
			return nil
		},
	},
	{
		name:   "move_tag_to_field",
		config: json.RawMessage(`{"move_tag_to_field_if": [{"if": "name == 'net_bytes_in'", "key" : "type-id", "value": "typeid"}]}`),
		check: func(msg lp.CCMessage) error {
			if msg.HasTag("type-id") || !msg.HasField("typeid") {
				return errors.New("moving tag 'type-id' to field 'typeid' failed")
			}
			return nil
		},
		pre: func(msg lp.CCMessage) error {
			msg.AddTag("type-id", "0")
			return nil
		},
	},
	{
		name:   "move_meta_to_tag",
		config: json.RawMessage(`{"move_meta_to_tag_if": [{"if": "name == 'net_bytes_in'", "key" : "unit", "value": "unit"}]}`),
		check: func(msg lp.CCMessage) error {
			if msg.HasMeta("unit") || !msg.HasTag("unit") {
				return errors.New("moving meta 'unit' to tag 'unit' failed")
			}
			return nil
		},
	},
	{
		name:   "move_meta_to_field",
		config: json.RawMessage(`{"move_meta_to_field_if": [{"if": "name == 'net_bytes_in'", "key" : "unit", "value": "unit"}]}`),
		check: func(msg lp.CCMessage) error {
			if msg.HasMeta("unit") || !msg.HasField("unit") {
				return errors.New("moving meta 'unit' to field 'unit' failed")
			}
			return nil
		},
	},
	{
		name:   "move_field_to_tag",
		config: json.RawMessage(`{"move_field_to_tag_if": [{"if": "name == 'net_bytes_in'", "key" : "myfield", "value": "field"}]}`),
		check: func(msg lp.CCMessage) error {
			if msg.HasField("myfield") || !msg.HasTag("field") {
				return errors.New("moving meta 'myfield' to tag 'field' failed")
			}
			return nil
		},
		pre: func(msg lp.CCMessage) error {
			msg.AddField("myfield", 12)
			return nil
		},
	},
	{
		name:   "move_field_to_meta",
		config: json.RawMessage(`{"move_field_to_meta_if": [{"if": "name == 'net_bytes_in'", "key" : "myfield", "value": "field"}]}`),
		check: func(msg lp.CCMessage) error {
			if msg.HasField("myfield") || !msg.HasMeta("field") {
				return errors.New("moving meta 'myfield' to meta 'field' failed")
			}
			return nil
		},
		pre: func(msg lp.CCMessage) error {
			msg.AddField("myfield", 12)
			return nil
		},
	},
}

func TestConfigList(t *testing.T) {
	for _, c := range test_configs {
		t.Run(c.name, func(t *testing.T) {
			m, err := lp.NewMetric("net_bytes_in", map[string]string{"type": "node", "type-id": "0"}, map[string]string{"unit": "Byte"}, float64(1024.0), time.Now())
			if err != nil {
				t.Error(err.Error())
				return
			}
			if c.pre != nil {
				if err = c.pre(m); err != nil {
					t.Errorf("error running pre-test function: %v", err.Error())
					return
				}
			}

			mp, err := NewMessageProcessor()
			if err != nil {
				t.Error(err.Error())
				return
			}
			err = mp.FromConfigJSON(c.config)
			if err != nil {
				t.Error(err.Error())
				return
			}
			//t.Log(m.ToLineProtocol(nil))
			drop, err := mp.ProcessMessage(m)
			if err != nil && !c.errors {
				cclog.SetDebug()
				mp.ProcessMessage(m)
				t.Error(err.Error())
				return
			}
			if drop != c.drop {
				if c.drop {
					t.Error("fail, message should be dropped but processor signalled NO dropping")
				} else {
					t.Error("fail, message should NOT be dropped but processor signalled dropping")
				}
				cclog.SetDebug()
				mp.ProcessMessage(m)
			}
			if c.check != nil {
				if err := c.check(m); err != nil {
					t.Errorf("check failed with %v", err.Error())
					t.Log("Rerun with debugging")
					cclog.SetDebug()
					mp.ProcessMessage(m)
				}
			}
		})
	}
}

func BenchmarkProcessing(b *testing.B) {

	mlist, err := generate_message_lists(b.N, 1000)
	if err != nil {
		b.Error(err.Error())
		return
	}

	mp, err := NewMessageProcessor()
	if err != nil {
		b.Error(err.Error())
		return
	}
	err = mp.FromConfigJSON(json.RawMessage(`{"move_meta_to_tag_if": [{"if" : "name == 'mymetric'", "key":"unit", "value":"unit"}]}`))
	if err != nil {
		b.Error(err.Error())
		return
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for _, m := range mlist[i] {
			if _, err := mp.ProcessMessage(m); err != nil {
				b.Errorf("failed processing message '%s': %v", m.ToLineProtocol(nil), err.Error())
				return
			}
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(b.Elapsed())/float64(len(mlist)*b.N), "ns/message")
}
