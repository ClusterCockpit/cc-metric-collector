package sinks

/*
#cgo CFLAGS: -I./ganglia/include -DGM_PROTOCOL_GUARD
#cgo LDFLAGS: -L/usr/lib -lganglia
#include <stdlib.h>
#include <ganglia.h>
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const LIBGANGLIA = `libganglia.so`

type Ganglia2SinkConfig struct {
	defaultSinkConfig
	GmetricPath     string `json:"gmetricPath,omitempty"`
	GmetricConfig   string `json:"gmetricConfig,omitempty"`
	AddGangliaGroup bool   `json:"add_ganglia_group,omitempty"`
	AddTagsAsDesc   bool   `json:"add_tags_as_desc,omitempty"`
	AddTypeToName   bool   `json:"add_type_to_name,omitempty"`
	AddUnits        bool   `json:"add_units,omitempty"`
	ClusterName     string `json:"cluster_name,omitempty"`
	GangliaLib      string `json:"libganglia_path,omitempty"`
	ConfuseLib      string `json:"libconfuse_path,omitempty"`
}

type Ganglia2Sink struct {
	sink

	config         Ganglia2SinkConfig
	global_context C.Ganglia_pool
	send_channels  C.Ganglia_udp_send_channels
	constStr       map[string]*C.char
}

func (s *Ganglia2Sink) Init(config json.RawMessage) error {
	var err error = nil
	s.name = "Ganglia2Sink"
	s.config.AddTagsAsDesc = false
	s.config.AddGangliaGroup = false
	s.config.AddTypeToName = false
	s.config.AddUnits = true
	if len(config) > 0 {
		err = json.Unmarshal(config, &s.config)
		if err != nil {
			fmt.Println(s.name, "Error reading config for", s.name, ":", err.Error())
			return err
		}
	}
	s.constStr = make(map[string]*C.char)
	// s.constStr["globals"] = C.CString("globals")
	s.constStr["conffile"] = C.CString(s.config.GmetricConfig)
	// s.constStr["override_hostname"] = C.CString("override_hostname")
	// s.constStr["override_ip"] = C.CString("override_ip")
	s.constStr["GROUP"] = C.CString("GROUP")
	s.constStr["CLUSTER"] = C.CString("CLUSTER")
	if len(s.config.ClusterName) > 0 {
		s.constStr[s.config.ClusterName] = C.CString(s.config.ClusterName)
	}
	s.constStr["double"] = C.CString("double")
	s.constStr["int32"] = C.CString("int32")
	s.constStr["string"] = C.CString("string")
	s.constStr[""] = C.CString("")

	s.global_context = C.Ganglia_pool_create(nil)
	gmond_config := C.Ganglia_gmond_config_create(s.constStr["conffile"], 0)
	//globals := C.cfg_getsec(gmond_config, s.constStr["globals"])
	//override_hostname := C.cfg_getstr(globals, s.constStr["override_hostname"])
	//override_ip := C.cfg_getstr(globals, s.constStr["override_ip"])

	s.send_channels = C.Ganglia_udp_send_channels_create(s.global_context, gmond_config)
	return nil
}

func (s *Ganglia2Sink) Write(point lp.CCMetric) error {
	var err error = nil
	var c_value *C.char
	var c_type *C.char
	var c_unit *C.char

	lookup := func(key string) *C.char {
		if _, exist := s.constStr[key]; !exist {
			s.constStr[key] = C.CString(key)
		}
		return s.constStr[key]
	}
	c_name := lookup(point.Name())
	value, ok := point.GetField("value")
	if !ok {
		return fmt.Errorf("metric %s has not 'value' field", point.Name())
	}
	switch real := value.(type) {
	case float64:
		c_value = C.CString(fmt.Sprintf("%f", real))
		c_type = lookup("double")
	case float32:
		c_value = C.CString(fmt.Sprintf("%f", real))
		c_type = lookup("float")
	case int64:
		c_value = C.CString(fmt.Sprintf("%d", real))
		c_type = lookup("int32")
	case int32:
		c_value = C.CString(fmt.Sprintf("%d", real))
		c_type = lookup("int32")
	case int:
		c_value = C.CString(fmt.Sprintf("%d", real))
		c_type = lookup("int32")
	case string:
		c_value = C.CString(real)
		c_type = lookup("string")
	default:
		C.free(unsafe.Pointer(c_name))
		return fmt.Errorf("metric %s has invalid 'value' type for %s", point.Name(), s.name)
	}
	if tagunit, tagok := point.GetTag("unit"); tagok {
		c_unit = lookup(tagunit)
	} else if metaunit, metaok := point.GetMeta("unit"); metaok {
		c_unit = lookup(metaunit)
	} else {
		c_unit = lookup("")
	}

	gmetric := C.Ganglia_metric_create(s.global_context)
	rval := C.int(0)
	rval = C.Ganglia_metric_set(gmetric, c_name, c_value, c_type, c_unit, C.GANGLIA_SLOPE_BOTH, 0, 0)
	switch rval {
	case 1:
		C.free(unsafe.Pointer(c_value))
		return errors.New("invalid parameters")
	case 2:
		C.free(unsafe.Pointer(c_value))
		return errors.New("one of your parameters has an invalid character '\"'")
	case 3:
		C.free(unsafe.Pointer(c_value))
		return fmt.Errorf("the type parameter \"%s\" is not a valid type", C.GoString(c_type))
	case 4:
		C.free(unsafe.Pointer(c_value))
		return fmt.Errorf("the value parameter \"%s\" does not represent a number", C.GoString(c_value))
	default:
	}
	if len(s.config.ClusterName) > 0 {
		C.Ganglia_metadata_add(gmetric, lookup("CLUSTER"), lookup(s.config.ClusterName))
	}
	if group, ok := point.GetMeta("group"); ok {
		c_group := lookup(group)
		C.Ganglia_metadata_add(gmetric, lookup("GROUP"), c_group)
	}
	rval = C.Ganglia_metric_send(gmetric, s.send_channels)
	if rval != 0 {
		err = fmt.Errorf("there was an error sending metric %s to %d of the send channels ", point.Name(), rval)
		// fall throuph to use Ganglia_metric_destroy from common cleanup
	}
	C.Ganglia_metric_destroy(gmetric)
	C.free(unsafe.Pointer(c_value))
	return err
}

func (s *Ganglia2Sink) Flush() error {
	return nil
}

func (s *Ganglia2Sink) Close() {
	C.Ganglia_pool_destroy(s.global_context)

	for _, cstr := range s.constStr {
		C.free(unsafe.Pointer(cstr))
	}
}
