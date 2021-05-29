package sinks

import (
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"sort"
)

var (
	socket_labels = []string{"socket"}
	cpu_labels    = []string{"cpu"}
)

type PrometheusCollector struct {
	sync.Mutex
	node    map[string]*prometheus.GaugeVec
	sockets map[string]*prometheus.GaugeVec
	cpus    map[string]*prometheus.GaugeVec
}

type PrometheusSink struct {
	Sink
	listening bool
	col       *PrometheusCollector
}

func NewPrometheusCollector() *PrometheusCollector {
	return &PrometheusCollector{
		node:    make(map[string]*prometheus.GaugeVec),
		sockets: make(map[string]*prometheus.GaugeVec),
		cpus:    make(map[string]*prometheus.GaugeVec),
	}
}

func (c *PrometheusCollector) check_metrics(measurement string, tags map[string]string, fields map[string]interface{}) error {
	labels := make([]string, 0)
	for k, _ := range tags {
		labels = append(labels, k)
	}
	sort.Strings(labels)
	switch measurement {
	case "node":
		for k, v := range fields {
			switch v.(type) {
			case float64:
			default:
				continue
			}
			if _, found := c.node[k]; !found {
				log.Print("Adding node metric ", k)
				c.node[k] = prometheus.NewGaugeVec(
					prometheus.GaugeOpts{
						Namespace: measurement,
						Name:      k,
						Help:      k,
					},
					labels,
				)
			}
		}
	case "socket":
		for k, v := range fields {
			switch v.(type) {
			case float64:
			default:
				continue
			}
			if _, found := c.sockets[k]; !found {
				log.Print("Adding socket metric ", k)
				c.sockets[k] = prometheus.NewGaugeVec(
					prometheus.GaugeOpts{
						Namespace: measurement,
						Name:      k,
						Help:      k,
					},
					labels,
				)
			}
		}
	case "cpu":
		for k, v := range fields {
			switch v.(type) {
			case float64:
			default:
				continue
			}
			if _, found := c.cpus[k]; !found {
				log.Print("Adding cpu metric ", k)
				c.cpus[k] = prometheus.NewGaugeVec(
					prometheus.GaugeOpts{
						Namespace: measurement,
						Name:      k,
						Help:      k,
					},
					labels,
				)
			}
		}
	}
	return nil
}

func (c *PrometheusCollector) update_metrics(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error {
	tagkeys := make([]string, 0)
	for k, _ := range tags {
		tagkeys = append(tagkeys, k)
	}
	sort.Strings(tagkeys)
	labels := make([]string, 0)
	for _, k := range tagkeys {
		labels = append(labels, tags[k])
	}
	switch measurement {
	case "node":
		for k, v := range fields {
			switch v.(type) {
			case float64:
			default:
				continue
			}
			log.Print("Setting node metric ", k, " (", strings.Join(labels, ","), "): ", v.(float64))
			c.node[k].WithLabelValues(labels...).Set(v.(float64))
		}
	case "socket":
		for k, v := range fields {
			switch v.(type) {
			case float64:
			default:
				continue
			}
			log.Print("Setting socket metric ", k, " (", strings.Join(labels, ","), "): ", v.(float64))
			c.sockets[k].WithLabelValues(labels...).Set(v.(float64))
		}
	case "cpu":
		for k, v := range fields {
			switch v.(type) {
			case float64:
			default:
				continue
			}
			log.Print("Setting cpu metric ", k, " (", strings.Join(labels, ","), "): ", v.(float64))
			c.cpus[k].WithLabelValues(labels...).Set(v.(float64))
		}
	}
	return nil
}

func (c *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	for k, _ := range c.node {
		c.node[k].Describe(ch)
	}
	for k, _ := range c.sockets {
		c.sockets[k].Describe(ch)
	}
	for k, _ := range c.cpus {
		c.cpus[k].Describe(ch)
	}
}

func (c *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	c.Lock()
	defer c.Unlock()

	for k, _ := range c.node {
		c.node[k].Collect(ch)
	}
	for k, _ := range c.sockets {
		c.sockets[k].Collect(ch)
	}
	for k, _ := range c.cpus {
		c.cpus[k].Collect(ch)
	}
}

func (s *PrometheusSink) Init(config SinkConfig) error {
	if len(config.Port) == 0 {
		return errors.New("Not all configuration variables set required by PrometheusSink")
	}
	s.host = config.Host
	s.port = config.Port
	s.database = config.Database
	s.organization = config.Organization
	s.user = config.User
	s.password = config.Password
	s.listening = false
	s.col = NewPrometheusCollector()
	log.Print("Init Prometheus HTTP")
	return nil
}

func (s *PrometheusSink) Write(measurement string, tags map[string]string, fields map[string]interface{}, t time.Time) error {
	err := s.col.check_metrics(measurement, tags, fields)
	if err != nil {
		log.Print(err)
	}
	err = s.col.update_metrics(measurement, tags, fields, t)
	if err != nil {
		log.Print(err)
	}
	if !s.listening {
		go func() {
			prometheus.MustRegister(s.col)
			addr := fmt.Sprintf("%s:%s", s.host, s.port)
			err = http.ListenAndServe(addr, promhttp.Handler())
			if err != nil {
				log.Fatal(err)
			}
		}()
		s.listening = true
	}
	return err
}

func (s *PrometheusSink) Close() {
	log.Print("Closing Prometheus HTTP")
}
