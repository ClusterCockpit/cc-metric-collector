package main
import (
    "strings"
    "io/ioutil"
    "fmt"
    "time"
    "os"
    "strconv"
    )

func main() {
    t := time.Now()
    hostname, err := os.Hostname()
    if err != nil {
        fmt.Println("#", err)
        os.Exit(1)
    }
    hostname = strings.Split(hostname, ".")[0]
    data, err := ioutil.ReadFile("/proc/meminfo")
    if err != nil {
        fmt.Println("#", err)
        os.Exit(1)
        return
    }
    lines := strings.Split(string(data), "\n")
    for _, l := range lines {
        if strings.HasPrefix(l, "MemTotal") {
            f := strings.Fields(l)
            v, err := strconv.ParseInt(f[1], 10, 0)
            if err == nil {
                fmt.Printf("mem_total,hostname=%s value=%v %v\n", hostname, v*1024, t.UnixNano())
            }
        } else if strings.HasPrefix(l, "MemAvailable") {
            f := strings.Fields(l)
            v, err := strconv.ParseInt(f[1], 10, 0)
            if err == nil {
                fmt.Printf("mem_avail,hostname=%s value=%v %v\n", hostname, v*1024, t.UnixNano())
            }
        } else if strings.HasPrefix(l, "MemFree") {
            f := strings.Fields(l)
            v, err := strconv.ParseInt(f[1], 10, 0)
            if err == nil {
                fmt.Printf("mem_free,hostname=%s value=%v %v\n", hostname, v*1024, t.UnixNano())
            }
        }
    }
    return
}
