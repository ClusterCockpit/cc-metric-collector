package ccunits

import (
	"fmt"
	"strings"
)

type Unit struct {
	scale      Prefix
	measure    Measure
	divMeasure Measure
}

func (u *Unit) String() string {
	if u.divMeasure != None {
		return fmt.Sprintf("%s%s/%s", u.scale.String(), u.measure.String(), u.divMeasure.String())
	} else {
		return fmt.Sprintf("%s%s", u.scale.String(), u.measure.String())
	}
}

func (u *Unit) Short() string {
	if u.divMeasure != None {
		return fmt.Sprintf("%s%s/%s", u.scale.Prefix(), u.measure.Short(), u.divMeasure.Short())
	} else {
		return fmt.Sprintf("%s%s", u.scale.Prefix(), u.measure.Short())
	}
}

func (u *Unit) AddDivisorUnit(div Measure) {
	u.divMeasure = div
}

func GetPrefixFactor(in Prefix, out Prefix) float64 {
	var factor = 1.0
	var in_scale = 1.0
	var out_scale = 1.0
	if in != Base {
		in_scale = float64(in)
	}
	if out != Base {
		out_scale = float64(out)
	}
	factor = in_scale / out_scale
	return factor
}

func GetUnitPrefixFactor(in Unit, out Unit) (float64, error) {
	if in.measure != out.measure || in.divMeasure != out.divMeasure {
		return 1.0, fmt.Errorf("invalid measures in in and out Unit")
	}
	return GetPrefixFactor(in.scale, out.scale), nil
}

func NewUnit(unit string) Unit {
	u := Unit{
		scale:      Base,
		measure:    None,
		divMeasure: None,
	}
	matches := prefixRegex.FindStringSubmatch(unit)
	if len(matches) > 2 {
		u.scale = NewPrefix(matches[1])
		measures := strings.Split(matches[2], "/")
		u.measure = NewMeasure(measures[0])
		// Special case for 'm' as scale for Bytes as thers is nothing like MilliBytes
		if u.measure == Bytes && u.scale == Milli {
			u.scale = Mega
		}
		if len(measures) > 1 {
			u.divMeasure = NewMeasure(measures[1])
		}
	}
	return u
}
