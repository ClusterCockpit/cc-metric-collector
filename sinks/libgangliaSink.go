package sinks

/*
#cgo CFLAGS: -DGM_PROTOCOL_GUARD
#cgo LDFLAGS: -L. -Wl,--unresolved-symbols=ignore-in-object-files
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

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	"github.com/NVIDIA/go-nvml/pkg/dl"
)

const (
	GANGLIA_LIB_NAME     = "libganglia.so"
	GANGLIA_LIB_DL_FLAGS = dl.RTLD_LAZY | dl.RTLD_GLOBAL
	GMOND_CONFIG_FILE    = `/etc/ganglia/gmond.conf`
)

// type LibgangliaSinkSpecialMetric struct {
// 	MetricName string `json:"metric_name,omitempty"`
// 	NewName    string `json:"new_name,omitempty"`
// 	Slope      string `json:"slope,omitempty"`
// }

type LibgangliaSinkConfig struct {
	defaultSinkConfig
	GangliaLib      string `json:"libganglia_path,omitempty"`
	GmondConfig     string `json:"gmond_config,omitempty"`
	AddGangliaGroup bool   `json:"add_ganglia_group,omitempty"`
	AddTypeToName   bool   `json:"add_type_to_name,omitempty"`
	AddUnits        bool   `json:"add_units,omitempty"`
	ClusterName     string `json:"cluster_name,omitempty"`
	//SpecialMetrics  map[string]LibgangliaSinkSpecialMetric `json:"rename_metrics,omitempty"` // Map to rename metric name from key to value
	//AddTagsAsDesc   bool              `json:"add_tags_as_desc,omitempty"`
}

type LibgangliaSink struct {
	sink
	config         LibgangliaSinkConfig
	global_context C.Ganglia_pool
	gmond_config   C.Ganglia_gmond_config
	send_channels  C.Ganglia_udp_send_channels
	cstrCache      map[string]*C.char
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

	conf := GetCommonGangliaConfig(point)
	if len(conf.Type) == 0 {
		conf = GetGangliaConfig(point)
	}
	if len(conf.Type) == 0 {
		return fmt.Errorf("metric %q (Ganglia name %q) has no 'value' field", point.Name(), conf.Name)
	}

	if s.config.AddTypeToName {
		conf.Name = GangliaMetricName(point)
	}

	c_value = C.CString(conf.Value)
	c_type = lookup(conf.Type)
	c_name = lookup(conf.Name)

	// Add unit
	unit := ""
	if s.config.AddUnits {
		unit = conf.Unit
	}
	c_unit = lookup(unit)

	// Determine the slope of the metric. Ganglia's own collector mostly use
	// 'both' but the mem and swap total uses 'zero'.
	slope_type := C.GANGLIA_SLOPE_BOTH
	switch conf.Slope {
	case "zero":
		slope_type = C.GANGLIA_SLOPE_ZERO
	case "both":
		slope_type = C.GANGLIA_SLOPE_BOTH
	}

	// Create a new Ganglia metric
	gmetric := C.Ganglia_metric_create(s.global_context)
	// Set name, value, type and unit in the Ganglia metric
	// The default slope_type is both directions, so up and down. Some metrics want 'zero' slope, probably constant.
	// The 'tmax' value is by default 300.
	rval := C.int(0)
	rval = C.Ganglia_metric_set(gmetric, c_name, c_value, c_type, c_unit, C.uint(slope_type), C.uint(conf.Tmax), 0)
	switch rval {
	case 1:
		C.free(unsafe.Pointer(c_value))
		return errors.New("invalid parameters")
	case 2:
		C.free(unsafe.Pointer(c_value))
		return errors.New("one of your parameters has an invalid character '\"'")
	case 3:
		C.free(unsafe.Pointer(c_value))
		return fmt.Errorf("the type parameter \"%s\" is not a valid type", conf.Type)
	case 4:
		C.free(unsafe.Pointer(c_value))
		return fmt.Errorf("the value parameter \"%s\" does not represent a number", conf.Value)
	default:
	}

	// Set the cluster name, otherwise it takes it from the configuration file
	if len(s.config.ClusterName) > 0 {
		C.Ganglia_metadata_add(gmetric, lookup("CLUSTER"), lookup(s.config.ClusterName))
	}
	// Set the group metadata in the Ganglia metric if configured
	if s.config.AddGangliaGroup {
		c_group := lookup(conf.Group)
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

func NewLibgangliaSink(name string, config json.RawMessage) (Sink, error) {
	s := new(LibgangliaSink)
	var err error = nil
	s.name = fmt.Sprintf("LibgangliaSink(%s)", name)
	//s.config.AddTagsAsDesc = false
	s.config.AddGangliaGroup = false
	s.config.AddTypeToName = false
	s.config.AddUnits = true
	s.config.GmondConfig = string(GMOND_CONFIG_FILE)
	s.config.GangliaLib = string(GANGLIA_LIB_NAME)
	if len(config) > 0 {
		err = json.Unmarshal(config, &s.config)
		if err != nil {
			cclog.ComponentError(s.name, "Error reading config:", err.Error())
			return nil, err
		}
	}
	lib := dl.New(s.config.GangliaLib, GANGLIA_LIB_DL_FLAGS)
	if lib == nil {
		return nil, fmt.Errorf("error instantiating DynamicLibrary for %s", s.config.GangliaLib)
	}
	err = lib.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %v", s.config.GangliaLib, err)
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
	return s, nil
}
