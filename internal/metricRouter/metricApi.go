package metricRouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	mct "github.com/ClusterCockpit/cc-metric-collector/internal/multiChanTicker"
	"github.com/gorilla/mux"
)

type statsApiConfig struct {
	PublishCollectorState bool   `json:"publish_collectorstate"`
	Host                  string `json:"bindhost"`
	Port                  string `json:"port"`
}

// Metric cache data structure
type statsApi struct {
	name     string
	input    chan lp.CCMetric
	indone   chan bool
	outdone  chan bool
	config   statsApiConfig
	wg       *sync.WaitGroup
	statsWg  sync.WaitGroup
	ticker   mct.MultiChanTicker
	tickchan chan time.Time
	server   *http.Server
	router   *mux.Router
	lock     sync.Mutex
	baseurl  string
	stats    map[string]map[string]int64
	outStats map[string]map[string]int64
}

type StatsApi interface {
	Start()
	Close()
	StatsFunc(w http.ResponseWriter, r *http.Request)
}

var statsApiServer *statsApi = nil

func (a *statsApi) updateStats(point lp.CCMetric) {
	switch point.Name() {
	case "_stats":
		if name, nok := point.GetMeta("source"); nok {
			var compStats map[string]int64
			var ok bool

			if compStats, ok = a.stats[name]; !ok {
				a.stats[name] = make(map[string]int64)
				compStats = a.stats[name]
			}
			for k, v := range point.Fields() {
				switch value := v.(type) {
				case int:
					compStats[k] = int64(value)
				case uint:
					compStats[k] = int64(value)
				case int32:
					compStats[k] = int64(value)
				case uint32:
					compStats[k] = int64(value)
				case int64:
					compStats[k] = int64(value)
				case uint64:
					compStats[k] = int64(value)
				default:
					cclog.ComponentDebug(a.name, "Unusable stats for", k, ". Values should be int64")
				}
			}
			a.stats[name] = compStats
		}
	}
}

func (a *statsApi) Start() {
	a.ticker.AddChannel(a.tickchan)
	a.wg.Add(1)
	a.statsWg.Add(1)
	go func() {
		a.stats = make(map[string]map[string]int64)
		defer a.statsWg.Done()
		for {
			select {
			case <-a.indone:
				cclog.ComponentDebug(a.name, "INPUT DONE")
				close(a.indone)
				return
			case p := <-a.input:
				a.lock.Lock()
				a.updateStats(p)
				a.lock.Unlock()
			}
		}
	}()
	a.statsWg.Add(1)
	go func() {
		a.outStats = make(map[string]map[string]int64)
		defer a.statsWg.Done()
		a.lock.Lock()
		for comp, compData := range a.stats {
			var outData map[string]int64
			var ok bool
			if outData, ok = a.outStats[comp]; !ok {
				outData = make(map[string]int64)
			}
			for k, v := range compData {
				outData[k] = v
			}
			a.outStats[comp] = outData
		}
		a.lock.Unlock()
		for {
			select {
			case <-a.outdone:
				cclog.ComponentDebug(a.name, "OUTPUT DONE")
				close(a.outdone)
				return
			case <-a.tickchan:
				a.lock.Lock()
				for comp, compData := range a.stats {
					var outData map[string]int64
					var ok bool
					if outData, ok = a.outStats[comp]; !ok {
						outData = make(map[string]int64)
					}
					for k, v := range compData {
						outData[k] = v
					}
					a.outStats[comp] = outData
				}
				a.lock.Unlock()
			}
		}
	}()
	a.statsWg.Add(1)
	go func() {
		defer a.statsWg.Done()
		err := a.server.ListenAndServe()
		if err != nil && err.Error() != "http: Server closed" {
			cclog.ComponentError(a.name, err.Error())
		}
		cclog.ComponentDebug(a.name, "SERVER DONE")
	}()
	cclog.ComponentDebug(a.name, "STARTED")
}

func (a *statsApi) StatsFunc(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(a.outStats)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, string(data))
	}
}

// Close finishes / stops the metric cache
func (a *statsApi) Close() {
	cclog.ComponentDebug(a.name, "CLOSE")
	a.indone <- true
	a.outdone <- true
	a.server.Shutdown(context.Background())
	// wait for close of channel r.done
	<-a.indone
	<-a.outdone
	a.statsWg.Wait()
	a.wg.Done()

	//a.wg.Wait()
}

func NewStatsApi(ticker mct.MultiChanTicker, wg *sync.WaitGroup, statsApiConfigfile string) (StatsApi, error) {
	a := new(statsApi)
	a.name = "StatsApi"
	a.config.Host = "localhost"
	a.config.Port = "8080"
	configFile, err := os.Open(statsApiConfigfile)
	if err != nil {
		cclog.ComponentError(a.name, err.Error())
		return nil, err
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&a.config)
	if err != nil {
		cclog.ComponentError(a.name, err.Error())
		return nil, err
	}
	a.input = make(chan lp.CCMetric)
	a.ticker = ticker
	a.tickchan = make(chan time.Time)
	a.wg = wg
	a.indone = make(chan bool)
	a.outdone = make(chan bool)
	a.router = mux.NewRouter()
	a.baseurl = fmt.Sprintf("%s:%s", a.config.Host, a.config.Port)
	a.server = &http.Server{Addr: a.baseurl, Handler: a.router}
	if a.config.PublishCollectorState {
		a.router.HandleFunc("/", a.StatsFunc)
	}
	statsApiServer = a
	return a, nil
}

func ComponentStatInt(component string, key string, value int64) {
	if statsApiServer == nil {
		return
	}
	y, err := lp.New("_stats", map[string]string{}, map[string]string{"source": component}, map[string]interface{}{key: value}, time.Now())
	if err == nil {
		statsApiServer.input <- y
	}
}

func ComponentStatString(component string, key string, value int64) {
	if statsApiServer == nil {
		return
	}
	y, err := lp.New("_stats", map[string]string{}, map[string]string{"source": component}, map[string]interface{}{key: value}, time.Now())
	if err == nil {
		statsApiServer.input <- y
	}
}
