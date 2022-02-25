package receivers

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
)

type PrometheusReceiverConfig struct {
	defaultReceiverConfig
	Addr     string `json:"address"`
	Port     string `json:"port"`
	Path     string `json:"path"`
	Interval string `json:"interval"`
	SSL      bool   `json:"ssl"`
}

type PrometheusReceiver struct {
	receiver
	meta     map[string]string
	config   PrometheusReceiverConfig
	interval time.Duration
	done     chan bool
	wg       sync.WaitGroup
	ticker   *time.Ticker
	uri      string
}

func (r *PrometheusReceiver) Start() {
	cclog.ComponentDebug(r.name, "START", r.uri)
	r.wg.Add(1)

	r.ticker = time.NewTicker(r.interval)
	go func() {
		for {
			select {
			case <-r.done:
				r.wg.Done()
				return
			case t := <-r.ticker.C:
				resp, err := http.Get(r.uri)
				if err != nil {
					log.Fatal(err)
				}
				defer resp.Body.Close()

				scanner := bufio.NewScanner(resp.Body)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "#") {
						continue
					}
					lineSplit := strings.Fields(line)
					// separate metric name from tags (labels in Prometheus)
					tags := map[string]string{}
					name := lineSplit[0]
					if sindex := strings.Index(name, "{"); sindex >= 0 {
						eindex := strings.Index(name, "}")
						for _, kv := range strings.Split(name[sindex+1:eindex], ",") {
							eq := strings.Index(kv, "=")
							tags[kv[0:eq]] = strings.Trim(kv[eq+1:], "\"")
						}
						name = lineSplit[0][0:sindex]
					}
					value, err := strconv.ParseFloat(lineSplit[1], 64)
					if err == nil {
						y, err := lp.New(name, tags, r.meta, map[string]interface{}{"value": value}, t)
						if err == nil {
							r.sink <- y
						}
					}
				}
			}
		}
	}()
}

func (r *PrometheusReceiver) Close() {
	cclog.ComponentDebug(r.name, "CLOSE")
	r.done <- true
	r.wg.Wait()
}

func NewPrometheusReceiver(name string, config json.RawMessage) (Receiver, error) {
	r := new(PrometheusReceiver)
	r.name = fmt.Sprintf("PrometheusReceiver(%s)", name)
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return nil, err
		}
	}
	if len(r.config.Addr) == 0 ||
		len(r.config.Port) == 0 ||
		len(r.config.Interval) == 0 {
		return nil, errors.New("not all configuration variables set required by PrometheusReceiver (address and port)")
	}
	if len(r.config.Interval) > 0 {
		t, err := time.ParseDuration(r.config.Interval)
		if err == nil {
			r.interval = t
		}
	}
	r.meta = map[string]string{"source": r.name}
	proto := "http"
	if r.config.SSL {
		proto = "https"
	}
	r.uri = fmt.Sprintf("%s://%s:%s/%s", proto, r.config.Addr, r.config.Port, r.config.Path)
	return r, nil
}
