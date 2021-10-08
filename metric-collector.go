package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ClusterCockpit/cc-metric-collector/collectors"
	"github.com/ClusterCockpit/cc-metric-collector/receivers"
	"github.com/ClusterCockpit/cc-metric-collector/sinks"
	lp "github.com/influxdata/line-protocol"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

// List of provided collectors. Which collector should be run can be
// configured at 'collectors' list  in 'config.json'.
var Collectors = map[string]collectors.MetricGetter{
	"likwid":     &collectors.LikwidCollector{},
	"loadavg":    &collectors.LoadavgCollector{},
	"memstat":    &collectors.MemstatCollector{},
	"netstat":    &collectors.NetstatCollector{},
	"ibstat":     &collectors.InfinibandCollector{},
	"lustrestat": &collectors.LustreCollector{},
	"cpustat":    &collectors.CpustatCollector{},
	"topprocs":   &collectors.TopProcsCollector{},
	"nvidia":     &collectors.NvidiaCollector{},
	"customcmd":  &collectors.CustomCmdCollector{},
	"diskstat":   &collectors.DiskstatCollector{},
}

var Sinks = map[string]sinks.SinkFuncs{
	"influxdb": &sinks.InfluxSink{},
	"stdout":   &sinks.StdoutSink{},
	"nats":     &sinks.NatsSink{},
}

var Receivers = map[string]receivers.ReceiverFuncs{
	"nats": &receivers.NatsReceiver{},
}

// Structure of the configuration file
type GlobalConfig struct {
	Sink       sinks.SinkConfig         `json:"sink"`
	Interval   int                      `json:"interval"`
	Duration   int                      `json:"duration"`
	Collectors []string                 `json:"collectors"`
	Receiver   receivers.ReceiverConfig `json:"receiver"`
	DefTags    map[string]string        `json:"default_tags"`
}

// Load JSON configuration file
func LoadConfiguration(file string, config *GlobalConfig) error {
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(config)
	return err
}

func ReadCli() map[string]string {
	var m map[string]string
	cfg := flag.String("config", "./config.json", "Path to configuration file")
	logfile := flag.String("log", "stderr", "Path for logfile")
	pidfile := flag.String("pidfile", "/var/run/cc-metric-collector.pid", "Path for PID file")
	flag.Parse()
	m = make(map[string]string)
	m["configfile"] = *cfg
	m["logfile"] = *logfile
	m["pidfile"] = *pidfile
	return m
}

func SetLogging(logfile string) error {
	var file *os.File
	var err error
	if logfile != "stderr" {
		file, err = os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal(err)
			return err
		}
	} else {
		file = os.Stderr
	}
	log.SetOutput(file)
	return nil
}

func CreatePidfile(pidfile string) error {
	file, err := os.OpenFile(pidfile, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		log.Print(err)
		return err
	}
	file.Write([]byte(fmt.Sprintf("%d", os.Getpid())))
	file.Close()
	return nil
}

func RemovePidfile(pidfile string) error {
	info, err := os.Stat(pidfile)
	if !os.IsNotExist(err) && !info.IsDir() {
		os.Remove(pidfile)
	}
	return nil
}

// Register an interrupt handler for Ctrl+C and similar. At signal,
// all collectors are closed
func shutdown(wg *sync.WaitGroup, config *GlobalConfig, sink sinks.SinkFuncs, recv receivers.ReceiverFuncs, pidfile string) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	go func(wg *sync.WaitGroup) {
		<-sigs
		log.Print("Shutdown...")
		for _, c := range config.Collectors {
			col := Collectors[c]
			log.Print("Stop ", col.Name())
			col.Close()
		}
		time.Sleep(1 * time.Second)
		if recv != nil {
			recv.Close()
		}
		sink.Close()
		RemovePidfile(pidfile)
		wg.Done()
	}(wg)
}

func main() {
	var config GlobalConfig
	var wg sync.WaitGroup
	var recv receivers.ReceiverFuncs = nil
	var use_recv bool
	use_recv = false
	wg.Add(1)
	host, err := os.Hostname()
	if err != nil {
		log.Print(err)
		return
	}
	clicfg := ReadCli()
	err = CreatePidfile(clicfg["pidfile"])
	err = SetLogging(clicfg["logfile"])
	if err != nil {
		log.Print("Error setting up logging system to ", clicfg["logfile"], " on ", host)
		return
	}

	// Load and check configuration
	err = LoadConfiguration(clicfg["configfile"], &config)
	if err != nil {
		log.Print("Error reading configuration file ", clicfg["configfile"])
		return
	}
	if config.Interval <= 0 || time.Duration(config.Interval)*time.Second <= 0 {
		log.Print("Configuration value 'interval' must be greater than zero")
		return
	}
	if config.Duration <= 0 {
		log.Print("Configuration value 'duration' must be greater than zero")
		return
	}
	if len(config.Collectors) == 0 {
		var keys []string
		for k := range Collectors {
			keys = append(keys, k)
		}
		log.Print("Configuration value 'collectors' does not contain any collector. Available: ", strings.Join(keys, ", "))
		return
	}
	for _, name := range config.Collectors {
		if _, found := Collectors[name]; !found {
			log.Print("Invalid collector '", name, "' in configuration")
			return
		}
	}
	if _, found := Sinks[config.Sink.Type]; !found {
		log.Print("Invalid sink type '", config.Sink.Type, "' in configuration")
		return
	}
	// Setup sink
	sink := Sinks[config.Sink.Type]
	err = sink.Init(config.Sink)
	if err != nil {
		log.Print(err)
		return
	}
	// Setup receiver
	if len(config.Receiver.Type) > 0 && config.Receiver.Type != "none" {
		if _, found := Receivers[config.Receiver.Type]; !found {
			log.Print("Invalid receiver type '", config.Receiver.Type, "' in configuration")
			return
		} else {
			recv = Receivers[config.Receiver.Type]
			err = recv.Init(config.Receiver, sink)
			if err == nil {
				use_recv = true
			} else {
				log.Print(err)
			}
		}
	}

	// Register interrupt handler
	shutdown(&wg, &config, sink, recv, clicfg["pidfile"])

	// Initialize all collectors
	tmp := make([]string, 0)
	for _, c := range config.Collectors {
		col := Collectors[c]
		err = col.Init()
		if err != nil {
			log.Print("SKIP ", col.Name())
		} else {
			log.Print("Start ", col.Name())
			tmp = append(tmp, c)
		}
	}
	config.Collectors = tmp
	config.DefTags["hostname"] = host

	// Setup up ticker loop
	log.Print("Running loop every ", time.Duration(config.Interval)*time.Second)
	ticker := time.NewTicker(time.Duration(config.Interval) * time.Second)
	done := make(chan bool)

	// Storage for all node metrics
	tmpPoints := make([]lp.MutableMetric, 0)

	// Start receiver
	if use_recv {
		recv.Start()
	}

	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:

				// Read all collectors are sort the results in the right
				// storage locations
				for _, c := range config.Collectors {
					col := Collectors[c]
					col.Read(time.Duration(config.Duration), &tmpPoints)

					for {
						if len(tmpPoints) == 0 {
							break
						}
						p := tmpPoints[0]
						for k, v := range config.DefTags {
							p.AddTag(k, v)
							p.SetTime(t)
						}
						sink.Write(p)
						tmpPoints = tmpPoints[1:]
					}
				}
			}
		}
	}()

	// Wait until receiving an interrupt
	wg.Wait()
}
