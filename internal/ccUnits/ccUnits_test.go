package ccunits

import (
	"fmt"
	"testing"
)

func TestUnitsExact(t *testing.T) {
	testCases := []struct {
		in   string
		want Unit
	}{
		{"b", NewUnit("Bytes")},
		{"B", NewUnit("Bytes")},
		{"byte", NewUnit("Bytes")},
		{"bytes", NewUnit("Bytes")},
		{"BYtes", NewUnit("Bytes")},
		{"Mb", NewUnit("MBytes")},
		{"MB", NewUnit("MBytes")},
		{"Mbyte", NewUnit("MBytes")},
		{"Mbytes", NewUnit("MBytes")},
		{"MbYtes", NewUnit("MBytes")},
		{"Gb", NewUnit("GBytes")},
		{"GB", NewUnit("GBytes")},
		{"Hz", NewUnit("Hertz")},
		{"MHz", NewUnit("MHertz")},
		{"GHertz", NewUnit("GHertz")},
		{"pkts", NewUnit("Packets")},
		{"packets", NewUnit("Packets")},
		{"packet", NewUnit("Packets")},
		{"flop", NewUnit("Flops")},
		{"flops", NewUnit("Flops")},
		{"floPS", NewUnit("Flops")},
		{"Mflop", NewUnit("MFlops")},
		{"Gflop", NewUnit("GFlops")},
		{"gflop", NewUnit("GFlops")},
		{"%", NewUnit("Percent")},
		{"percent", NewUnit("Percent")},
		{"degc", NewUnit("degC")},
		{"degC", NewUnit("degC")},
		{"degf", NewUnit("degF")},
		{"°f", NewUnit("degF")},
		{"events", NewUnit("events")},
		{"event", NewUnit("events")},
		{"EveNts", NewUnit("events")},
		{"reqs", NewUnit("requests")},
		{"requests", NewUnit("requests")},
		{"Requests", NewUnit("requests")},
		{"cyc", NewUnit("cycles")},
		{"cy", NewUnit("cycles")},
		{"Cycles", NewUnit("cycles")},
		{"J", NewUnit("Joules")},
		{"Joule", NewUnit("Joules")},
		{"joule", NewUnit("Joules")},
		{"W", NewUnit("Watt")},
		{"Watts", NewUnit("Watt")},
		{"watt", NewUnit("Watt")},
		{"s", NewUnit("seconds")},
		{"sec", NewUnit("seconds")},
		{"secs", NewUnit("seconds")},
		{"RPM", NewUnit("rpm")},
		{"rPm", NewUnit("rpm")},
		{"watt/byte", NewUnit("W/B")},
		{"watts/bytes", NewUnit("W/B")},
		{"flop/byte", NewUnit("flops/Bytes")},
		{"F/B", NewUnit("flops/Bytes")},
	}
	compareUnitExact := func(in, out Unit) bool {
		if in.getMeasure() == out.getMeasure() && in.getDivMeasure() == out.getDivMeasure() && in.getPrefix() == out.getPrefix() {
			return true
		}
		return false
	}
	for _, c := range testCases {
		u := NewUnit(c.in)
		if (!u.Valid()) || (!compareUnitExact(u, c.want)) {
			t.Errorf("func NewUnit(%q) == %q, want %q", c.in, u.String(), c.want.String())
		}
	}
}

func TestUnitsDifferentPrefix(t *testing.T) {
	testCases := []struct {
		in           string
		want         Unit
		prefixFactor float64
	}{
		{"kb", NewUnit("Bytes"), 1000},
		{"Mb", NewUnit("Bytes"), 1000000},
		{"Mb/s", NewUnit("Bytes/s"), 1000000},
		{"Flops/s", NewUnit("MFlops/s"), 1e-6},
		{"Flops/s", NewUnit("GFlops/s"), 1e-9},
		{"MHz", NewUnit("Hertz"), 1e6},
		{"kb", NewUnit("Kib"), 1000.0 / 1024},
		{"Mib", NewUnit("MBytes"), (1024 * 1024.0) / (1e6)},
		{"mb", NewUnit("MBytes"), 1.0},
	}
	compareUnitWithPrefix := func(in, out Unit, factor float64) bool {
		if in.getMeasure() == out.getMeasure() && in.getDivMeasure() == out.getDivMeasure() {
			if f := GetPrefixFactor(in.getPrefix(), out.getPrefix()); f(1.0) == factor {
				return true
			} else {
				fmt.Println(f(1.0))
			}
		}
		return false
	}
	for _, c := range testCases {
		u := NewUnit(c.in)
		if (!u.Valid()) || (!compareUnitWithPrefix(u, c.want, c.prefixFactor)) {
			t.Errorf("func NewUnit(%q) == %q, want %q with factor %f", c.in, u.String(), c.want.String(), c.prefixFactor)
		}
	}
}
