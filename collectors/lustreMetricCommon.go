package collectors

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const LUSTRE_SYSFS = `/sys/fs/lustre`
const LCTL_CMD = `lctl`
const LCTL_OPTION = `get_param`

type LustreMetricDefinition struct {
	name       string
	lineprefix string
	lineoffset int
	offsetname string
	unit       string
	calc       string
}

type LustreMetricData struct {
	sample_time       int64
	start_time        int64
	elapsed_time      int64
	op_data           map[string]map[string]int64
	op_units          map[string]string
	sample_time_unit  string
	start_time_unit   string
	elapsed_time_unit string
}

var devicePattern = regexp.MustCompile(`^[\w\d\-_]+\.([\w\d\-_]+)\.[\w\d\-_]+=$`)
var jobPattern = regexp.MustCompile(`^-\s*job_id:\s*([\w\d\-_\.:]+)$`)
var snapshotPattern = regexp.MustCompile(`^\s*snapshot_time\s*:\s*([\d\.]+)\s*([\w\d\-_\.]*)$`)
var startPattern = regexp.MustCompile(`^\s*start_time\s*:\s*([\d\.]+)\s*([\w\d\-_\.]*)$`)
var elapsedPattern = regexp.MustCompile(`^\s*elapsed_time\s*:\s*([\d\.]+)\s*([\w\d\-_\.]*)$`)
var linePattern = regexp.MustCompile(`^\s*([\w\d\-_\.]+):\s*\{\s*samples:\s*([\d\.]+),\s*unit:\s*([\w\d\-_\.]+),\s*min:\s*([\d\.]+),\s*max:\s*([\d\.]+),\s*sum:\s*([\d\.]+),\s*sumsq:\s*([\d\.]+)\s*\}`)

func executeLustreCommand(sudo, lctl, option, search string, use_sudo bool) []string {
	var command *exec.Cmd
	if use_sudo {
		command = exec.Command(sudo, lctl, option, search)
	} else {
		command = exec.Command(lctl, option, search)
	}
	command.Wait()
	stdout, _ := command.Output()
	return strings.Split(string(stdout), "\n")
}

func splitTree(lines []string, splitRegex *regexp.Regexp) map[string][]string {
	entries := make(map[string][]string)
	ent_lines := make([]int, 0)
	for i, l := range lines {
		m := splitRegex.FindStringSubmatch(l)
		if len(m) == 2 {
			ent_lines = append(ent_lines, i)
		}
	}
	if len(ent_lines) > 0 {
		for i, idx := range ent_lines[:len(ent_lines)-1] {
			m := splitRegex.FindStringSubmatch(lines[idx])
			entries[m[1]] = lines[idx+1 : ent_lines[i+1]]
		}
		last := ent_lines[len(ent_lines)-1]
		m := splitRegex.FindStringSubmatch(lines[last])
		entries[m[1]] = lines[last:]
	}
	return entries
}

func readDevices(lines []string) map[string][]string {
	return splitTree(lines, devicePattern)
}

func readJobs(lines []string) map[string][]string {
	return splitTree(lines, jobPattern)
}

func readJobdata(lines []string) LustreMetricData {

	jobdata := LustreMetricData{
		op_data:           make(map[string]map[string]int64),
		op_units:          make(map[string]string),
		sample_time:       0,
		sample_time_unit:  "nsec",
		start_time:        0,
		start_time_unit:   "nsec",
		elapsed_time:      0,
		elapsed_time_unit: "nsec",
	}
	parseTime := func(value, unit string) int64 {
		var t int64 = 0
		if len(unit) == 0 {
			unit = "secs"
		}
		values := strings.Split(value, ".")
		units := strings.Split(unit, ".")
		if len(values) != len(units) {
			fmt.Printf("Invalid time specification '%s' and '%s'\n", value, unit)
		}
		for i, v := range values {
			if len(units) > i {
				s, err := strconv.ParseInt(v, 10, 64)
				if err == nil {
					switch units[i] {
					case "secs":
						t += s * 1e9
					case "msecs":
						t += s * 1e6
					case "usecs":
						t += s * 1e3
					case "nsecs":
						t += s
					}
				}
			}
		}
		return t
	}
	parseNumber := func(value string) int64 {
		s, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return s
		}
		return 0
	}
	for _, l := range lines {
		if jobdata.sample_time == 0 {
			m := snapshotPattern.FindStringSubmatch(l)
			if len(m) == 3 {
				if len(m[2]) > 0 {
					jobdata.sample_time = parseTime(m[1], m[2])
				} else {
					jobdata.sample_time = parseTime(m[1], "secs")
				}
			}
		}

		if jobdata.start_time == 0 {
			m := startPattern.FindStringSubmatch(l)
			if len(m) == 3 {
				if len(m[2]) > 0 {
					jobdata.start_time = parseTime(m[1], m[2])
				} else {
					jobdata.start_time = parseTime(m[1], "secs")
				}
			}
		}
		if jobdata.elapsed_time == 0 {
			m := elapsedPattern.FindStringSubmatch(l)
			if len(m) == 3 {
				if len(m[2]) > 0 {
					jobdata.elapsed_time = parseTime(m[1], m[2])
				} else {
					jobdata.elapsed_time = parseTime(m[1], "secs")
				}
			}
		}
		m := linePattern.FindStringSubmatch(l)
		if len(m) == 8 {
			jobdata.op_units[m[1]] = m[3]
			jobdata.op_data[m[1]] = map[string]int64{
				"samples": parseNumber(m[2]),
				"min":     parseNumber(m[4]),
				"max":     parseNumber(m[5]),
				"sum":     parseNumber(m[6]),
				"sumsq":   parseNumber(m[7]),
			}
		}
	}
	return jobdata
}

func readCommandOutput(lines []string) map[string]map[string]LustreMetricData {
	var data map[string]map[string]LustreMetricData = make(map[string]map[string]LustreMetricData)
	devs := readDevices(lines)
	for d, ddata := range devs {
		data[d] = make(map[string]LustreMetricData)
		jobs := readJobs(ddata)
		for j, jdata := range jobs {
			x := readJobdata(jdata)
			data[d][j] = x
		}
	}
	return data
}
