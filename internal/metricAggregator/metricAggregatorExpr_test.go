// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-lib.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// additional authors:
// Holger Obermaier (NHR@KIT)

package metricAggregator

import (
	"math"
	"testing"
	"time"

	lp "github.com/ClusterCockpit/cc-lib/v2/ccMessage"
)

func GenMetricNoCheck(value any, tags, meta map[string]string) lp.CCMessage {
	msg, _ := lp.NewMetric("test", tags, meta, value, time.Now())
	return msg
}

type TestBoolConfig struct {
	msg             lp.CCMessage
	cond            string
	expected_result bool
	should_fail     bool
}

var testBoolConfig []TestBoolConfig = []TestBoolConfig{
	{
		msg:             GenMetricNoCheck(1.0, nil, nil),
		cond:            "fields.value == 1",
		expected_result: true,
		should_fail:     false,
	},
	{
		msg:             GenMetricNoCheck(1.0, map[string]string{"hostname": "testhost"}, nil),
		cond:            "tags.hostname == 'testhost'",
		expected_result: true,
		should_fail:     false,
	},
}

func TestEvalBoolConditionExprSimple(t *testing.T) {

	for _, test := range testBoolConfig {
		res, err := EvalBoolConditionExpr(test.cond, test.msg)
		if err != nil && !test.should_fail {
			t.Error(err.Error())
			return
		}
		if !test.should_fail && res != test.expected_result {
			t.Errorf("Condition '%s' evaluated to %v despite expecting %v", test.cond, res, test.expected_result)
			return
		}
	}
}

type TestFloat64Config struct {
	msg             lp.CCMessage
	cond            string
	expected_result float64
	should_fail     bool
}

var testFloat64Config []TestFloat64Config = []TestFloat64Config{
	{
		msg:             GenMetricNoCheck(1.0, nil, nil),
		cond:            "fields.value + 3.14",
		expected_result: 4.14,
		should_fail:     false,
	},
	{
		msg:             GenMetricNoCheck(2.0, nil, nil),
		cond:            "fields.value * 2",
		expected_result: 4.0,
		should_fail:     false,
	},
}

const compareFloat64Max = 1e-9

func TestEvalFloat64ConditionExprSimple(t *testing.T) {

	for _, test := range testFloat64Config {
		res, err := EvalFloat64ConditionExpr(test.cond, test.msg)
		if err != nil && !test.should_fail {
			t.Error(err.Error())
			return
		}
		if !test.should_fail && math.Abs(res-test.expected_result) > compareFloat64Max {
			t.Errorf("Condition '%s' evaluated to %f despite expecting %f", test.cond, res, test.expected_result)
			return
		}
	}
}
