package main

import (
	"encoding/json"
	"flag"
//	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/ClusterCockpit/cc-metric-collector/collectors"
	"github.com/ClusterCockpit/cc-metric-collector/receivers"
	"github.com/ClusterCockpit/cc-metric-collector/sinks"

	//	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/internal/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/internal/ccMetric"
	mr "github.com/ClusterCockpit/cc-metric-collector/internal/metricRouter"
	mct "github.com/ClusterCockpit/cc-metric-collector/internal/multiChanTicker"
)

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
		cclog.Error(err.Error())
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
	return RuntimeConfig{
		Router:         nil,
		CollectManager: nil,
		SinkManager:    nil,
		ReceiveManager: nil,
	}
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
	debug := flag.Bool("debug", false, "Activate debug output")
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
	if *debug {
		m["debug"] = "true"
		cclog.SetDebug()
	} else {
		m["debug"] = "false"
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
	cclog.Info("Shutdown...")
	if config.CollectManager != nil {
		cclog.Debug("Shutdown CollectManager...")
		config.CollectManager.Close()
	}
	if config.ReceiveManager != nil {
		cclog.Debug("Shutdown ReceiveManager...")
		config.ReceiveManager.Close()
	}
	if config.Router != nil {
		cclog.Debug("Shutdown Router...")
		config.Router.Close()
	}
	if config.SinkManager != nil {
		cclog.Debug("Shutdown SinkManager...")
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
		shutdown(config)
	}(config)
}

func mainFunc() int {
	var err error
	use_recv := false

	rcfg := prepare_runcfg()
	rcfg.CliArgs = ReadCli()

	// Load and check configuration
	err = LoadCentralConfiguration(rcfg.CliArgs["configfile"], &rcfg.ConfigFile)
	if err != nil {
		cclog.Error("Error reading configuration file ", rcfg.CliArgs["configfile"], ": ", err.Error())
		return 1
	}
	if rcfg.ConfigFile.Interval <= 0 || time.Duration(rcfg.ConfigFile.Interval)*time.Second <= 0 {
		cclog.Error("Configuration value 'interval' must be greater than zero")
		return 1
	}
	rcfg.Interval = time.Duration(rcfg.ConfigFile.Interval) * time.Second
	if rcfg.ConfigFile.Duration <= 0 || time.Duration(rcfg.ConfigFile.Duration)*time.Second <= 0 {
		cclog.Error("Configuration value 'duration' must be greater than zero")
		return 1
	}
	rcfg.Duration = time.Duration(rcfg.ConfigFile.Duration) * time.Second

	rcfg.Hostname, err = os.Hostname()
	if err != nil {
		cclog.Error(err.Error())
		return 1
	}
	// Drop domain part of host name
	rcfg.Hostname = strings.SplitN(rcfg.Hostname, `.`, 2)[0]
	//	err = CreatePidfile(rcfg.CliArgs["pidfile"])

	if rcfg.CliArgs["logfile"] != "stderr" {
	    cclog.SetOutput(rcfg.CliArgs["logfile"])
	}
	//	err = SetLogging(rcfg.CliArgs["logfile"])
	//	if err != nil {
	//		log.Print("Error setting up logging system to ", rcfg.CliArgs["logfile"], " on ", rcfg.Hostname)
	//		return
	//	}
	rcfg.Ticker = mct.NewTicker(rcfg.Interval)
	if len(rcfg.ConfigFile.RouterConfigFile) > 0 {
		rcfg.Router, err = mr.New(rcfg.Ticker, &rcfg.Sync, rcfg.ConfigFile.RouterConfigFile)
		if err != nil {
			cclog.Error(err.Error())
			return 1
		}
	}
	if len(rcfg.ConfigFile.SinkConfigFile) > 0 {
		rcfg.SinkManager, err = sinks.New(&rcfg.Sync, rcfg.ConfigFile.SinkConfigFile)
		if err != nil {
			cclog.Error(err.Error())
			return 1
		}
		RouterToSinksChannel := make(chan lp.CCMetric)
		rcfg.SinkManager.AddInput(RouterToSinksChannel)
		rcfg.Router.AddOutput(RouterToSinksChannel)
	}
	if len(rcfg.ConfigFile.CollectorConfigFile) > 0 {
		rcfg.CollectManager, err = collectors.New(rcfg.Ticker, rcfg.Duration, &rcfg.Sync, rcfg.ConfigFile.CollectorConfigFile)
		if err != nil {
			cclog.Error(err.Error())
			return 1
		}
		CollectToRouterChannel := make(chan lp.CCMetric)
		rcfg.CollectManager.AddOutput(CollectToRouterChannel)
		rcfg.Router.AddInput(CollectToRouterChannel)
	}
	if len(rcfg.ConfigFile.ReceiverConfigFile) > 0 {
		rcfg.ReceiveManager, err = receivers.New(&rcfg.Sync, rcfg.ConfigFile.ReceiverConfigFile)
		if err != nil {
			cclog.Error(err.Error())
			return 1
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

	// Wait until one tick has passed. This is a workaround
	if rcfg.CliArgs["once"] == "true" {
		x := 1.8 * float64(rcfg.ConfigFile.Interval)
		time.Sleep(time.Duration(int(x)) * time.Second)
		shutdown(&rcfg)
	}

	// Wait until receiving an interrupt
	rcfg.Sync.Wait()
	return 0
}

func main() {
	exitCode := mainFunc()
	os.Exit(exitCode)
}
