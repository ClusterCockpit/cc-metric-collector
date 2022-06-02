package sinks

import (
	"errors"
	"fmt"
	"sort"
	"time"

	ilp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	lp "github.com/influxdata/line-protocol"
)

// A node in a tree structured by tags
type compactorEntry struct {
	tagKey   string                     // The tag key all points share in this sub-tree.
	tagValue string                     // The tag value all points share in this sub-tree.
	points   map[int64][]ilp.CCMetric   // The points with those keys (only), grouped by points where the time is the same.
	entries  map[string]*compactorEntry // Points with even more keys go here, grouped by those keys.
}

func (ce *compactorEntry) read(lines []ilp.CCMetric, tags []lp.Tag) ([]ilp.CCMetric, error) {
	for t, points := range ce.points {
		fields := make(map[string]interface{})
		for _, point := range points {
			if val, ok := point.GetField("value"); ok {
				fields[point.Name()] = val
			} else {
				return nil, errors.New("only field expected is 'value'")
			}
		}

		p, err := ilp.New("data", nil, nil, fields, time.Unix(t, 0))
		if err != nil {
			return nil, err
		}
		for _, tag := range tags {
			p.AddTag(tag.Key, tag.Value)
		}

		lines = append(lines, p)
	}

	for _, e := range ce.entries {
		var err error
		lines, err = e.read(lines, append(tags, lp.Tag{
			Key:   e.tagKey,
			Value: e.tagValue,
		}))
		if err != nil {
			return nil, err
		}
	}

	return lines, nil
}

type Compactor struct {
	wrapped Sink
	n       int
	root    compactorEntry
}

var _ Sink = (*Compactor)(nil)

func NewCompactor(name string, wrapped Sink) (Sink, error) {
	c := &Compactor{
		wrapped: wrapped,
		root: compactorEntry{
			points:  make(map[int64][]ilp.CCMetric),
			entries: make(map[string]*compactorEntry),
		},
	}
	return c, nil
}

func (c *Compactor) Write(point ilp.CCMetric) error {
	taglist := make([]lp.Tag, 0)
	for k, v := range point.Tags() {
		taglist = append(taglist, lp.Tag{Key: k, Value: v})
	}

	sort.Slice(taglist, func(i, j int) bool {
		a, b := taglist[i], taglist[j]
		return a.Key < b.Key
	})

	e := &c.root
	for _, tag := range taglist {
		mapkey := tag.Key + ":" + tag.Value
		ce, ok := e.entries[mapkey]
		if !ok {
			ce = &compactorEntry{
				tagKey:   tag.Key,
				tagValue: tag.Value,
				points:   make(map[int64][]ilp.CCMetric),
				entries:  make(map[string]*compactorEntry),
			}
			e.entries[mapkey] = ce
		}
		e = ce
	}

	t := point.Time().Unix()
	c.n += 1
	e.points[t] = append(e.points[t], point)
	return nil
}

func (c *Compactor) Flush() error {
	points := make([]ilp.CCMetric, 0, c.n)
	points, err := c.root.read(points, make([]lp.Tag, 0, 5))
	if err != nil {
		return err
	}

	for _, p := range points {
		if err := c.wrapped.Write(p); err != nil {
			return err
		}
	}

	c.n = 0
	c.root.points = make(map[int64][]ilp.CCMetric)
	c.root.entries = make(map[string]*compactorEntry)
	return c.wrapped.Flush()
}

func (c *Compactor) Close() {
	c.wrapped.Close()
}

func (c *Compactor) Name() string {
	return fmt.Sprintf("%s (compacted)", c.wrapped.Name())
}
