package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/ClusterCockpit/cc-metric-collector/collectors"
	"github.com/ClusterCockpit/cc-metric-collector/receivers"
	"github.com/ClusterCockpit/cc-metric-collector/sinks"

	//	"strings"
	"sync"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	mr "github.com/ClusterCockpit/cc-metric-collector/internal/metricRouter"
	mct "github.com/ClusterCockpit/cc-metric-collector/internal/multiChanTicker"
)

// List of provided collectors. Which collector should be run can be
// configured at 'collectors' list  in 'config.json'.
//var Collectors = map[string]collectors.MetricCollector{
//	"likwid":     &collectors.LikwidCollector{},
//	"loadavg":    &collectors.LoadavgCollector{},
//	"memstat":    &collectors.MemstatCollector{},
//	"netstat":    &collectors.NetstatCollector{},
//	"ibstat":     &collectors.InfinibandCollector{},
//	"lustrestat": &collectors.LustreCollector{},
//	"cpustat":    &collectors.CpustatCollector{},
//	"topprocs":   &collectors.TopProcsCollector{},
//	"nvidia":     &collectors.NvidiaCollector{},
//	"customcmd":  &collectors.CustomCmdCollector{},
//	"diskstat":   &collectors.DiskstatCollector{},
//	"tempstat":   &collectors.TempCollector{},
//	"ipmistat":   &collectors.IpmiCollector{},
//}

//var Sinks = map[string]sinks.Sink{
//	"influxdb": &sinks.InfluxSink{},
//	"stdout":   &sinks.StdoutSink{},
//	"nats":     &sinks.NatsSink{},
//	"http":     &sinks.HttpSink{},
//}

//var Receivers = map[string]receivers.ReceiverFuncs{
//	"nats": &receivers.NatsReceiver{},
//}

type CentralConfigFile struct {
	Interval            int    `json:"interval"`
	Duration            int    `json:"duration"`
	Pidfile             string `json:"pidfile,omitempty"`
	CollectorConfigFile string `json:"collectors"`
	RouterConfigFile    string `json:"router"`
	SinkConfigFile      string `json:"sinks"`
	ReceiverConfigFile  string `json:"receivers,omitempty"`
}

func LoadCentralConfiguration(file string, config *CentralConfigFile) error {
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(config)
	return err
}

type RuntimeConfig struct {
	Hostname   string
	Interval   time.Duration
	Duration   time.Duration
	CliArgs    map[string]string
	ConfigFile CentralConfigFile

	Router         mr.MetricRouter
	CollectManager collectors.CollectorManager
	SinkManager    sinks.SinkManager
	ReceiveManager receivers.ReceiveManager
	Ticker         mct.MultiChanTicker

	Channels []chan lp.CCMetric
	Sync     sync.WaitGroup
}

func prepare_runcfg() RuntimeConfig {
	r := RuntimeConfig{}
	r.Router = nil
	r.CollectManager = nil
	r.SinkManager = nil
	r.ReceiveManager = nil
	return r
}

//// Structure of the configuration file
//type GlobalConfig struct {
//	Sink           sinks.SinkConfig           `json:"sink"`
//	Interval       int                        `json:"interval"`
//	Duration       int                        `json:"duration"`
//	Collectors     []string                   `json:"collectors"`
//	Receiver       receivers.ReceiverConfig   `json:"receiver"`
//	DefTags        map[string]string          `json:"default_tags"`
//	CollectConfigs map[string]json.RawMessage `json:"collect_config"`
//}

//// Load JSON configuration file
//func LoadConfiguration(file string, config *GlobalConfig) error {
//	configFile, err := os.Open(file)
//	defer configFile.Close()
//	if err != nil {
//		fmt.Println(err.Error())
//		return err
//	}
//	jsonParser := json.NewDecoder(configFile)
//	err = jsonParser.Decode(config)
//	return err
//}

func ReadCli() map[string]string {
	var m map[string]string
	cfg := flag.String("config", "./config.json", "Path to configuration file")
	logfile := flag.String("log", "stderr", "Path for logfile")
	pidfile := flag.String("pidfile", "/var/run/cc-metric-collector.pid", "Path for PID file")
	once := flag.Bool("once", false, "Run all collectors only once")
	flag.Parse()
	m = make(map[string]string)
	m["configfile"] = *cfg
	m["logfile"] = *logfile
	m["pidfile"] = *pidfile
	if *once {
		m["once"] = "true"
	} else {
		m["once"] = "false"
	}
	return m
}

//func SetLogging(logfile string) error {
//	var file *os.File
//	var err error
//	if logfile != "stderr" {
//		file, err = os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
//		if err != nil {
//			log.Fatal(err)
//			return err
//		}
//	} else {
//		file = os.Stderr
//	}
//	log.SetOutput(file)
//	return nil
//}

//func CreatePidfile(pidfile string) error {
//	file, err := os.OpenFile(pidfile, os.O_CREATE|os.O_RDWR, 0600)
//	if err != nil {
//		log.Print(err)
//		return err
//	}
//	file.Write([]byte(fmt.Sprintf("%d", os.Getpid())))
//	file.Close()
//	return nil
//}

//func RemovePidfile(pidfile string) error {
//	info, err := os.Stat(pidfile)
//	if !os.IsNotExist(err) && !info.IsDir() {
//		os.Remove(pidfile)
//	}
//	return nil
//}

// General shutdown function that gets executed in case of interrupt or graceful shutdown
func shutdown(config *RuntimeConfig) {
	log.Print("Shutdown...")
	if config.CollectManager != nil {
		log.Print("Shutdown CollectManager...")
		config.CollectManager.Close()
	}
	if config.ReceiveManager != nil {
		log.Print("Shutdown ReceiveManager...")
		config.ReceiveManager.Close()
	}
	if config.Router != nil {
		log.Print("Shutdown Router...")
		config.Router.Close()
	}
	if config.SinkManager != nil {
		log.Print("Shutdown SinkManager...")
		config.SinkManager.Close()
	}

	//	pidfile := config.ConfigFile.Pidfile
	//	RemovePidfile(pidfile)
	//	pidfile = config.CliArgs["pidfile"]
	//	RemovePidfile(pidfile)
	config.Sync.Done()
}

// Register an interrupt handler for Ctrl+C and similar. At signal,
// all collectors are closed
func prepare_shutdown(config *RuntimeConfig) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	go func(config *RuntimeConfig) {
		<-sigs
		log.Print("Shutdown...")
		shutdown(config)
	}(config)
}

func main() {
	var err error
	use_recv := false

	rcfg := prepare_runcfg()
	rcfg.CliArgs = ReadCli()

	// Load and check configuration
	err = LoadCentralConfiguration(rcfg.CliArgs["configfile"], &rcfg.ConfigFile)
	if err != nil {
		log.Print("Error reading configuration file ", rcfg.CliArgs["configfile"])
		log.Print(err.Error())
		return
	}
	if rcfg.ConfigFile.Interval <= 0 || time.Duration(rcfg.ConfigFile.Interval)*time.Second <= 0 {
		log.Print("Configuration value 'interval' must be greater than zero")
		return
	}
	rcfg.Interval = time.Duration(rcfg.ConfigFile.Interval) * time.Second
	if rcfg.ConfigFile.Duration <= 0 || time.Duration(rcfg.ConfigFile.Duration)*time.Second <= 0 {
		log.Print("Configuration value 'duration' must be greater than zero")
		return
	}
	rcfg.Duration = time.Duration(rcfg.ConfigFile.Duration) * time.Second

	rcfg.Hostname, err = os.Hostname()
	if err != nil {
		log.Print(err.Error())
		return
	}
	// Drop domain part of host name
	rcfg.Hostname = strings.SplitN(rcfg.Hostname, `.`, 2)[0]
	//	err = CreatePidfile(rcfg.CliArgs["pidfile"])
	//	err = SetLogging(rcfg.CliArgs["logfile"])
	//	if err != nil {
	//		log.Print("Error setting up logging system to ", rcfg.CliArgs["logfile"], " on ", rcfg.Hostname)
	//		return
	//	}
	rcfg.Ticker = mct.NewTicker(rcfg.Interval)
	if len(rcfg.ConfigFile.RouterConfigFile) > 0 {
		rcfg.Router, err = mr.New(rcfg.Ticker, &rcfg.Sync, rcfg.ConfigFile.RouterConfigFile)
		if err != nil {
			log.Print(err.Error())
			return
		}
	}
	if len(rcfg.ConfigFile.SinkConfigFile) > 0 {
		rcfg.SinkManager, err = sinks.New(&rcfg.Sync, rcfg.ConfigFile.SinkConfigFile)
		if err != nil {
			log.Print(err.Error())
			return
		}
		RouterToSinksChannel := make(chan lp.CCMetric)
		rcfg.SinkManager.AddInput(RouterToSinksChannel)
		rcfg.Router.AddOutput(RouterToSinksChannel)
	}
	if len(rcfg.ConfigFile.CollectorConfigFile) > 0 {
		rcfg.CollectManager, err = collectors.New(rcfg.Ticker, rcfg.Duration, &rcfg.Sync, rcfg.ConfigFile.CollectorConfigFile)
		if err != nil {
			log.Print(err.Error())
			return
		}
		CollectToRouterChannel := make(chan lp.CCMetric)
		rcfg.CollectManager.AddOutput(CollectToRouterChannel)
		rcfg.Router.AddInput(CollectToRouterChannel)
	}
	if len(rcfg.ConfigFile.ReceiverConfigFile) > 0 {
		rcfg.ReceiveManager, err = receivers.New(&rcfg.Sync, rcfg.ConfigFile.ReceiverConfigFile)
		if err != nil {
			log.Print(err.Error())
			return
		}
		ReceiveToRouterChannel := make(chan lp.CCMetric)
		rcfg.ReceiveManager.AddOutput(ReceiveToRouterChannel)
		rcfg.Router.AddInput(ReceiveToRouterChannel)
		use_recv = true
	}
	prepare_shutdown(&rcfg)
	rcfg.Sync.Add(1)
	rcfg.Router.Start()
	rcfg.SinkManager.Start()
	rcfg.CollectManager.Start()

	if use_recv {
		rcfg.ReceiveManager.Start()
	}
	//	if len(config.Collectors) == 0 {
	//		var keys []string
	//		for k := range Collectors {
	//			keys = append(keys, k)
	//		}
	//		log.Print("Configuration value 'collectors' does not contain any collector. Available: ", strings.Join(keys, ", "))
	//		return
	//	}
	//	for _, name := range config.Collectors {
	//		if _, found := Collectors[name]; !found {
	//			log.Print("Invalid collector '", name, "' in configuration")
	//			return
	//		}
	//	}
	//	if _, found := Sinks[config.Sink.Type]; !found {
	//		log.Print("Invalid sink type '", config.Sink.Type, "' in configuration")
	//		return
	//	}
	//	// Setup sink
	//	sink := Sinks[config.Sink.Type]
	//	err = sink.Init(config.Sink)
	//	if err != nil {
	//		log.Print(err)
	//		return
	//	}
	//	sinkChannel := make(chan bool)
	//	mproxy.Init(sinkChannel, &wg)
	//	// Setup receiver
	//	if len(config.Receiver.Type) > 0 && config.Receiver.Type != "none" {
	//		if _, found := Receivers[config.Receiver.Type]; !found {
	//			log.Print("Invalid receiver type '", config.Receiver.Type, "' in configuration")
	//			return
	//		} else {
	//			recv = Receivers[config.Receiver.Type]
	//			err = recv.Init(config.Receiver, sink)
	//			if err == nil {
	//				use_recv = true
	//			} else {
	//				log.Print(err)
	//			}
	//		}
	//	}

	//	// Register interrupt handler
	//	prepare_shutdown(&wg, &config, sink, recv, clicfg["pidfile"])

	//	// Initialize all collectors
	//	tmp := make([]string, 0)
	//	for _, c := range config.Collectors {
	//		col := Collectors[c]
	//		conf, found := config.CollectConfigs[c]
	//		if !found {
	//			conf = json.RawMessage("")
	//		}
	//		err = col.Init([]byte(conf))
	//		if err != nil {
	//			log.Print("SKIP ", col.Name(), " (", err.Error(), ")")
	//		} else if !col.Initialized() {
	//			log.Print("SKIP ", col.Name(), " (Not initialized)")
	//		} else {
	//			log.Print("Start ", col.Name())
	//			tmp = append(tmp, c)
	//		}
	//	}
	//	config.Collectors = tmp
	//	config.DefTags["hostname"] = host

	//	// Setup up ticker loop
	//	if clicfg["once"] != "true" {
	//		log.Print("Running loop every ", time.Duration(config.Interval)*time.Second)
	//	} else {
	//		log.Print("Running loop only once")
	//	}
	//	ticker := time.NewTicker(time.Duration(config.Interval) * time.Second)
	//	done := make(chan bool)

	//	// Storage for all node metrics
	//	tmpPoints := make([]lp.MutableMetric, 0)

	//	// Start receiver
	//	if use_recv {
	//		recv.Start()
	//	}

	//	go func() {
	//		for {
	//			select {
	//			case <-done:
	//				return
	//			case t := <-ticker.C:

	//				// Read all collectors are sort the results in the right
	//				// storage locations
	//				for _, c := range config.Collectors {
	//					col := Collectors[c]
	//					col.Read(time.Duration(config.Duration), &tmpPoints)

	//					for {
	//						if len(tmpPoints) == 0 {
	//							break
	//						}
	//						p := tmpPoints[0]
	//						for k, v := range config.DefTags {
	//							p.AddTag(k, v)
	//							p.SetTime(t)
	//						}
	//						sink.Write(p)
	//						tmpPoints = tmpPoints[1:]
	//					}
	//				}

	//				if err := sink.Flush(); err != nil {
	//					log.Printf("sink error: %s\n", err)
	//				}
	//				if clicfg["once"] == "true" {
	//					shutdown(&wg, config.Collectors, sink, recv, clicfg["pidfile"])
	//					return
	//				}
	//			}
	//		}
	//	}()

	// Wait until receiving an interrupt
	rcfg.Sync.Wait()
}
