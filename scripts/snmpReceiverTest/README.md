# snmpReceiverTest

This script is a basic implementation of how the SNMPReceiver to test the connection before configuring
the collector to get the data periodically.

It does not support the specification of the `type`, `type-id`, `stype` and `stype-id` but since they are
not required to test the functionality, they are left out.

## Usage

```sh
$ go run snmpReceiverTest -h
Usage of snmpReceiverTest:
  -community string
    	SNMP community (default "public")
  -hostname string
    	Hostname (default "127.0.0.1")
  -name string
    	Name of metric or OID
  -port string
    	Port number (default "161")
  -timeout string
    	Timeout for SNMP request (default "1s")
  -unit string
    	Unit of metric or OID
  -value string
    	Value OID
  -version string
    	SNMP version (default "2c")
```

## Example

```sh
$ go run scripts/snmpReceiverTest/snmpReceiverTest.go -name serialNumber -value .1.3.6.1.4.1.6574.1.5.2.0 -hostname $IP -community $COMMUNITY
Name: serialNumber, Tags: map[type:node], Meta: map[], fields: map[value:18B0PCNXXXXX], Timestamp: 1702050709599311288
```