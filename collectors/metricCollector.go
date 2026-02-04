// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package collectors

import (
	"encoding/json"
	"fmt"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

type MetricCollector interface {
	Name() string                      // Name of the metric collector
	Init(config json.RawMessage) error // Initialize metric collector
	Initialized() bool                 // Is metric collector initialized?
	Parallel() bool
	Read(duration time.Duration, output chan lp.CCMessage) // Read metrics from metric collector
	Close()                                                // Close / finish metric collector
}

type metricCollector struct {
	name     string            // name of the metric
	init     bool              // is metric collector initialized?
	parallel bool              // can the metric collector be executed in parallel with others
	meta     map[string]string // static meta data tags
}

// Name returns the name of the metric collector
func (c *metricCollector) Name() string {
	return c.name
}

// Name returns the name of the metric collector
func (c *metricCollector) Parallel() bool {
	return c.parallel
}

// Setup is for future use
func (c *metricCollector) setup() error {
	return nil
}

// Initialized indicates whether the metric collector has been initialized
func (c *metricCollector) Initialized() bool {
	return c.init
}

// stringArrayContains scans an array of strings if the value str is present in the array
// If the specified value is found, the corresponding array index is returned.
// The bool value is used to signal success or failure
func stringArrayContains(array []string, str string) (int, bool) {
	for i, a := range array {
		if a == str {
			return i, true
		}
	}
	return -1, false
}

// RemoveFromStringList removes the string r from the array of strings s
// If r is not contained in the array an error is returned
func RemoveFromStringList(s []string, r string) ([]string, error) {
	for i := range s {
		if r == s[i] {
			return append(s[:i], s[i+1:]...), nil
		}
	}
	return s, fmt.Errorf("no such string in list")
}
