package receivers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	cclog "github.com/ClusterCockpit/cc-metric-collector/pkg/ccLogger"
	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
)

type IPMIReceiverClientConfig struct {

	// Hostname the IPMI service belongs to
	Protocol         string            // Protocol / tool to use for IPMI sensor reading
	DriverType       string            // Out of band IPMI driver
	Fanout           int               // Maximum number of simultaneous IPMI connections
	NumHosts         int               // Number of remote IPMI devices with the same configuration
	IPMIHosts        string            // List of remote IPMI devices to communicate with
	IPMI2HostMapping map[string]string // Mapping between IPMI device name and host name
	Username         string            // User name to authenticate with
	Password         string            // Password to use for authentication
	CLIOptions       []string          // Additional command line options for ipmi-sensors
	isExcluded       map[string]bool   // is metric excluded
}

type IPMIReceiver struct {
	receiver
	config struct {
		Interval time.Duration

		// Client config for each IPMI hosts
		ClientConfigs []IPMIReceiverClientConfig
	}

	// Storage for static information
	meta map[string]string

	done chan bool      // channel to finish / stop IPMI receiver
	wg   sync.WaitGroup // wait group for IPMI receiver
}

// doReadMetrics reads metrics from all configure IPMI hosts.
func (r *IPMIReceiver) doReadMetric() {
	for i := range r.config.ClientConfigs {
		clientConfig := &r.config.ClientConfigs[i]
		var cmd_options []string
		if clientConfig.Protocol == "ipmi-sensors" {
			cmd_options = append(cmd_options,
				"--always-prefix",
				"--sdr-cache-recreate",
				// Attempt to interpret OEM data, such as event data, sensor readings, or general extra info
				"--interpret-oem-data",
				// Ignore not-available (i.e. N/A) sensors in output
				"--ignore-not-available-sensors",
				// Ignore unrecognized sensor events
				"--ignore-unrecognized-events",
				// Output fields in comma separated format
				"--comma-separated-output",
				// Do not output column headers
				"--no-header-output",
				// Output non-abbreviated units (e.g. 'Amps' instead of 'A').
				// May aid in disambiguation of units (e.g. 'C' for Celsius or Coulombs).
				"--non-abbreviated-units",
				"--fanout", fmt.Sprint(clientConfig.Fanout),
				"--driver-type", clientConfig.DriverType,
				"--hostname", clientConfig.IPMIHosts,
				"--username", clientConfig.Username,
				"--password", clientConfig.Password,
			)
			cmd_options := append(cmd_options, clientConfig.CLIOptions...)

			command := exec.Command("ipmi-sensors", cmd_options...)
			stdout, _ := command.StdoutPipe()
			errBuf := new(bytes.Buffer)
			command.Stderr = errBuf

			// start command
			if err := command.Start(); err != nil {
				cclog.ComponentError(
					r.name,
					fmt.Sprintf("doReadMetric(): Failed to start command \"%s\": %v", command.String(), err),
				)
				continue
			}

			// Read command output
			const (
				idxID = iota
				idxName
				idxType
				idxReading
				idxUnits
				idxEvent
			)
			numPrefixRegex := regexp.MustCompile("^[[:digit:]][[:digit:]]-(.*)$")
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				// Read host
				v1 := strings.Split(scanner.Text(), ": ")
				if len(v1) != 2 {
					continue
				}
				host, ok := clientConfig.IPMI2HostMapping[v1[0]]
				if !ok {
					continue
				}

				// Read sensors
				v2 := strings.Split(v1[1], ",")
				if len(v2) != 6 {
					continue
				}
				// Skip sensors with non available sensor readings
				if v2[idxReading] == "N/A" {
					continue
				}

				metric := strings.ToLower(v2[idxType])
				name := strings.ToLower(
					strings.Replace(
						strings.TrimSpace(
							v2[idxName]), " ", "_", -1))
				// remove prefix enumeration like 01-...
				if v := numPrefixRegex.FindStringSubmatch(name); v != nil {
					name = v[1]
				}
				unit := v2[idxUnits]
				if unit == "Watts" {

					// Power
					metric = "power"
					name = strings.TrimSuffix(name, "_power")
					name = strings.TrimSuffix(name, "_pwr")
					name = strings.TrimPrefix(name, "pwr_")
				} else if metric == "voltage" &&
					unit == "Volts" {

					// Voltage
					name = strings.TrimPrefix(name, "volt_")
				} else if metric == "current" &&
					unit == "Amps" {

					// Current
					unit = "Ampere"
				} else if metric == "temperature" &&
					unit == "degrees C" {

					// Temperature
					name = strings.TrimSuffix(name, "_temp")
					unit = "degC"
				} else if metric == "temperature" &&
					unit == "degrees F" {

					// Temperature
					name = strings.TrimSuffix(name, "_temp")
					unit = "degF"
				} else if metric == "fan" && unit == "RPM" {

					// Fan speed
					metric = "fan_speed"
					name = strings.TrimSuffix(name, "_tach")
					name = strings.TrimPrefix(name, "spd_")
				} else if (metric == "cooling device" ||
					metric == "other units based sensor") &&
					name == "system_air_flow" &&
					unit == "CFM" {

					// Air flow
					metric = "air_flow"
					name = strings.TrimSuffix(name, "_air_flow")
					unit = "CubicFeetPerMinute"
				} else if (metric == "processor" ||
					metric == "other units based sensor") &&
					(name == "cpu_utilization" ||
						name == "io_utilization" ||
						name == "mem_utilization" ||
						name == "sys_utilization") &&
					(unit == "unspecified" ||
						unit == "%") {

					// Utilization
					metric = "utilization"
					name = strings.TrimSuffix(name, "_utilization")
					unit = "percent"
				} else {
					if false {
						// Debug output for unprocessed metrics
						fmt.Printf(
							"host: '%s', metric: '%s', name: '%s', unit: '%s'\n",
							host, metric, name, unit)
					}
					continue
				}

				// Skip excluded metrics
				if clientConfig.isExcluded[metric] {
					continue
				}

				// Parse sensor value
				value, err := strconv.ParseFloat(v2[idxReading], 64)
				if err != nil {
					continue
				}

				y, err := lp.New(
					metric,
					map[string]string{
						"hostname": host,
						"type":     "node",
						"name":     name,
					},
					map[string]string{
						"source": r.name,
						"group":  "IPMI",
						"unit":   unit,
					},
					map[string]interface{}{
						"value": value,
					},
					time.Now())
				if err == nil {
					r.sink <- y
				}
			}

			// Wait for command end
			if err := command.Wait(); err != nil {
				errMsg, _ := io.ReadAll(errBuf)
				cclog.ComponentError(
					r.name,
					fmt.Sprintf("doReadMetric(): Failed to wait for the end of command \"%s\": %v\n",
						strings.Replace(command.String(), clientConfig.Password, "<PW>", -1), err),
					fmt.Sprintf("doReadMetric(): command stderr: \"%s\"\n", string(errMsg)),
				)
			}
		}
	}
}

func (r *IPMIReceiver) Start() {
	cclog.ComponentDebug(r.name, "START")

	// Start IPMI receiver
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		// Create ticker
		ticker := time.NewTicker(r.config.Interval)
		defer ticker.Stop()

		for {
			r.doReadMetric()

			select {
			case tickerTime := <-ticker.C:
				// Check if we missed the ticker event
				if since := time.Since(tickerTime); since > 5*time.Second {
					cclog.ComponentInfo(r.name, "Missed ticker event for more then", since)
				}

				// process ticker event -> continue
				continue
			case <-r.done:
				// process done event
				return
			}
		}
	}()

	cclog.ComponentDebug(r.name, "STARTED")
}

// Close receiver: close network connection, close files, close libraries, ...
func (r *IPMIReceiver) Close() {
	cclog.ComponentDebug(r.name, "CLOSE")

	// Send the signal and wait
	close(r.done)
	r.wg.Wait()

	cclog.ComponentDebug(r.name, "DONE")
}

// NewIPMIReceiver creates a new instance of the redfish receiver
// Initialize the receiver by giving it a name and reading in the config JSON
func NewIPMIReceiver(name string, config json.RawMessage) (Receiver, error) {
	r := new(IPMIReceiver)

	// Config options from config file
	configJSON := struct {
		Type string `json:"type"`

		// How often the IPMI sensor metrics should be read and send to the sink (default: 30 s)
		IntervalString string `json:"interval,omitempty"`

		// Maximum number of simultaneous IPMI connections (default: 64)
		Fanout int `json:"fanout,omitempty"`

		// Out of band IPMI driver (default: LAN_2_0)
		DriverType string `json:"driver_type,omitempty"`

		// Default client username, password and endpoint
		Username *string `json:"username"` // User name to authenticate with
		Password *string `json:"password"` // Password to use for authentication
		Endpoint *string `json:"endpoint"` // URL of the IPMI device

		// Globally excluded metrics
		ExcludeMetrics []string `json:"exclude_metrics,omitempty"`

		ClientConfigs []struct {
			Fanout     int      `json:"fanout,omitempty"`      // Maximum number of simultaneous IPMI connections (default: 64)
			DriverType string   `json:"driver_type,omitempty"` // Out of band IPMI driver (default: LAN_2_0)
			HostList   []string `json:"host_list"`             // List of hosts with the same client configuration
			Username   *string  `json:"username"`              // User name to authenticate with
			Password   *string  `json:"password"`              // Password to use for authentication
			Endpoint   *string  `json:"endpoint"`              // URL of the IPMI service

			// Per client excluded metrics
			ExcludeMetrics []string `json:"exclude_metrics,omitempty"`

			// Additional command line options for ipmi-sensors
			CLIOptions []string `json:"cli_options,omitempty"`
		} `json:"client_config"`
	}{
		// Set defaults values
		// Allow overwriting these defaults by reading config JSON
		Fanout:         64,
		DriverType:     "LAN_2_0",
		IntervalString: "30s",
	}

	// Set name of IPMIReceiver
	r.name = fmt.Sprintf("IPMIReceiver(%s)", name)

	// Create done channel
	r.done = make(chan bool)

	// Set static information
	r.meta = map[string]string{"source": r.name}

	// Read the IPMI receiver specific JSON config
	if len(config) > 0 {
		err := json.Unmarshal(config, &configJSON)
		if err != nil {
			cclog.ComponentError(r.name, "Error reading config:", err.Error())
			return nil, err
		}
	}

	// Convert interval string representation to duration
	var err error
	r.config.Interval, err = time.ParseDuration(configJSON.IntervalString)
	if err != nil {
		err := fmt.Errorf(
			"Failed to parse duration string interval='%s': %w",
			configJSON.IntervalString,
			err,
		)
		cclog.Error(r.name, err)
		return nil, err
	}

	// Create client config from JSON config
	totalNumHosts := 0
	for i := range configJSON.ClientConfigs {
		clientConfigJSON := &configJSON.ClientConfigs[i]

		var endpoint string
		if clientConfigJSON.Endpoint != nil {
			endpoint = *clientConfigJSON.Endpoint
		} else if configJSON.Endpoint != nil {
			endpoint = *configJSON.Endpoint
		} else {
			err := fmt.Errorf("client config number %v requires endpoint", i)
			cclog.ComponentError(r.name, err)
			return nil, err
		}

		fanout := configJSON.Fanout
		if clientConfigJSON.Fanout != 0 {
			fanout = clientConfigJSON.Fanout
		}

		driverType := configJSON.DriverType
		if clientConfigJSON.DriverType != "" {
			driverType = clientConfigJSON.DriverType
		}
		if driverType != "LAN" && driverType != "LAN_2_0" {
			err := fmt.Errorf("client config number %v has invalid driver type %s", i, driverType)
			cclog.ComponentError(r.name, err)
			return nil, err
		}

		var protocol string
		var host_pattern string
		if e := strings.Split(endpoint, "://"); len(e) == 2 {
			protocol = e[0]
			host_pattern = e[1]
		} else {
			err := fmt.Errorf("client config number %v has invalid endpoint %s", i, endpoint)
			cclog.ComponentError(r.name, err)
			return nil, err
		}

		var username string
		if clientConfigJSON.Username != nil {
			username = *clientConfigJSON.Username
		} else if configJSON.Username != nil {
			username = *configJSON.Username
		} else {
			err := fmt.Errorf("client config number %v requires username", i)
			cclog.ComponentError(r.name, err)
			return nil, err
		}

		var password string
		if clientConfigJSON.Password != nil {
			password = *clientConfigJSON.Password
		} else if configJSON.Password != nil {
			password = *configJSON.Password
		} else {
			err := fmt.Errorf("client config number %v requires password", i)
			cclog.ComponentError(r.name, err)
			return nil, err
		}

		// Create mapping between ipmi hostname and node hostname
		// This also guaranties that all ipmi hostnames are uniqu
		ipmi2HostMapping := make(map[string]string)
		for _, host := range clientConfigJSON.HostList {
			ipmiHost := strings.Replace(host_pattern, "%h", host, -1)
			ipmi2HostMapping[ipmiHost] = host
		}

		numHosts := len(ipmi2HostMapping)
		totalNumHosts += numHosts
		ipmiHostList := make([]string, 0, numHosts)
		for ipmiHost := range ipmi2HostMapping {
			ipmiHostList = append(ipmiHostList, ipmiHost)
		}

		// Additional command line options
		for _, v := range clientConfigJSON.CLIOptions {
			switch {
			case v == "-u" || strings.HasPrefix(v, "--username"):
				err := fmt.Errorf("client config number %v: do not set username in cli_options. Use json config username instead", i)
				cclog.ComponentError(r.name, err)
				return nil, err
			case v == "-p" || strings.HasPrefix(v, "--password"):
				err := fmt.Errorf("client config number %v: do not set password in cli_options. Use json config password instead", i)
				cclog.ComponentError(r.name, err)
				return nil, err
			case v == "-h" || strings.HasPrefix(v, "--hostname"):
				err := fmt.Errorf("client config number %v: do not set hostname in cli_options. Use json config host_list instead", i)
				cclog.ComponentError(r.name, err)
				return nil, err
			case v == "-D" || strings.HasPrefix(v, "--driver-type"):
				err := fmt.Errorf("client config number %v: do not set driver type in cli_options. Use json config driver_type instead", i)
				cclog.ComponentError(r.name, err)
				return nil, err
			case v == "-F" || strings.HasPrefix(v, " --fanout"):
				err := fmt.Errorf("client config number %v: do not set fanout in cli_options. Use json config fanout instead", i)
				cclog.ComponentError(r.name, err)
				return nil, err
			case v == "--always-prefix" ||
				v == "--sdr-cache-recreate" ||
				v == "--interpret-oem-data" ||
				v == "--ignore-not-available-sensors" ||
				v == "--ignore-unrecognized-events" ||
				v == "--comma-separated-output" ||
				v == "--no-header-output" ||
				v == "--non-abbreviated-units":
				err := fmt.Errorf("client config number %v: Do not use option %s in cli_options, it is used internally", i, v)
				cclog.ComponentError(r.name, err)
				return nil, err
			}
		}
		cliOptions := make([]string, 0)
		cliOptions = append(cliOptions, clientConfigJSON.CLIOptions...)

		// Is metrics excluded globally or per client
		isExcluded := make(map[string]bool)
		for _, key := range clientConfigJSON.ExcludeMetrics {
			isExcluded[key] = true
		}
		for _, key := range configJSON.ExcludeMetrics {
			isExcluded[key] = true
		}

		r.config.ClientConfigs = append(
			r.config.ClientConfigs,
			IPMIReceiverClientConfig{
				Protocol:         protocol,
				Fanout:           fanout,
				DriverType:       driverType,
				NumHosts:         numHosts,
				IPMIHosts:        strings.Join(ipmiHostList, ","),
				IPMI2HostMapping: ipmi2HostMapping,
				Username:         username,
				Password:         password,
				CLIOptions:       cliOptions,
				isExcluded:       isExcluded,
			})
	}

	if totalNumHosts == 0 {
		err := fmt.Errorf("at least one IPMI host config is required")
		cclog.ComponentError(r.name, err)
		return nil, err
	}

	cclog.ComponentInfo(r.name, "monitoring", totalNumHosts, "IPMI hosts")
	return r, nil
}
