package collectors

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-energy-manager/pkg/cc-message"
)

const DEFAULT_BEEGFS_CMD = "beegfs-ctl"

// Struct for the collector-specific JSON config
type BeegfsMetaCollectorConfig struct {
	Beegfs            string   `json:"beegfs_path"`
	ExcludeMetrics    []string `json:"exclude_metrics,omitempty"`
	ExcludeFilesystem []string `json:"exclude_filesystem"`
}

type BeegfsMetaCollector struct {
	metricCollector
	tags    map[string]string
	matches map[string]string
	config  BeegfsMetaCollectorConfig
	skipFS  map[string]struct{}
}

func (m *BeegfsMetaCollector) Init(config json.RawMessage) error {
	// Check if already initialized
	if m.init {
		return nil
	}
	// Metrics
	var nodeMdstat_array = [39]string{
		"sum", "ack", "close", "entInf",
		"fndOwn", "mkdir", "create", "rddir",
		"refrEn", "mdsInf", "rmdir", "rmLnk",
		"mvDirIns", "mvFiIns", "open", "ren",
		"sChDrct", "sAttr", "sDirPat", "stat",
		"statfs", "trunc", "symlnk", "unlnk",
		"lookLI", "statLI", "revalLI", "openLI",
		"createLI", "hardlnk", "flckAp", "flckEn",
		"flckRg", "dirparent", "listXA", "getXA",
		"rmXA", "setXA", "mirror"}

	m.name = "BeegfsMetaCollector"
	m.setup()
	m.parallel = true
	// Set default beegfs-ctl binary

	m.config.Beegfs = DEFAULT_BEEGFS_CMD

	// Read JSON configuration
	if len(config) > 0 {
		err := json.Unmarshal(config, &m.config)
		if err != nil {
			return err
		}
	}

	//create map with possible variables
	m.matches = make(map[string]string)
	for _, value := range nodeMdstat_array {
		_, skip := stringArrayContains(m.config.ExcludeMetrics, value)
		if skip {
			m.matches["other"] = "0"
		} else {
			m.matches["beegfs_cmeta_"+value] = "0"
		}
	}

	m.meta = map[string]string{
		"source": m.name,
		"group":  "BeegfsMeta",
	}
	m.tags = map[string]string{
		"type":       "node",
		"filesystem": "",
	}
	m.skipFS = make(map[string]struct{})
	for _, fs := range m.config.ExcludeFilesystem {
		m.skipFS[fs] = struct{}{}
	}

	// Beegfs file system statistics can only be queried by user root
	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("BeegfsMetaCollector.Init(): Failed to get current user: %v", err)
	}
	if user.Uid != "0" {
		return fmt.Errorf("BeegfsMetaCollector.Init(): BeeGFS file system statistics can only be queried by user root")
	}

	// Check if beegfs-ctl is in executable search path
	_, err = exec.LookPath(m.config.Beegfs)
	if err != nil {
		return fmt.Errorf("BeegfsMetaCollector.Init(): Failed to find beegfs-ctl binary '%s': %v", m.config.Beegfs, err)
	}
	m.init = true
	return nil
}

func (m *BeegfsMetaCollector) Read(interval time.Duration, output chan lp.CCMessage) {
	if !m.init {
		return
	}
	//get mounpoint
	buffer, _ := os.ReadFile(string("/proc/mounts"))
	mounts := strings.Split(string(buffer), "\n")
	var mountpoints []string
	for _, line := range mounts {
		if len(line) == 0 {
			continue
		}
		f := strings.Fields(line)
		if strings.Contains(f[0], "beegfs_ondemand") {
			// Skip excluded filesystems
			if _, skip := m.skipFS[f[1]]; skip {
				continue
			}
			mountpoints = append(mountpoints, f[1])
		}
	}

	if len(mountpoints) == 0 {
		return
	}

	for _, mountpoint := range mountpoints {
		m.tags["filesystem"] = mountpoint

		// bwwgfs-ctl:
		// --clientstats: Show client IO statistics.
		// --nodetype=meta: The node type to query (meta, storage).
		// --interval:
		// --mount=/mnt/beeond/: Which mount point
		//cmd := exec.Command(m.config.Beegfs, "/root/mc/test.txt")
		mountoption := "--mount=" + mountpoint
		cmd := exec.Command(m.config.Beegfs, "--clientstats",
			"--nodetype=meta", mountoption, "--allstats")
		cmd.Stdin = strings.NewReader("\n")
		cmdStdout := new(bytes.Buffer)
		cmdStderr := new(bytes.Buffer)
		cmd.Stdout = cmdStdout
		cmd.Stderr = cmdStderr
		err := cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "BeegfsMetaCollector.Read(): Failed to execute command \"%s\": %s\n", cmd.String(), err.Error())
			fmt.Fprintf(os.Stderr, "BeegfsMetaCollector.Read(): command exit code: \"%d\"\n", cmd.ProcessState.ExitCode())
			data, _ := io.ReadAll(cmdStderr)
			fmt.Fprintf(os.Stderr, "BeegfsMetaCollector.Read(): command stderr: \"%s\"\n", string(data))
			data, _ = io.ReadAll(cmdStdout)
			fmt.Fprintf(os.Stderr, "BeegfsMetaCollector.Read(): command stdout: \"%s\"\n", string(data))
			return
		}
		// Read I/O statistics
		scanner := bufio.NewScanner(cmdStdout)

		sumLine := regexp.MustCompile(`^Sum:\s+\d+\s+\[[a-zA-Z]+\]+`)
		//Line := regexp.MustCompile(`^(.*)\s+(\d)+\s+\[([a-zA-Z]+)\]+`)
		statsLine := regexp.MustCompile(`^(.*?)\s+?(\d.*?)$`)
		singleSpacePattern := regexp.MustCompile(`\s+`)
		removePattern := regexp.MustCompile(`[\[|\]]`)

		for scanner.Scan() {
			readLine := scanner.Text()
			//fmt.Println(readLine)
			// Jump few lines, we only want the I/O stats from nodes
			if !sumLine.MatchString(readLine) {
				continue
			}

			match := statsLine.FindStringSubmatch(readLine)
			// nodeName = "Sum:" or would be nodes
			// nodeName := match[1]
			//Remove multiple whitespaces
			dummy := removePattern.ReplaceAllString(match[2], " ")
			metaStats := strings.TrimSpace(singleSpacePattern.ReplaceAllString(dummy, " "))
			split := strings.Split(metaStats, " ")

			// fill map with values
			// split[i+1] = mdname
			// split[i] = amount of md operations
			for i := 0; i <= len(split)-1; i += 2 {
				if _, ok := m.matches[split[i+1]]; ok {
					m.matches["beegfs_cmeta_"+split[i+1]] = split[i]
				} else {
					f1, err := strconv.ParseFloat(m.matches["other"], 32)
					if err != nil {
						cclog.ComponentError(
							m.name,
							fmt.Sprintf("Metric (other): Failed to convert str written '%s' to float: %v", m.matches["other"], err))
						continue
					}
					f2, err := strconv.ParseFloat(split[i], 32)
					if err != nil {
						cclog.ComponentError(
							m.name,
							fmt.Sprintf("Metric (other): Failed to convert str written '%s' to float: %v", m.matches["other"], err))
						continue
					}
					//mdStat["other"] = fmt.Sprintf("%f", f1+f2)
					m.matches["beegfs_cstorage_other"] = fmt.Sprintf("%f", f1+f2)
				}
			}

			for key, data := range m.matches {
				value, _ := strconv.ParseFloat(data, 32)
				y, err := lp.NewMessage(key, m.tags, m.meta, map[string]interface{}{"value": value}, time.Now())
				if err == nil {
					output <- y
				}
			}
		}
	}
}

func (m *BeegfsMetaCollector) Close() {
	m.init = false
}
