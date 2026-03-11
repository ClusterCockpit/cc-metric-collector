// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package collectors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"syscall"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
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
	if err := m.setup(); err != nil {
		return fmt.Errorf("%s Init(): setup() call failed: %w", m.name, err)
	}
	m.parallel = true
	m.meta = map[string]string{
		"source": m.name,
		"group":  "Self",
	}
	m.tags = map[string]string{
		"type": "node",
	}
	if len(config) > 0 {
		d := json.NewDecoder(bytes.NewReader(config))
		d.DisallowUnknownFields()
		if err := d.Decode(&m.config); err != nil {
			return fmt.Errorf("%s Init(): Error decoding JSON config: %w", m.name, err)
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

		y, err := lp.NewMetric("total_alloc", m.tags, m.meta, memstats.TotalAlloc, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMetric("heap_alloc", m.tags, m.meta, memstats.HeapAlloc, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMetric("heap_sys", m.tags, m.meta, memstats.HeapSys, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMetric("heap_idle", m.tags, m.meta, memstats.HeapIdle, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMetric("heap_inuse", m.tags, m.meta, memstats.HeapInuse, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMetric("heap_released", m.tags, m.meta, memstats.HeapReleased, timestamp)
		if err == nil {
			y.AddMeta("unit", "Bytes")
			output <- y
		}
		y, err = lp.NewMetric("heap_objects", m.tags, m.meta, memstats.HeapObjects, timestamp)
		if err == nil {
			output <- y
		}
	}
	if m.config.GoRoutines {
		y, err := lp.NewMetric("num_goroutines", m.tags, m.meta, runtime.NumGoroutine(), timestamp)
		if err == nil {
			output <- y
		}
	}
	if m.config.CgoCalls {
		y, err := lp.NewMetric("num_cgo_calls", m.tags, m.meta, runtime.NumCgoCall(), timestamp)
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
			y, err := lp.NewMetric("rusage_user_time", m.tags, m.meta, t, timestamp)
			if err == nil {
				y.AddMeta("unit", "seconds")
				output <- y
			}
			sec, nsec = rusage.Stime.Unix()
			t = float64(sec) + (float64(nsec) * 1e-9)
			y, err = lp.NewMetric("rusage_system_time", m.tags, m.meta, t, timestamp)
			if err == nil {
				y.AddMeta("unit", "seconds")
				output <- y
			}
			y, err = lp.NewMetric("rusage_vol_ctx_switch", m.tags, m.meta, rusage.Nvcsw, timestamp)
			if err == nil {
				output <- y
			}
			y, err = lp.NewMetric("rusage_invol_ctx_switch", m.tags, m.meta, rusage.Nivcsw, timestamp)
			if err == nil {
				output <- y
			}
			y, err = lp.NewMetric("rusage_signals", m.tags, m.meta, rusage.Nsignals, timestamp)
			if err == nil {
				output <- y
			}
			y, err = lp.NewMetric("rusage_major_pgfaults", m.tags, m.meta, rusage.Majflt, timestamp)
			if err == nil {
				output <- y
			}
			y, err = lp.NewMetric("rusage_minor_pgfaults", m.tags, m.meta, rusage.Minflt, timestamp)
			if err == nil {
				output <- y
			}
		}

	}
}

func (m *SelfCollector) Close() {
	m.init = false
}
