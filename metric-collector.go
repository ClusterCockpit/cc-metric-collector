package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/ClusterCockpit/cc-metric-collector/collectors"
)

var lsc collectors.LoadstatCollector
var msc collectors.MemstatCollector
var nsc collectors.NetstatCollector

var Collectors = map[string]collectors.MetricGetter{
	"loadstat": &lsc,
	"memstat":  &msc,
	"netstat":  &nsc}

type GlobalConfig struct {
	Sink struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Host     string `json:"host"`
		Port     string `json:"port"`
	} `json:"sink"`
	Interval   int      `json:"interval"`
	Collectors []string `json:"collectors"`
}

func LoadConfiguration(file string, config *GlobalConfig) error {
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(config)
	return err
}

func shutdown(wg *sync.WaitGroup, config *GlobalConfig) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	go func(wg *sync.WaitGroup) {
		<-sigs
		log.Print("Shutdown...")
		for _, c := range config.Collectors {
			col := Collectors[c]
			log.Print("Stop ", c)
			col.Stop()
		}
		time.Sleep(1 * time.Second)
		wg.Done()
	}(wg)
}

func main() {
	var config GlobalConfig
	var wg sync.WaitGroup
	wg.Add(1)

	LoadConfiguration("config.json", &config)
	shutdown(&wg, &config)

	for _, c := range config.Collectors {
		col := Collectors[c]
		log.Print("Start ", c)
		col.Init()
		col.Start(time.Duration(config.Interval))
	}

	wg.Wait()
}
