package main

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	lp "github.com/ClusterCockpit/cc-metric-collector/pkg/ccMetric"
	"github.com/gosnmp/gosnmp"
)

func ReadCLI() map[string]string {
	args := map[string]string{
		"port":      "161",
		"community": "public",
		"version":   "2c",
		"hostname":  "127.0.0.1",
		"timeout":   "1s",
	}

	host_cfg := flag.String("hostname", "127.0.0.1", "Hostname")
	port_cfg := flag.String("port", "161", "Port number")
	comm_cfg := flag.String("community", "public", "SNMP community")
	vers_cfg := flag.String("version", "2c", "SNMP version")
	time_cfg := flag.String("timeout", "1s", "Timeout for SNMP request")

	name_cfg := flag.String("name", "", "Name of metric or OID")
	value_cfg := flag.String("value", "", "Value OID")
	unit_cfg := flag.String("unit", "", "Unit of metric or OID")

	flag.Parse()

	args["port"] = *port_cfg
	args["community"] = *comm_cfg
	args["hostname"] = *host_cfg
	args["version"] = *vers_cfg
	args["timeout"] = *time_cfg

	args["name"] = *name_cfg
	args["value"] = *value_cfg
	args["unit"] = *unit_cfg

	if len(args["name"]) == 0 || len(args["value"]) == 0 {
		fmt.Printf("Required arguments: --name and --value\n")
		flag.Usage()
	}

	return args
}

func validOid(oid string) bool {
	// Regex from https://github.com/BornToBeRoot/NETworkManager/blob/6805740762bf19b95051c7eaa73cf2b4727733c3/Source/NETworkManager.Utilities/RegexHelper.cs#L88
	// Match on leading dot added by Thomas Gruber <thomas.gruber@fau.de>
	match, err := regexp.MatchString(`^[\.]?[012]\.(?:[0-9]|[1-3][0-9])(\.\d+)*$`, oid)
	if err != nil {
		return false
	}
	return match
}

func main() {

	args := ReadCLI()

	if len(args["name"]) == 0 || len(args["value"]) == 0 {
		return
	}

	version := gosnmp.Version2c
	if len(args["version"]) > 0 {
		switch args["version"] {
		case "1":
			version = gosnmp.Version1
		case "2c":
			version = gosnmp.Version2c
		case "3":
			version = gosnmp.Version3
		default:
			fmt.Printf("Invalid SNMP version '%s'\n", args["version"])
			return
		}
	}
	v, err := strconv.ParseInt(args["port"], 10, 16)
	if err != nil {
		fmt.Printf("Failed to parse port number '%s'\n", args["port"])
		return
	}
	port := uint16(v)

	t, err := time.ParseDuration(args["timeout"])
	if err != nil {
		fmt.Printf("Failed to parse timeout '%s'\n", args["timeout"])
		return
	}
	timeout := t

	params := &gosnmp.GoSNMP{
		Target:    args["hostname"],
		Port:      port,
		Community: args["community"],
		Version:   version,
		Timeout:   timeout,
	}
	err = params.Connect()
	if err != nil {
		fmt.Printf("Failed to connect to %s:%d : %v\n", params.Target, params.Port, err.Error())
		return
	}

	oids := make([]string, 0)
	idx := 0
	name := gosnmp.SnmpPDU{
		Value: args["name"],
		Name:  args["name"],
	}
	nameidx := -1
	value := gosnmp.SnmpPDU{
		Value: nil,
		Name:  args["value"],
	}
	valueidx := -1
	unit := gosnmp.SnmpPDU{
		Value: args["unit"],
		Name:  args["unit"],
	}
	unitidx := -1
	if validOid(args["name"]) {
		oids = append(oids, args["name"])
		nameidx = idx
		idx++
	}
	if validOid(args["value"]) {
		oids = append(oids, args["value"])
		valueidx = idx
		idx++
	}
	if len(args["unit"]) > 0 && validOid(args["unit"]) {
		oids = append(oids, args["unit"])
		unitidx = idx
	}
	result, err := params.Get(oids)
	if err != nil {
		fmt.Printf("Failed to get data for OIDs [%s] : %v\n", strings.Join(oids, ", "), err.Error())
		return
	}
	if nameidx >= 0 && len(result.Variables) > nameidx {
		name = result.Variables[nameidx]
	}
	if valueidx >= 0 && len(result.Variables) > valueidx {
		value = result.Variables[valueidx]
	}
	if unitidx >= 0 && len(result.Variables) > unitidx {
		unit = result.Variables[unitidx]
	}
	if value.Value != nil {
		y, err := lp.New(name.Value.(string), map[string]string{"type": "node"}, map[string]string{}, map[string]interface{}{"value": value.Value}, time.Now())
		if err == nil {
			if len(unit.Name) > 0 && unit.Value != nil {
				y.AddMeta("unit", unit.Value.(string))
			}
			fmt.Println(y)
		}
	}
}
