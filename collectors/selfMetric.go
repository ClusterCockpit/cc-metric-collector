package collectors

import (
	"encoding/json"
	"runtime"
	"syscall"
	"time"

	cclog "github.com/ClusterCockpit/cc-lib/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
)

type SelfCollectorConfig struct {
	MemStats   bool `json:"read_mem_stats"`
	GoRoutines bool `json:"read_goroutines"`
	CgoCalls   bool `json:"read_cgo_calls"`
	Rusage     bool `json:"read_rusage"`
}

type SelfCollector struct {
	metricCollector
	config SelfCollectorConfig // the configuration structure
	meta   map[string]string   // default meta information
	tags   map[string]string   // default tags
}

func (m *SelfCollector) Init(config json.RawMessage) error {
	var err error = nil
	m.name = "SelfCollector"
	m.setup()
	m.parallel = true
	m.meta = map[string]string{"source": m.name, "group": "Self"}
	m.tags = map[string]string{"type": "node"}
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			cclog.ComponentError(m.name, "Error reading config:", err.Error())
			return err
		}
	}
	m.init = true
	return err
}

func (m *SelfCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	timestamp := time.Now()

	if m.config.MemStats {
		var memstats runtime.MemStats
		runtime.ReadMemStats(&memstats)

		y, err := lp.NewMessage("total_alloc", m.tags, m.meta, map[string]interface{}{"value": memstats.TotalAlloc}, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMessage("heap_alloc", m.tags, m.meta, map[string]interface{}{"value": memstats.HeapAlloc}, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMessage("heap_sys", m.tags, m.meta, map[string]interface{}{"value": memstats.HeapSys}, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMessage("heap_idle", m.tags, m.meta, map[string]interface{}{"value": memstats.HeapIdle}, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMessage("heap_inuse", m.tags, m.meta, map[string]interface{}{"value": memstats.HeapInuse}, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMessage("heap_released", m.tags, m.meta, map[string]interface{}{"value": memstats.HeapReleased}, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMessage("heap_objects", m.tags, m.meta, map[string]interface{}{"value": memstats.HeapObjects}, timestamp)
		if err == nil {
			output <- y
		}
	}
	if m.config.GoRoutines {
		y, err := lp.NewMessage("num_goroutines", m.tags, m.meta, map[string]interface{}{"value": runtime.NumGoroutine()}, timestamp)
		if err == nil {
			output <- y
		}
	}
	if m.config.CgoCalls {
		y, err := lp.NewMessage("num_cgo_calls", m.tags, m.meta, map[string]interface{}{"value": runtime.NumCgoCall()}, timestamp)
		if err == nil {
			output <- y
		}
	}
	if m.config.Rusage {
		var rusage syscall.Rusage
		err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage)
		if err == nil {
			sec, nsec := rusage.Utime.Unix()
			t := float64(sec) + (float64(nsec) * 1e-9)
			y, err := lp.NewMessage("rusage_user_time", m.tags, m.meta, map[string]interface{}{"value": t}, timestamp)
			if err == nil {
				y.AddMeta("unit", "seconds")
				output <- y
			}
			sec, nsec = rusage.Stime.Unix()
			t = float64(sec) + (float64(nsec) * 1e-9)
			y, err = lp.NewMessage("rusage_system_time", m.tags, m.meta, map[string]interface{}{"value": t}, timestamp)
			if err == nil {
				y.AddMeta("unit", "seconds")
				output <- y
			}
			y, err = lp.NewMessage("rusage_vol_ctx_switch", m.tags, m.meta, map[string]interface{}{"value": rusage.Nvcsw}, timestamp)
			if err == nil {
				output <- y
			}
			y, err = lp.NewMessage("rusage_invol_ctx_switch", m.tags, m.meta, map[string]interface{}{"value": rusage.Nivcsw}, timestamp)
			if err == nil {
				output <- y
			}
			y, err = lp.NewMessage("rusage_signals", m.tags, m.meta, map[string]interface{}{"value": rusage.Nsignals}, timestamp)
			if err == nil {
				output <- y
			}
			y, err = lp.NewMessage("rusage_major_pgfaults", m.tags, m.meta, map[string]interface{}{"value": rusage.Majflt}, timestamp)
			if err == nil {
				output <- y
			}
			y, err = lp.NewMessage("rusage_minor_pgfaults", m.tags, m.meta, map[string]interface{}{"value": rusage.Minflt}, timestamp)
			if err == nil {
				output <- y
			}
		}

	}
}

func (m *SelfCollector) Close() {
	m.init = false
}
