package main

import (
    "fmt"
    "strings"
    "io/ioutil"
    "os"
    "os/signal"
    "strconv"
    "time"
)


// geht nicht
//enum CollectScope {
//    Node: 0,
//    Socket,
//    Die,
//    LLC,
//    NUMA,
//    Core,
//    HWThread
//}

//var scopeNames = map[CollectScope]string{
//    Node: "Node",
//    Socket: "Socket",
//    Die: "Die",
//    LLC: "LLC",
//    NUMA: "NUMA",
//    Core: "Core",
//    HWThread: "HWThread"
//}

type CollectValue struct {
    Name string
    Value interface{}
    //scope CollectScope
}

type InitFunc func() error
type ReadFunc func(time.Duration) ([]CollectValue, error)
type CloseFunc func() error
type SinkFunc func([]CollectValue) error

func read_memavg(duration time.Duration) ([]CollectValue, error) {
    var values []CollectValue
    data, err := ioutil.ReadFile("/proc/meminfo")
    if err != nil {
        fmt.Println(err.Error())
        return values, err
    }
    var matches = map[string]string {
        "MemTotal" : "mem_total",
        "MemAvailable" : "mem_avail",
        "MemFree" : "mem_free",
    }
    lines := strings.Split(string(data), "\n")
    for _, l := range lines {
        for i,o := range matches {
            if strings.HasPrefix(l, i) {
                f := strings.Fields(l)
                v, err := strconv.ParseInt(f[1], 10, 0)
                if err == nil {
                    var value CollectValue
    //                value.Scope = Node
                    value.Name = o
                    value.Value = v
                    values = append(values, value)
                }
            }
        }
    }
    return values, nil
}

func read_loadavg(duration time.Duration) ([]CollectValue, error) {
    var values []CollectValue
    data, err := ioutil.ReadFile("/proc/loadavg")
    if err != nil {
        fmt.Println(err.Error())
        return values, err
    }
    var matches = map[int]string {
        0 : "loadavg1m",
        1 : "loadavg5m",
        2 : "loadavg15m",
    }
    f := strings.Fields(string(data))
    for i, m := range matches {
        v, err := strconv.ParseFloat(f[i], 64)
        if err == nil {
            var value CollectValue
            value.Name = m
            value.Value = v
    //        value.Scope = Node
            values = append(values, value)
        }
    }
    return values, nil
}

func read_netstat(duration time.Duration) ([]CollectValue, error) {
    var values []CollectValue
    data, err := ioutil.ReadFile("/proc/net/dev")
    if err != nil {
        fmt.Println(err.Error())
        return values, err
    }
    var matches = map[int]string {
        1 : "bytes_in",
        9 : "bytes_out",
        2 : "pkts_in",
        10 : "pkts_out",
    }
    lines := strings.Split(string(data), "\n")
    for _, l := range lines {
        if ! strings.Contains(l, ":") {
            continue
        }
        f := strings.Fields(l)
        dev := f[0][0:len(f[0])-1]
        if dev == "lo" {
            continue
        }
        for i, m := range matches {
            v, err := strconv.ParseInt(f[i], 10, 0)
            if err == nil {
                var value CollectValue
                value.Name = fmt.Sprintf("%s_%s", dev, m)
                value.Value = v
                //value.Scope = Node
                values = append(values, value)
            }
        }
    }
    return values, nil
}

func Send(values []CollectValue) error {
    for _, v := range values {
        fmt.Printf("Name: '%s' Value: '%v'\n", v.Name, v.Value)
    }
    return nil
}

func ReadAll(duration time.Duration, reads []ReadFunc, sink SinkFunc) {
    for _, f := range reads {
        values, err := f(duration)
        if err == nil {
            sink(values)
        }
    }
}

func ReadLoop(interval time.Duration, duration time.Duration, reads []ReadFunc, sink SinkFunc) {
    ticker := time.NewTicker(interval)
    done := make(chan bool)
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, os.Interrupt)
    ReadAll(duration, reads, sink)
    go func() {
        <-sigs
        // Should call all CloseFunc functions here
        os.Exit(1)
    }()
    func() {
        select {
        case <-done:
            return
        case t := <-ticker.C:
            fmt.Println("Tick at", t)
            ReadAll(duration, reads, sink)
        }
    }()
    ticker.Stop()
    done <- true
}

func main() {
    //var inits []InitFunc
    var reads = []ReadFunc {read_memavg, read_loadavg, read_netstat}
    //var closes []CloseFunc
    var duration time.Duration
    var interval time.Duration
    duration = time.Duration(1) * time.Second
    interval = time.Duration(10) * time.Second
    ReadLoop(interval, duration, reads, Send)
    return
}
