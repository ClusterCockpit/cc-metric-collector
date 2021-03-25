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
	protocol "github.com/influxdata/line-protocol"
)

var Collectors = map[string]collectors.MetricGetter{
	"likwid":   &collectors.LikwidCollector{},
	"loadavg": &collectors.LoadavgCollector{},
	"memstat":  &collectors.MemstatCollector{},
	"netstat":  &collectors.NetstatCollector{},
	"ibstat":  &collectors.InfinibandCollector{},
	"lustrestat":  &collectors.LustreCollector{},
}
type GlobalConfig struct {
	Sink struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Host     string `json:"host"`
		Port     string `json:"port"`
	} `json:"sink"`
	Interval   int      `json:"interval"`
	Duration   int      `json:"duration"`
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
			log.Print("Stop ", col.Name())
			col.Close()
		}
		time.Sleep(1 * time.Second)
		wg.Done()
	}(wg)
}

func main() {
	var config GlobalConfig
	var wg sync.WaitGroup
	wg.Add(1)
	host, err := os.Hostname()
    if err != nil {
        log.Print(err)
        return
    }
    var tags = map[string]string {"host": host}

	LoadConfiguration("config.json", &config)
	if config.Interval <= 0 || time.Duration(config.Interval) * time.Second <= 0 {
	    log.Print("Configuration value 'interval' must be greater than zero")
	    return
	}
	if config.Duration <= 0 {
	    log.Print("Configuration value 'duration' must be greater than zero")
	    return
	}
	shutdown(&wg, &config)
    serializer := protocol.NewEncoder(os.Stdout)
	serializer.SetPrecision(time.Second)
	serializer.SetMaxLineBytes(1024)
	for _, c := range config.Collectors {
		col := Collectors[c]
		col.Init()
		log.Print("Start ", col.Name())
	}
	log.Print(config.Interval, time.Duration(config.Interval) * time.Second)
	ticker := time.NewTicker(time.Duration(config.Interval) * time.Second)
	done := make(chan bool)
	node_fields := make(map[string]interface{})
	slist := collectors.SocketList()
    sockets_fields := make(map[int]map[string]interface{}, len(slist))
    for _, s := range slist {
        sockets_fields[s] = make(map[string]interface{})
    }
    clist := collectors.CpuList()
    cpu_fields := make(map[int]map[string]interface{}, len(clist))
    for _, s := range clist {
        cpu_fields[s] = make(map[string]interface{})
    }

	go func() {
		for {
			select {
			case <-done:
				return
			case t:= <-ticker.C:
			    
			    
			    scount := 0
			    ccount := 0
			    for _, c := range config.Collectors {
			        col := Collectors[c]
				    col.Read(time.Duration(config.Duration))
		            for key, val := range col.GetNodeMetric() {
	                    node_fields[key] = val
	                }
                    for sid, socket := range col.GetSocketMetrics() {
                        for key, val := range socket {
    	                    sockets_fields[sid][key] = val
    	                    scount++
    	                }
	                }
	                for cid, cpu := range col.GetCpuMetrics() {
                        for key, val := range cpu {
    	                    cpu_fields[cid][key] = val
    	                    ccount++
    	                }
	                }
				}
				var CurrentNode protocol.MutableMetric
				CurrentNode, err = protocol.New("node", tags, node_fields, t)
				if err != nil {
			        log.Print(err)
		        }
				_, err := serializer.Encode(CurrentNode)
		        if err != nil {
			        log.Print(err)
		        }
		        if scount > 0 {
		            for sid, socket := range sockets_fields {
		                var CurrentSocket protocol.MutableMetric
		                var stags = map[string]string {"socket": fmt.Sprintf("%d", sid), "host": host}
		                CurrentSocket, err = protocol.New("socket", stags, socket, t)
				        if err != nil {
			                log.Print(err)
		                }
		                _, err := serializer.Encode(CurrentSocket)
		                if err != nil {
			                log.Print(err)
		                }
		            }
		        }
		        if ccount > 0 {
		            for cid, cpu := range cpu_fields {
		                var CurrentCpu protocol.MutableMetric
		                var ctags = map[string]string {"host": host, "cpu": fmt.Sprintf("%d", cid)}
		                CurrentCpu, err = protocol.New("cpu", ctags, cpu, t)
				        if err != nil {
			                log.Print(err)
		                }
		                _, err := serializer.Encode(CurrentCpu)
		                if err != nil {
			                log.Print(err)
		                }
		            }
		        }
			}
		}
	}()


	wg.Wait()
}
