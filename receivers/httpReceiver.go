package receivers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	"github.com/gorilla/mux"
	influx "github.com/influxdata/line-protocol/v2/lineprotocol"
)

const HTTP_RECEIVER_PORT = "8080"

type HttpReceiverConfig struct {
	Type string `json:"type"`
	Addr string `json:"address"`
	Port string `json:"port"`
	Path string `json:"path"`

	// Maximum amount of time to wait for the next request when keep-alives are enabled
	// should be larger than the measurement interval to keep the connection open
	IdleTimeout string `json:"idle_timeout"`
	idleTimeout time.Duration
}

type HttpReceiver struct {
	receiver
	meta   map[string]string
	config HttpReceiverConfig
	router *mux.Router
	server *http.Server
	wg     sync.WaitGroup
}

func (r *HttpReceiver) Init(name string, config json.RawMessage) error {
	r.name = fmt.Sprintf("HttpReceiver(%s)", name)

	// Set default values
	r.config.Port = HTTP_RECEIVER_PORT
	// should be larger than the measurement interval to keep the connection open
	r.config.IdleTimeout = "120s"

	// Read config
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
	if len(r.config.IdleTimeout) > 0 {
		t, err := time.ParseDuration(r.config.IdleTimeout)
		if err == nil {
			cclog.ComponentDebug(r.name, "idleTimeout", t)
			r.config.idleTimeout = t
		}
	}
	r.meta = map[string]string{"source": r.name}
	p := r.config.Path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	addr := fmt.Sprintf("%s:%s", r.config.Addr, r.config.Port)
	uri := addr + p
	cclog.ComponentDebug(r.name, "INIT", "listen on:", uri)

	// Create new router and register p as path
	r.router = mux.NewRouter()
	r.router.Path(p).HandlerFunc(r.ServerHttp)

	// Create http server, with router as handler
	r.server = &http.Server{
		Addr:        addr,
		Handler:     r.router,
		IdleTimeout: r.config.idleTimeout,
	}
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

	d := influx.NewDecoder(req.Body)
	for d.Next() {

		// Decode measurement name
		measurement, err := d.Measurement()
		if err != nil {
			msg := "ServerHttp: Failed to decode measurement: " + err.Error()
			cclog.ComponentError(r.name, msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		// Decode tags
		tags := make(map[string]string)
		for {
			key, value, err := d.NextTag()
			if err != nil {
				msg := "ServerHttp: Failed to decode tag: " + err.Error()
				cclog.ComponentError(r.name, msg)
				http.Error(w, msg, http.StatusInternalServerError)
				return
			}
			if key == nil {
				break
			}
			tags[string(key)] = string(value)
		}

		// Decode fields
		fields := make(map[string]interface{})
		for {
			key, value, err := d.NextField()
			if err != nil {
				msg := "ServerHttp: Failed to decode field: " + err.Error()
				cclog.ComponentError(r.name, msg)
				http.Error(w, msg, http.StatusInternalServerError)
				return
			}
			if key == nil {
				break
			}
			fields[string(key)] = value.Interface()
		}

		// Decode time stamp
		t, err := d.Time(influx.Nanosecond, time.Time{})
		if err != nil {
			msg := "ServerHttp: Failed to decode time stamp: " + err.Error()
			cclog.ComponentError(r.name, msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		y, _ := lp.New(
			string(measurement),
			tags,
			r.meta,
			fields,
			t,
		)

		if r.sink != nil {
			r.sink <- y
		}
	}

	// Check for IO errors
	err := d.Err()
	if err != nil {
		msg := "ServerHttp: Failed to decode: " + err.Error()
		cclog.ComponentError(r.name, msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
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
