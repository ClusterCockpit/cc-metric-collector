package sinks

/*
#cgo CFLAGS: -DGM_PROTOCOL_GUARD
#cgo LDFLAGS: -L. -lganglia
#include <stdlib.h>

// This is a copy&paste snippet of ganglia.h (BSD-3 license)
// See https://github.com/ganglia/monitor-core
// for further information

enum ganglia_slope {
   GANGLIA_SLOPE_ZERO = 0,
   GANGLIA_SLOPE_POSITIVE,
   GANGLIA_SLOPE_NEGATIVE,
   GANGLIA_SLOPE_BOTH,
   GANGLIA_SLOPE_UNSPECIFIED,
   GANGLIA_SLOPE_DERIVATIVE,
   GANGLIA_SLOPE_LAST_LEGAL_VALUE=GANGLIA_SLOPE_DERIVATIVE
};
typedef enum ganglia_slope ganglia_slope_t;

typedef struct Ganglia_pool* Ganglia_pool;
typedef struct Ganglia_gmond_config* Ganglia_gmond_config;
typedef struct Ganglia_udp_send_channels* Ganglia_udp_send_channels;

struct Ganglia_metric {
   Ganglia_pool pool;
   struct Ganglia_metadata_message *msg;
   char *value;
   void *extra;
};
typedef struct Ganglia_metric * Ganglia_metric;

#ifdef __cplusplus
extern "C" {
#endif

Ganglia_gmond_config Ganglia_gmond_config_create(char *path, int fallback_to_default);
//void Ganglia_gmond_config_destroy(Ganglia_gmond_config config);

Ganglia_udp_send_channels Ganglia_udp_send_channels_create(Ganglia_pool p, Ganglia_gmond_config config);
void Ganglia_udp_send_channels_destroy(Ganglia_udp_send_channels channels);

int Ganglia_udp_send_message(Ganglia_udp_send_channels channels, char *buf, int len );

Ganglia_metric Ganglia_metric_create( Ganglia_pool parent_pool );
int Ganglia_metric_set( Ganglia_metric gmetric, char *name, char *value, char *type, char *units, unsigned int slope, unsigned int tmax, unsigned int dmax);
int Ganglia_metric_send( Ganglia_metric gmetric, Ganglia_udp_send_channels send_channels );
//int Ganglia_metadata_send( Ganglia_metric gmetric, Ganglia_udp_send_channels send_channels );
//int Ganglia_metadata_send_real( Ganglia_metric gmetric, Ganglia_udp_send_channels send_channels, char *override_string );
void Ganglia_metadata_add( Ganglia_metric gmetric, char *name, char *value );
//int Ganglia_value_send( Ganglia_metric gmetric, Ganglia_udp_send_channels send_channels );
void Ganglia_metric_destroy( Ganglia_metric gmetric );

Ganglia_pool Ganglia_pool_create( Ganglia_pool parent );
void Ganglia_pool_destroy( Ganglia_pool pool );

//ganglia_slope_t cstr_to_slope(const char* str);
//const char*     slope_to_cstr(unsigned int slope);

#ifdef __cplusplus
}
#endif
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

const GMOND_CONFIG_FILE = `/var/ganglia/gmond.conf`

type LibgangliaSinkConfig struct {
	defaultSinkConfig
	GmondConfig     string `json:"gmond_config,omitempty"`
	AddGangliaGroup bool   `json:"add_ganglia_group,omitempty"`
	//AddTagsAsDesc   bool   `json:"add_tags_as_desc,omitempty"`
	AddTypeToName bool   `json:"add_type_to_name,omitempty"`
	AddUnits      bool   `json:"add_units,omitempty"`
	ClusterName   string `json:"cluster_name,omitempty"`
}

type LibgangliaSink struct {
	sink
	config         LibgangliaSinkConfig
	global_context C.Ganglia_pool
	gmond_config   C.Ganglia_gmond_config
	send_channels  C.Ganglia_udp_send_channels
	cstrCache      map[string]*C.char
}

func (s *LibgangliaSink) Init(config json.RawMessage) error {
	var err error = nil
	s.name = "LibgangliaSink"
	//s.config.AddTagsAsDesc = false
	s.config.AddGangliaGroup = false
	s.config.AddTypeToName = false
	s.config.AddUnits = true
	s.config.GmondConfig = string(GMOND_CONFIG_FILE)
	if len(config) > 0 {
		err = json.Unmarshal(config, &s.config)
		if err != nil {
			fmt.Println(s.name, "Error reading config for", s.name, ":", err.Error())
			return err
		}
	}

	// Set up cache for the C strings
	s.cstrCache = make(map[string]*C.char)
	// s.cstrCache["globals"] = C.CString("globals")

	// s.cstrCache["override_hostname"] = C.CString("override_hostname")
	// s.cstrCache["override_ip"] = C.CString("override_ip")

	// Add some constant strings
	s.cstrCache["GROUP"] = C.CString("GROUP")
	s.cstrCache["CLUSTER"] = C.CString("CLUSTER")
	s.cstrCache[""] = C.CString("")

	// Add cluster name for lookup in Write()
	if len(s.config.ClusterName) > 0 {
		s.cstrCache[s.config.ClusterName] = C.CString(s.config.ClusterName)
	}
	// Add supported types for later lookup in Write()
	s.cstrCache["double"] = C.CString("double")
	s.cstrCache["int32"] = C.CString("int32")
	s.cstrCache["string"] = C.CString("string")

	// Create Ganglia pool
	s.global_context = C.Ganglia_pool_create(nil)
	// Load Ganglia configuration
	s.cstrCache[s.config.GmondConfig] = C.CString(s.config.GmondConfig)
	s.gmond_config = C.Ganglia_gmond_config_create(s.cstrCache[s.config.GmondConfig], 0)
	//globals := C.cfg_getsec(gmond_config, s.cstrCache["globals"])
	//override_hostname := C.cfg_getstr(globals, s.cstrCache["override_hostname"])
	//override_ip := C.cfg_getstr(globals, s.cstrCache["override_ip"])

	s.send_channels = C.Ganglia_udp_send_channels_create(s.global_context, s.gmond_config)
	return nil
}

func (s *LibgangliaSink) Write(point lp.CCMetric) error {
	var err error = nil
	var c_name *C.char
	var c_value *C.char
	var c_type *C.char
	var c_unit *C.char

	// helper function for looking up C strings in the cache
	lookup := func(key string) *C.char {
		if _, exist := s.cstrCache[key]; !exist {
			s.cstrCache[key] = C.CString(key)
		}
		return s.cstrCache[key]
	}

	// Get metric name
	if s.config.AddTypeToName {
		c_name = lookup(gangliaMetricName(point))
	} else {
		c_name = lookup(point.Name())
	}

	// Get the value C string and lookup the type string in the cache
	value, ok := point.GetField("value")
	if !ok {
		return fmt.Errorf("metric %s has no 'value' field", point.Name())
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
		return fmt.Errorf("metric %s has invalid 'value' type for %s", point.Name(), s.name)
	}

	// Add unit
	if s.config.AddUnits {
		if tagunit, tagok := point.GetTag("unit"); tagok {
			c_unit = lookup(tagunit)
		} else if metaunit, metaok := point.GetMeta("unit"); metaok {
			c_unit = lookup(metaunit)
		} else {
			c_unit = lookup("")
		}
	} else {
		c_unit = lookup("")
	}

	// Create a new Ganglia metric
	gmetric := C.Ganglia_metric_create(s.global_context)
	rval := C.int(0)
	// Set name, value, type and unit in the Ganglia metric
	// Since we don't have this information from the collectors,
	// we assume that the metric value can go up and down (slope),
	// and their is no maximum for 'dmax' and 'tmax'
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

	// Set the cluster name, otherwise it takes it from the configuration file
	if len(s.config.ClusterName) > 0 {
		C.Ganglia_metadata_add(gmetric, lookup("CLUSTER"), lookup(s.config.ClusterName))
	}
	// Set the group metadata in the Ganglia metric if configured
	if group, ok := point.GetMeta("group"); ok && s.config.AddGangliaGroup {
		c_group := lookup(group)
		C.Ganglia_metadata_add(gmetric, lookup("GROUP"), c_group)
	}

	// Now we send the metric
	// gmetric does provide some more options like description and other options
	// but they are not provided by the collectors
	rval = C.Ganglia_metric_send(gmetric, s.send_channels)
	if rval != 0 {
		err = fmt.Errorf("there was an error sending metric %s to %d of the send channels ", point.Name(), rval)
		// fall throuph to use Ganglia_metric_destroy from common cleanup
	}
	// Cleanup Ganglia metric
	C.Ganglia_metric_destroy(gmetric)
	// Free the value C string, the only one not stored in the cache
	C.free(unsafe.Pointer(c_value))
	return err
}

func (s *LibgangliaSink) Flush() error {
	return nil
}

func (s *LibgangliaSink) Close() {
	// Destroy Ganglia configration struct
	// (not done by gmetric, I thought I am more clever but no...)
	//C.Ganglia_gmond_config_destroy(s.gmond_config)
	// Destroy Ganglia pool
	C.Ganglia_pool_destroy(s.global_context)

	// Cleanup C string cache
	for _, cstr := range s.cstrCache {
		C.free(unsafe.Pointer(cstr))
	}
}
