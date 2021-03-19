package main

import (
	"fmt"
	"os"
	"os/exec"

	//"bytes"
	//    "context"
	"encoding/json"
	"path/filepath"

	//"sort"
	"errors"
	"strings"
	"time"

	protocol "github.com/influxdata/line-protocol"
)

type GlobalConfig struct {
	Sink struct {
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"sink"`
	Host   string `json:"host"`
	Port   string `json:"port"`
	Report struct {
		Levels   string `json:"levels"`
		Interval int    `json:"interval"`
	} `json:"report"`
	Schedule struct {
		Core struct {
			Frequency int `json:"frequency"`
			Duration  int `json:"duration"`
		} `json:"core"`
		Node struct {
			Frequency int `json:"frequency"`
			Duration  int `json:"duration"`
		} `json:"node"`
	} `json:"schedule"`
	Metrics       []string `json:"metrics"`
	CollectorPath string   `json:"collector_path"`
}

type CollectorConfig struct {
	Command  string   `json:"command"`
	Args     string   `json:"arguments"`
	Provides []string `json:"provides"`
}

type InternalCollectorConfig struct {
	Config   CollectorConfig
	Location string
	LastRun  time.Time
	encoder  *protocol.Encoder
}

//////////////////////////////////////////////////////////////////////////////
// Load global configuration from JSON file
//////////////////////////////////////////////////////////////////////////////
func LoadGlobalConfiguration(file string, config *GlobalConfig) error {
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		return err
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(config)
	return err
}

//////////////////////////////////////////////////////////////////////////////
// Load collector configuration from JSON file
//////////////////////////////////////////////////////////////////////////////
func LoadCollectorConfiguration(file string, config *CollectorConfig) error {
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		return err
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(config)
	return err
}

//////////////////////////////////////////////////////////////////////////////
// Load collector configurations
//////////////////////////////////////////////////////////////////////////////
func GetSingleCollector(folders *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			configfile := filepath.Join(path, "config.json")
			if _, err := os.Stat(configfile); err == nil {
				// TODO: Validate config?
				p, err := filepath.Abs(path)
				if err == nil {
					*folders = append(*folders, p)
				}
			}
		}
		return nil
	}
}

func GetCollectorFolders(root string, folders *[]string) error {
	err := filepath.Walk(root, GetSingleCollector(folders))
	if err != nil {
		err = errors.New("Cannot get collectors")
	}
	return err
}

//////////////////////////////////////////////////////////////////////////////
// Setup all collectors
//////////////////////////////////////////////////////////////////////////////
func SetupCollectors(config GlobalConfig) ([]InternalCollectorConfig, error) {
	var folders []string
	var outconfig []InternalCollectorConfig
	//encoder := protocol.NewEncoder(buf)
	//encoder.SetMaxLineBytes(1024)
	GetCollectorFolders(config.CollectorPath, &folders)
	for _, path := range folders {
		var col_config InternalCollectorConfig
		LoadCollectorConfiguration(filepath.Join(path, "config.json"), &col_config.Config)
		col_config.LastRun = time.Now()
		col_config.Location = path
		//buf := &bytes.Buffer{}
		//col_config.Encoder := protocol.NewEncoder(buf)
		//col_config.Encoder.SetMaxLineBytes(1024)
		outconfig = append(outconfig, col_config)
	}
	return outconfig, nil
}

//////////////////////////////////////////////////////////////////////////////
// Run collector
//////////////////////////////////////////////////////////////////////////////
func RunCollector(config InternalCollectorConfig) ([]string, error) {
	var results []string
	var err error
	cmd := config.Config.Command

	if _, err = os.Stat(cmd); err != nil {
		//fmt.Println(err.Error())
		if !strings.HasPrefix(cmd, "/") {
			cmd = filepath.Join(config.Location, config.Config.Command)
			if _, err = os.Stat(cmd); err != nil {
				//fmt.Println(err.Error())
				cmd, err = exec.LookPath(config.Config.Command)
			}
		}
	}
	if err != nil {
		fmt.Println(err.Error())
		return results, err
	}

	// TODO: Add timeout

	command := exec.Command(cmd, config.Config.Args)
	command.Dir = config.Location
	command.Wait()
	stdout, err := command.Output()
	if err != nil {
		//log.error(err.Error())
		fmt.Println(err.Error())
		return results, err
	}

	lines := strings.Split(string(stdout), "\n")

	for _, l := range lines {
		if strings.HasPrefix(l, "#") {
			continue
		}
		results = append(results, l)
	}
	return results, err
}

//////////////////////////////////////////////////////////////////////////////
// Setup sink
//////////////////////////////////////////////////////////////////////////////
func SetupSink(config GlobalConfig) chan string {

	c := make(chan string, 300)

	// TODO: Setup something for sending? Establish HTTP connection?
	return c
}

func RunSink(config GlobalConfig, queue *chan string) (*time.Ticker, chan bool) {

	interval := time.Duration(config.Report.Interval) * time.Second
	ticker := time.NewTicker(interval)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				fmt.Println("SinkTick at", t)
				empty := false
				var batch []string
				for empty == false {
					select {
					case metric := <-*queue:
						fmt.Println(metric)
						batch = append(batch, metric)
					default:
						// No metric available, wait for the next iteration
						empty = true
						break
					}
				}
				for _, m := range batch {
					fmt.Println(m)
				}
			}
		}
	}()
	return ticker, done
}

func CloseSink(config GlobalConfig, queue *chan string, ticker *time.Ticker, done chan bool) {
	ticker.Stop()
	done <- true
	close(*queue)
}

func MainLoop(config GlobalConfig, sink *chan string) (*time.Ticker, chan bool) {
	var intConfig []InternalCollectorConfig
	intConfig, err := SetupCollectors(config)
	if err != nil {
		panic(err)
	}

	interval := time.Duration(config.Schedule.Node.Frequency) * time.Second
	ticker := time.NewTicker(time.Second)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				fmt.Println("CollectorTick at", t)
				unix := time.Now()
				for i, _ := range intConfig {
					if time.Duration(unix.Sub(intConfig[i].LastRun)) > interval {
						res, err := RunCollector(intConfig[i])
						if err != nil {
							//log.error("Collector failed: ", err.Error())
						} else {
							//TODO: parse and skip in case of error, encode to []string
							for _, r := range res {
								if len(r) > 0 {
									*sink <- r
								}
							}
						}
						intConfig[i].LastRun = time.Now()
					}
				}
			}
		}
	}()
	return ticker, done
}

func main() {
	//    fmt.Println("Hello")
	//    cmd_opts := []string{"la","le","lu"}
	//    cmd := "echo"
	//    s := run_cmd(cmd, cmd_opts)
	//    fmt.Println(s)
	//    tags := map[string]string {
	//        "host" : "broadep2",
	//    }
	//    fields := map[string]interface{} {
	//        "value" : float64(1.0),
	//    }
	//    fmt.Println(CreatePoint("flops_any", tags, fields, time.Now().UnixNano()))
	var config GlobalConfig
	LoadGlobalConfiguration("config.json", &config)

	queue := SetupSink(config)
	sinkTicker, sinkDone := RunSink(config, &queue)
	collectTicker, collectDone := MainLoop(config, &queue)
	time.Sleep(1600 * time.Second)
	collectTicker.Stop()
	collectDone <- true
	CloseSink(config, &queue, sinkTicker, sinkDone)

	//	var folders []string
	//	GetCollectorFolders(config.CollectorPath, &folders)

	//	for _, path := range folders {
	//		var col_config CollectorConfig
	//		LoadCollectorConfiguration(filepath.Join(path, "config.json"), &col_config)
	//		stdout := run_cmd(filepath.Join(path, col_config.Command), col_config.Args)

	//		metrics := strings.Split(stdout, "\n")
	//		for _, m := range metrics {
	//			if len(m) > 0 {
	//				t := strings.Fields(m)
	//				if len(t) == 2 {
	//					var s strings.Builder
	//					fmt.Fprintf(&s, "%s %d", m, time.Now().UnixNano())
	//					m = s.String()
	//				}
	//				fmt.Println("SEND", m)
	//			}
	//		}
	//	}
}
