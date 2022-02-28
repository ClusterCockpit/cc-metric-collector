package receivers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	"github.com/gorilla/mux"
	influx "github.com/influxdata/line-protocol"
)

const HTTP_RECEIVER_PORT = "8080"

type HttpReceiverConfig struct {
	Type string `json:"type"`
	Addr string `json:"address"`
	Port string `json:"port"`
	Path string `json:"path"`
}

type HttpReceiver struct {
	receiver
	handler *influx.MetricHandler
	parser  *influx.Parser
	meta    map[string]string
	config  HttpReceiverConfig
	router  *mux.Router
	server  *http.Server
	wg      sync.WaitGroup
}

func (r *HttpReceiver) Init(name string, config json.RawMessage) error {
	r.name = fmt.Sprintf("HttpReceiver(%s)", name)
	r.config.Port = HTTP_RECEIVER_PORT
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return err
		}
	}
	if len(r.config.Port) == 0 {
		return errors.New("not all configuration variables set required by HttpReceiver")
	}
	r.meta = map[string]string{"source": r.name}
	p := r.config.Path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	uri := fmt.Sprintf("%s:%s%s", r.config.Addr, r.config.Port, p)
	cclog.ComponentDebug(r.name, "INIT", uri)
	r.handler = influx.NewMetricHandler()
	r.parser = influx.NewParser(r.handler)
	r.parser.SetTimeFunc(DefaultTime)

	r.router = mux.NewRouter()
	r.router.Path(p).HandlerFunc(r.ServerHttp)
	r.server = &http.Server{Addr: uri, Handler: r.router}
	return nil
}

func (r *HttpReceiver) Start() {
	cclog.ComponentDebug(r.name, "START")
	r.wg.Add(1)
	go func() {
		err := r.server.ListenAndServe()
		if err != nil && err.Error() != "http: Server closed" {
			cclog.ComponentError(r.name, err.Error())
		}
		r.wg.Done()
	}()
}

func (r *HttpReceiver) ServerHttp(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	metrics, err := r.parser.Parse(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	for _, m := range metrics {
		y := lp.FromInfluxMetric(m)
		for k, v := range r.meta {
			y.AddMeta(k, v)
		}
		if r.sink != nil {
			r.sink <- y
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (r *HttpReceiver) Close() {
	r.server.Shutdown(context.Background())
}

func NewHttpReceiver(name string, config json.RawMessage) (Receiver, error) {
	r := new(HttpReceiver)
	err := r.Init(name, config)
	return r, err
}
