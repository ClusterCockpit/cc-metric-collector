package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/ClusterCockpit/cc-lib/receivers"
	"github.com/ClusterCockpit/cc-lib/sinks"
	"github.com/ClusterCockpit/cc-metric-collector/collectors"

	//	"strings"
	"sync"
	"time"

	ccconf "github.com/ClusterCockpit/cc-lib/ccConfig"
	cclog "github.com/ClusterCockpit/cc-lib/ccLogger"
	lp "github.com/ClusterCockpit/cc-lib/ccMessage"
	mr "github.com/ClusterCockpit/cc-metric-collector/internal/metricRouter"
	mct "github.com/ClusterCockpit/cc-metric-collector/pkg/multiChanTicker"
)

type CentralConfigFile struct {
	Interval string `json:"interval"`
	Duration string `json:"duration"`
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

	Channels []chan lp.CCMessage
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
	once := flag.Bool("once", false, "Run all collectors only once")
	loglevel := flag.String("loglevel", "info", "Set log level")
	flag.Parse()
	m = make(map[string]string)
	m["configfile"] = *cfg
	m["logfile"] = *logfile
	if *once {
		m["once"] = "true"
	} else {
		m["once"] = "false"
	}
	m["loglevel"] = *loglevel
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

	// Set loglevel based on command line input.
	cclog.Init(rcfg.CliArgs["loglevel"], false)

	// Init ccConfig with configuration file
	ccconf.Init(rcfg.CliArgs["configfile"])

	// Load and check configuration
	main := ccconf.GetPackageConfig("main")
	err = json.Unmarshal(main, &rcfg.ConfigFile)
	if err != nil {
		cclog.Error("Error reading configuration file ", rcfg.CliArgs["configfile"], ": ", err.Error())
		return 1
	}

	// Properly use duration parser with inputs like '60s', '5m' or similar
	if len(rcfg.ConfigFile.Interval) > 0 {
		t, err := time.ParseDuration(rcfg.ConfigFile.Interval)
		if err != nil {
			cclog.Error("Configuration value 'interval' no valid duration")
		}
		rcfg.Interval = t
		if rcfg.Interval == 0 {
			cclog.Error("Configuration value 'interval' must be greater than zero")
			return 1
		}
	}

	// Properly use duration parser with inputs like '60s', '5m' or similar
	if len(rcfg.ConfigFile.Duration) > 0 {
		t, err := time.ParseDuration(rcfg.ConfigFile.Duration)
		if err != nil {
			cclog.Error("Configuration value 'duration' no valid duration")
		}
		rcfg.Duration = t
		if rcfg.Duration == 0 {
			cclog.Error("Configuration value 'duration' must be greater than zero")
			return 1
		}
	}
	if rcfg.Duration > rcfg.Interval {
		cclog.Error("The interval should be greater than duration")
		return 1
	}

	routerConf := ccconf.GetPackageConfig("router")
	if len(routerConf) == 0 {
		cclog.Error("Metric router configuration file must be set")
		return 1
	}

	sinkConf := ccconf.GetPackageConfig("sinks")
	if len(sinkConf) == 0 {
		cclog.Error("Sink configuration file must be set")
		return 1
	}

	collectorConf := ccconf.GetPackageConfig("collectors")
	if len(collectorConf) == 0 {
		cclog.Error("Metric collector configuration file must be set")
		return 1
	}

	// Set log file
	// if logfile := rcfg.CliArgs["logfile"]; logfile != "stderr" {
	// 	cclog.SetOutput(logfile)
	// }

	// Creat new multi channel ticker
	rcfg.MultiChanTicker = mct.NewTicker(rcfg.Interval)

	// Create new metric router
	rcfg.MetricRouter, err = mr.New(rcfg.MultiChanTicker, &rcfg.Sync, routerConf)
	if err != nil {
		cclog.Error(err.Error())
		return 1
	}

	// Create new sink
	rcfg.SinkManager, err = sinks.New(&rcfg.Sync, sinkConf)
	if err != nil {
		cclog.Error(err.Error())
		return 1
	}

	// Connect metric router to sink manager
	RouterToSinksChannel := make(chan lp.CCMessage, 200)
	rcfg.SinkManager.AddInput(RouterToSinksChannel)
	rcfg.MetricRouter.AddOutput(RouterToSinksChannel)

	// Create new collector manager
	rcfg.CollectManager, err = collectors.New(rcfg.MultiChanTicker, rcfg.Duration, &rcfg.Sync, collectorConf)
	if err != nil {
		cclog.Error(err.Error())
		return 1
	}

	// Connect collector manager to metric router
	CollectToRouterChannel := make(chan lp.CCMessage, 200)
	rcfg.CollectManager.AddOutput(CollectToRouterChannel)
	rcfg.MetricRouter.AddCollectorInput(CollectToRouterChannel)

	// Create new receive manager
	receiveConf := ccconf.GetPackageConfig("receivers")
	if len(receiveConf) > 0 {
		rcfg.ReceiveManager, err = receivers.New(&rcfg.Sync, receiveConf)
		if err != nil {
			cclog.Error(err.Error())
			return 1
		}

		// Connect receive manager to metric router
		ReceiveToRouterChannel := make(chan lp.CCMessage, 200)
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
		x := 1.2 * float64(rcfg.Interval.Seconds())
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
