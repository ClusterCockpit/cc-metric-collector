package collectors

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	lp "github.com/influxdata/line-protocol"
)

type GpfsCollector struct {
	MetricCollector
	config struct {
		Mmpmon string `json:"mmpmon"`
	}
}

func (m *GpfsCollector) Init(config []byte) error {
	var err error
	m.name = "GpfsCollector"
	m.setup()

	// Set default mmpmon binary
	m.config.Mmpmon = "/usr/lpp/mmfs/bin/mmpmon"

	// Read JSON configuration
	if len(config) > 0 {
		err = json.Unmarshal(config, &m.config)
		if err != nil {
			log.Print(err.Error())
			return err
		}
	}

	// GPFS / IBM Spectrum Scale file system statistics can only be queried by user root
	user, err := user.Current()
	if err != nil {
		return fmt.Errorf("GpfsCollector.Init(): Failed to get current user: %v", err)
	}
	if user.Uid != "0" {
		return fmt.Errorf("GpfsCollector.Init(): GPFS file system statistics can only be queried by user root")
	}

	// Check if mmpmon is in executable search path
	_, err = exec.LookPath(m.config.Mmpmon)
	if err != nil {
		return fmt.Errorf("GpfsCollector.Init(): Failed to find mmpmon binary '%s': %v", m.config.Mmpmon, err)
	}

	m.init = true
	return nil
}

func (m *GpfsCollector) Read(interval time.Duration, out *[]lp.MutableMetric) {
	if !m.init {
		return
	}

	// mmpmon:
	// -p: generate output that can be parsed
	// -s: suppress the prompt on input
	// fs_io_s: Displays I/O statistics per mounted file system
	cmd := exec.Command(m.config.Mmpmon, "-p", "-s")
	cmd.Stdin = strings.NewReader("once fs_io_s\n")
	cmdStdout := new(bytes.Buffer)
	cmdStderr := new(bytes.Buffer)
	cmd.Stdout = cmdStdout
	cmd.Stderr = cmdStderr
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): Failed to execute command \"%s\": %s\n", cmd.String(), err.Error())
		fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): command exit code: \"%d\"\n", cmd.ProcessState.ExitCode())
		data, _ := ioutil.ReadAll(cmdStderr)
		fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): command stderr: \"%s\"\n", string(data))
		data, _ = ioutil.ReadAll(cmdStdout)
		fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): command stdout: \"%s\"\n", string(data))
		return
	}

	// Read I/O statistics
	scanner := bufio.NewScanner(cmdStdout)
	for scanner.Scan() {
		lineSplit := strings.Fields(scanner.Text())
		if lineSplit[0] == "_fs_io_s_" {
			key_value := make(map[string]string)
			for i := 1; i < len(lineSplit); i += 2 {
				key_value[lineSplit[i]] = lineSplit[i+1]
			}

			// Ignore keys:
			// _n_:  node IP address,
			// _nn_: node name,
			// _cl_: cluster name,
			// _d_:  number of disks

			filesystem, ok := key_value["_fs_"]
			if !ok {
				fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): Failed to get filesystem name.\n")
				continue
			}

			// return code
			rc, err := strconv.Atoi(key_value["_rc_"])
			if err != nil {
				fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): Failed to convert return code: %s\n", err.Error())
				continue
			}
			if rc != 0 {
				fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): Filesystem %s not ok.", filesystem)
				continue
			}

			/* requires go 1.17
			// unix epoch in microseconds
			timestampInt, err := strconv.ParseInt(key_value["_t_"]+key_value["_tu_"], 10, 64)
			timestamp := time.UnixMicro(timestampInt)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"GpfsCollector.Read(): Failed to convert time stamp '%s': %s\n",
					key_value["_t_"]+key_value["_tu_"], err.Error())
				continue
			}
			*/
			timestamp := time.Now()

			// bytes read
			bytesRead, err := strconv.ParseInt(key_value["_br_"], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"GpfsCollector.Read(): Failed to convert bytes read '%s': %s\n",
					key_value["_br_"], err.Error())
				continue
			}
			y, err := lp.New(
				"gpfs_bytes_read",
				map[string]string{
					"filesystem": filesystem,
				},
				map[string]interface{}{
					"value": bytesRead,
				},
				timestamp)
			if err == nil {
				*out = append(*out, y)
			}

			// bytes written
			bytesWritten, err := strconv.ParseInt(key_value["_bw_"], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"GpfsCollector.Read(): Failed to convert bytes written '%s': %s\n",
					key_value["_bw_"], err.Error())
				continue
			}
			y, err = lp.New(
				"gpfs_bytes_written",
				map[string]string{
					"filesystem": filesystem,
				},
				map[string]interface{}{
					"value": bytesWritten,
				},
				timestamp)
			if err == nil {
				*out = append(*out, y)
			}

			// number of opens
			numOpens, err := strconv.ParseInt(key_value["_oc_"], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"GpfsCollector.Read(): Failed to convert number of opens '%s': %s\n",
					key_value["_oc_"], err.Error())
				continue
			}
			y, err = lp.New(
				"gpfs_num_opens",
				map[string]string{
					"filesystem": filesystem,
				},
				map[string]interface{}{
					"value": numOpens,
				},
				timestamp)
			if err == nil {
				*out = append(*out, y)
			}

			// number of closes
			numCloses, err := strconv.ParseInt(key_value["_cc_"], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): Failed to convert number of closes: %s\n", err.Error())
				continue
			}
			y, err = lp.New(
				"gpfs_num_closes",
				map[string]string{
					"filesystem": filesystem,
				},
				map[string]interface{}{
					"value": numCloses,
				},
				timestamp)
			if err == nil {
				*out = append(*out, y)
			}

			// number of reads
			numReads, err := strconv.ParseInt(key_value["_rdc_"], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): Failed to convert number of reads: %s\n", err.Error())
				continue
			}
			y, err = lp.New(
				"gpfs_num_reads",
				map[string]string{
					"filesystem": filesystem,
				},
				map[string]interface{}{
					"value": numReads,
				},
				timestamp)
			if err == nil {
				*out = append(*out, y)
			}

			// number of writes
			numWrites, err := strconv.ParseInt(key_value["_wc_"], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): Failed to convert number of writes: %s\n", err.Error())
				continue
			}
			y, err = lp.New(
				"gpfs_num_writes",
				map[string]string{
					"filesystem": filesystem,
				},
				map[string]interface{}{
					"value": numWrites,
				},
				timestamp)
			if err == nil {
				*out = append(*out, y)
			}

			// number of read directories
			numReaddirs, err := strconv.ParseInt(key_value["_dir_"], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): Failed to convert number of read directories: %s\n", err.Error())
				continue
			}
			y, err = lp.New(
				"gpfs_num_readdirs",
				map[string]string{
					"filesystem": filesystem,
				},
				map[string]interface{}{
					"value": numReaddirs,
				},
				timestamp)
			if err == nil {
				*out = append(*out, y)
			}

			// Number of inode updates
			numInodeUpdates, err := strconv.ParseInt(key_value["_iu_"], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "GpfsCollector.Read(): Failed to convert Number of inode updates: %s\n", err.Error())
				continue
			}
			y, err = lp.New(
				"gpfs_num_inode_updates",
				map[string]string{
					"filesystem": filesystem,
				},
				map[string]interface{}{
					"value": numInodeUpdates,
				},
				timestamp)
			if err == nil {
				*out = append(*out, y)
			}
		}
	}
}

func (m *GpfsCollector) Close() {
	m.init = false
}
