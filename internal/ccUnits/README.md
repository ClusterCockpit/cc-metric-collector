# ccUnits - A unit system for ClusterCockpit

When working with metrics, the problem comes up that they may use different unit name but have the same unit in fact. There are a lot of real world examples like 'kB' and 'Kbyte'. In CC Metric Collector, the Collectors read data from different sources which may use different units or the programmer specifies a unit for a metric by hand. In order to enable unit comparison and conversion, the ccUnits package provides some helpers:

There are basically two important functions:
```go
NewUnit(unit string) Unit
GetUnitPrefixFactor(in, out Unit) (float64, error) // Get the prefix difference for conversion
```

In order to get the "normalized" string unit back, you can use:
```go
u := NewUnit("MB")
fmt.Printf("Long string %q", u.String())
fmt.Printf("Short string %q", u.Short())
```

If you have two units and need the conversion factor:
```go
u1 := NewUnit("kB")
u2 := NewUnit("MBytes")
factor, err := GetUnitPrefixFactor(u1, u2) // Returns an error if the units have different measures
if err == nil {
    v2 := v1 * factor
}
```

If you have a metric and want the derivation to a bandwidth or events per second, you can use the original unit:

```go
in_unit, err := metric.GetMeta("unit")
if err == nil {
    value, ok := metric.GetField("value")
    if ok {
        out_unit = NewUnit(in_unit)
        out_unit.AddDivisorUnit("seconds")
        y, err := lp.New(metric.Name()+"_bw",
                         metric.Tags(),
                         metric.Meta(),
                         map[string]interface{"value": value/time},
                         metric.Time())
        if err == nil {
            y.AddMeta("unit", out_unit.Short())
        }
    }
}
```

## Supported prefixes

```go
const (
	Base  Prefix = iota
	Peta         = 1e15
	Tera         = 1e12
	Giga         = 1e9
	Mega         = 1e6
	Kilo         = 1e3
	Milli        = 1e-3
	Micro        = 1e-6
	Nano         = 1e-9
	Kibi         = 1024
	Mebi         = 1024 * 1024
	Gibi         = 1024 * 1024 * 1024
	Tebi         = 1024 * 1024 * 1024 * 1024
)
```

The prefixes are detected using a regular expression `^([kKmMgGtTpP]?[i]?)(.*)` that splits the prefix from the measure. You probably don't need to deal with the prefixes in the code.

## Supported measures

```go
const (
	None Measure = iota
	Bytes
	Flops
	Percentage
	TemperatureC
	TemperatureF
	Rotation
	Hertz
	Time
	Power
	Energy
	Cycles
	Requests
	Packets
	Events
)
```

There a regular expression for each of the measures like `^([bB][yY]?[tT]?[eE]?[sS]?)` for the `Bytes` measure. 