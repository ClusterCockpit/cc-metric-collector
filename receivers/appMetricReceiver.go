package receivers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	influx "github.com/influxdata/line-protocol"
)

// SampleReceiver configuration: receiver type, listen address, port
type AppMetricReceiverConfig struct {
	Type       string `json:"type"`
	SocketFile string `json:"socket_file"`
}

type AppMetricReceiver struct {
	receiver
	config AppMetricReceiverConfig

	// Storage for static information
	meta map[string]string
	// Use in case of own go routine
	done chan bool
	wg   sync.WaitGroup
	// Influx stuff
	handler *influx.MetricHandler
	parser  *influx.Parser
	// WaitGroup for individual connections
	connWg   sync.WaitGroup
	listener net.Listener
}

func (r *AppMetricReceiver) newConnection(conn net.Conn) {
	//defer conn.Close()
	//defer wg.Done()

	buffer, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		conn.Close()
		return
	}

	metrics, err := r.parser.Parse(buffer)
	if err != nil {
		cclog.ComponentError(r.name, "failed to parse received metrics")
		return
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

	r.newConnection(conn)

}

func (r *AppMetricReceiver) newAccepter(listenSocket net.Listener) {
accept_loop:
	for {
		select {
		case <-r.done:
			break accept_loop
		default:
			conn, err := listenSocket.Accept()
			if err == nil {
				r.connWg.Add(1)
				go func() {
					r.newConnection(conn)
					r.connWg.Done()
				}()
			}
		}
	}
	r.wg.Done()
}

// Implement functions required for Receiver interface
// Start(), Close()
// See: metricReceiver.go

func (r *AppMetricReceiver) Start() {
	var err error = nil
	cclog.ComponentDebug(r.name, "START")

	r.listener, err = net.Listen("unix", r.config.SocketFile)
	if err != nil {
		cclog.ComponentError(r.name, "failed to listen at socket", r.config.SocketFile)
	}
	if _, err := os.Stat(r.config.SocketFile); err != nil {
		cclog.ComponentError(r.name, "failed to create socket", r.config.SocketFile)
	}

	r.done = make(chan bool)
	r.wg.Add(1)
	go r.newAccepter(r.listener)

}

// Close receiver: close network connection, close files, close libraries, ...
func (r *AppMetricReceiver) Close() {
	cclog.ComponentDebug(r.name, "CLOSE")

	if _, err := os.Stat(r.config.SocketFile); err == nil {
		if err := os.RemoveAll(r.config.SocketFile); err != nil {
			cclog.ComponentError(r.name, "Failed to remove UNIX socket", r.config.SocketFile)
		}
	}

	// in case of own go routine, send the signal and wait
	r.listener.Close()
	r.done <- true
	close(r.done)
	r.connWg.Wait()
	r.wg.Wait()
}

// New function to create a new instance of the receiver
// Initialize the receiver by giving it a name and reading in the config JSON
func NewAppMetricReceiver(name string, config json.RawMessage) (Receiver, error) {
	r := new(AppMetricReceiver)

	// Set name of SampleReceiver
	// The name should be chosen in such a way that different instances of SampleReceiver can be distinguished
	r.name = fmt.Sprintf("AppMetricReceiver(%s)", name)

	// Set static information
	r.meta = map[string]string{"source": r.name}

	// Set defaults in r.config
	// Allow overwriting these defaults by reading config JSON
	r.config.SocketFile = "/tmp/cc.sock"

	// Read the sample receiver specific JSON config
	if len(config) > 0 {
		err := json.Unmarshal(config, &r.config)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return nil, err
		}
	}
	if len(r.config.SocketFile) == 0 {
		cclog.ComponentError(r.name, "Invalid socket_file setting:", r.config.SocketFile)
		return nil, fmt.Errorf("invalid socket_file setting: %s", r.config.SocketFile)
	}

	// Check that all required fields in the configuration are set
	// Use 'if len(r.config.Option) > 0' for strings
	r.handler = influx.NewMetricHandler()
	r.parser = influx.NewParser(r.handler)
	r.parser.SetTimeFunc(DefaultTime)

	return r, nil
}
