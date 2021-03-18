package main

import (
	"fmt"
	"os"
	"os/exec"

	//    "context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	Command string `json:"command"`
	Args    string `json:"arguments"`
}

func LoadGlobalConfiguration(file string, config *GlobalConfig) error {
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(config)
	return err
}

func LoadCollectorConfiguration(file string, config *CollectorConfig) error {
	configFile, err := os.Open(file)
	defer configFile.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(config)
	return err
}

func SortStringStringMap(input map[string]string) []string {
	keys := make([]string, 0, len(input))
	output := make([]string, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		var s strings.Builder
		fmt.Fprintf(&s, "%s=%s", k, string(input[k]))
		output[i] = s.String()
	}
	return output
}

func SortStringInterfaceMap(input map[string]interface{}) []string {
	keys := make([]string, 0, len(input))
	output := make([]string, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, k := range keys {
		var s strings.Builder
		fmt.Fprintf(&s, "%s=%v", k, input[k])
		output[i] = s.String()
	}
	return output
}

func CreatePoint(metricname string, tags map[string]string, fields map[string]interface{}, timestamp int64) string {
	var s strings.Builder
	taglist := SortStringStringMap(tags)
	fieldlist := SortStringInterfaceMap(fields)

	if len(taglist) > 0 {
		fmt.Fprintf(&s, "%s,%s %s %d", metricname, strings.Join(taglist, ","), strings.Join(fieldlist, ","), timestamp)
	} else {
		fmt.Fprintf(&s, "%s %s %d", metricname, strings.Join(fieldlist, ","), timestamp)
	}
	return s.String()
}

func run_cmd(cmd string, cmd_opts string) string {
	//ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	//defer cancel()
	//command := exec.CommandContext(ctx, cmd, strings.Join(cmd_opts, " "))
	command := exec.Command(cmd, cmd_opts)
	command.Wait()
	//select {
	//  case <-time.After(101 * time.Second):
	//      fmt.Println("overslept")
	//  case <-ctx.Done():
	//      fmt.Println(ctx.Err()) // prints "context deadline exceeded"
	//}

	stdout, err := command.Output()

	if err != nil {
		fmt.Println(err.Error())
		return ""
	}
	return (string(stdout))
}

func GetSingleCollector(folders *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}
		if info.IsDir() {
			configfile := filepath.Join(path, "config.json")
			if _, err := os.Stat(configfile); err == nil {
				*folders = append(*folders, path)
			}
		}
		return nil
	}
}

func GetCollectorFolders(root string, folders *[]string) error {

	err := filepath.Walk(root, GetSingleCollector(folders))
	if err != nil {
		panic(err)
	}
	return err
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

	var folders []string
	GetCollectorFolders(config.CollectorPath, &folders)

	for _, path := range folders {
		var col_config CollectorConfig
		LoadCollectorConfiguration(filepath.Join(path, "config.json"), &col_config)
		stdout := run_cmd(filepath.Join(path, col_config.Command), col_config.Args)

		metrics := strings.Split(stdout, "\n")
		for _, m := range metrics {
			if len(m) > 0 {
				t := strings.Fields(m)
				if len(t) == 2 {
					var s strings.Builder
					fmt.Fprintf(&s, "%s %d", m, time.Now().UnixNano())
					m = s.String()
				}
				fmt.Println("SEND", m)
			}
		}
	}
}
