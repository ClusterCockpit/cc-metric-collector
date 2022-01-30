package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"syscall"

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
	if err != nil {
		cclog.Error(err.Error())
		return err
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(config)
	return err
}

type RuntimeConfig struct {
	Interval   time.Duration
	Duration   time.Duration
	CliArgs    map[string]string
	ConfigFile CentralConfigFile

	MetricRouter    mr.MetricRouter
	CollectManager  collectors.CollectorManager
	SinkManager     sinks.SinkManager
	ReceiveManager  receivers.ReceiveManager
	MultiChanTicker mct.MultiChanTicker

	Channels []chan lp.CCMetric
	Sync     sync.WaitGroup
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

// General shutdownHandler function that gets executed in case of interrupt or graceful shutdownHandler
func shutdownHandler(config *RuntimeConfig, shutdownSignal chan os.Signal) {
	defer config.Sync.Done()

	<-shutdownSignal
	// Remove shutdown handler
	// every additional interrupt signal will stop without cleaning up
	signal.Stop(shutdownSignal)

	cclog.Info("Shutdown...")

	cclog.Debug("Shutdown Ticker...")
	config.MultiChanTicker.Close()

	if config.CollectManager != nil {
		cclog.Debug("Shutdown CollectManager...")
		config.CollectManager.Close()
	}
	if config.ReceiveManager != nil {
		cclog.Debug("Shutdown ReceiveManager...")
		config.ReceiveManager.Close()
	}
	if config.MetricRouter != nil {
		cclog.Debug("Shutdown Router...")
		config.MetricRouter.Close()
	}
	if config.SinkManager != nil {
		cclog.Debug("Shutdown SinkManager...")
		config.SinkManager.Close()
	}

	//	pidfile := config.ConfigFile.Pidfile
	//	RemovePidfile(pidfile)
	//	pidfile = config.CliArgs["pidfile"]
	//	RemovePidfile(pidfile)
}

func mainFunc() int {
	var err error
	use_recv := false

	// Initialize runtime configuration
	rcfg := RuntimeConfig{
		MetricRouter:   nil,
		CollectManager: nil,
		SinkManager:    nil,
		ReceiveManager: nil,
		CliArgs:        ReadCli(),
	}

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

	if len(rcfg.ConfigFile.RouterConfigFile) == 0 {
		cclog.Error("Metric router configuration file must be set")
		return 1
	}

	if len(rcfg.ConfigFile.SinkConfigFile) == 0 {
		cclog.Error("Sink configuration file must be set")
		return 1
	}

	if len(rcfg.ConfigFile.CollectorConfigFile) == 0 {
		cclog.Error("Metric collector configuration file must be set")
		return 1
	}

	//	err = CreatePidfile(rcfg.CliArgs["pidfile"])

	// Set log file
	if logfile := rcfg.CliArgs["logfile"]; logfile != "stderr" {
		cclog.SetOutput(logfile)
	}

	// Creat new multi channel ticker
	rcfg.MultiChanTicker = mct.NewTicker(rcfg.Interval)

	// Create new metric router
	rcfg.MetricRouter, err = mr.New(rcfg.MultiChanTicker, &rcfg.Sync, rcfg.ConfigFile.RouterConfigFile)
	if err != nil {
		cclog.Error(err.Error())
		return 1
	}

	// Create new sink
	rcfg.SinkManager, err = sinks.New(&rcfg.Sync, rcfg.ConfigFile.SinkConfigFile)
	if err != nil {
		cclog.Error(err.Error())
		return 1
	}

	// Connect metric router to sink manager
	RouterToSinksChannel := make(chan lp.CCMetric, 200)
	rcfg.SinkManager.AddInput(RouterToSinksChannel)
	rcfg.MetricRouter.AddOutput(RouterToSinksChannel)

	// Create new collector manager
	rcfg.CollectManager, err = collectors.New(rcfg.MultiChanTicker, rcfg.Duration, &rcfg.Sync, rcfg.ConfigFile.CollectorConfigFile)
	if err != nil {
		cclog.Error(err.Error())
		return 1
	}

	// Connect collector manager to metric router
	CollectToRouterChannel := make(chan lp.CCMetric, 200)
	rcfg.CollectManager.AddOutput(CollectToRouterChannel)
	rcfg.MetricRouter.AddCollectorInput(CollectToRouterChannel)

	// Create new receive manager
	if len(rcfg.ConfigFile.ReceiverConfigFile) > 0 {
		rcfg.ReceiveManager, err = receivers.New(&rcfg.Sync, rcfg.ConfigFile.ReceiverConfigFile)
		if err != nil {
			cclog.Error(err.Error())
			return 1
		}

		// Connect receive manager to metric router
		ReceiveToRouterChannel := make(chan lp.CCMetric, 200)
		rcfg.ReceiveManager.AddOutput(ReceiveToRouterChannel)
		rcfg.MetricRouter.AddReceiverInput(ReceiveToRouterChannel)
		use_recv = true
	}

	// Create shutdown handler
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt)
	signal.Notify(shutdownSignal, syscall.SIGTERM)
	rcfg.Sync.Add(1)
	go shutdownHandler(&rcfg, shutdownSignal)

	// Start the managers
	rcfg.MetricRouter.Start()
	rcfg.SinkManager.Start()
	rcfg.CollectManager.Start()

	if use_recv {
		rcfg.ReceiveManager.Start()
	}

	// Wait until one tick has passed. This is a workaround
	if rcfg.CliArgs["once"] == "true" {
		x := 1.2 * float64(rcfg.ConfigFile.Interval)
		time.Sleep(time.Duration(int(x)) * time.Second)
		shutdownSignal <- os.Interrupt
	}

	// Wait that all goroutines finish
	rcfg.Sync.Wait()

	return 0
}

func main() {
	exitCode := mainFunc()
	os.Exit(exitCode)
}
