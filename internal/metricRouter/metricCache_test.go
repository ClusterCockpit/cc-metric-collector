// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package metricRouter

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
	mct "github.com/ClusterCockpit/cc-metric-collector/pkg/multiChanTicker"
)

func TestCache(t *testing.T) {
	output := make(chan lp.CCMessage, 2000)

	var wg sync.WaitGroup
	tickTime := time.Second
	testChan := make(chan time.Time)
	ticker := mct.NewTicker(tickTime)
	ticker.AddChannel(testChan)
	maxIntervals := 100

	c, err := NewCache(output, ticker, &wg, 4)
	if err != nil {
		t.Errorf("failed to create new cache: %s", err.Error())
		return
	}
	c.Start()
	defer c.Close()
	err = c.AddAggregation("ps_input_power", "Avg(values)", "name == 'ps1_input_power' || name == 'ps2_input_power' || name == 'ps3_input_power'", map[string]string{"hostname": "<copy>", "type": "<copy>"}, map[string]string{"unit": "<copy>"})
	if err != nil {
		t.Errorf("failed to add aggregation for %s", "ps_input_power")
		return
	}

	for range maxIntervals {
		timestamp := <-testChan
		raw_metrics := []string{
			fmt.Sprintf("ps1_input_power,type=node,hostname=myhost,unit=W value=696.0 %d", timestamp.UnixNano()),
			fmt.Sprintf("ps2_input_power,type=node,hostname=myhost,unit=W value=732.0 %d", timestamp.UnixNano()),
			fmt.Sprintf("ps3_input_power,type=node,hostname=myhost,unit=W value=720.0 %d", timestamp.UnixNano()),
			fmt.Sprintf("cpu_load,type=hwthread,type-id=0,hostname=myhost value=45 %d", timestamp.UnixNano()),
		}
		metrics, err := lp.FromBytes([]byte(strings.Join(raw_metrics, "\n")))
		if err != nil {
			t.Errorf("failed to generate metrics: %s", err.Error())
		}
		for _, m := range metrics {
			c.Add(m)
		}

		if len(output) > 0 {
			for i := 0; i < len(output); i++ {
				m := <-output
				t.Log(m.ToLineProtocol(nil))
			}
		}

	}

}
