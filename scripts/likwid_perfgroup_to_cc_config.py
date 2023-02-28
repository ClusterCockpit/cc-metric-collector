#!/usr/bin/env python3

import os, os.path, sys, getopt, re, json

def which(cmd):
    ospath = os.environ.get("PATH", "")
    for p in ospath.split(":"):
        testcmd = os.path.join(p, cmd)
        if os.access(testcmd, os.X_OK):
            return testcmd
    return None

def group_to_json(groupfile):
    gdata = []
    with open(groupfile, "r") as fp:
        gdata = fp.read().strip().split("\n")
    events = {}
    metrics = []
    parse_events = False
    parse_metrics = False
    for line in gdata:
        if line == "EVENTSET":
            parse_events = True
            parse_metrics = False
            continue
        if line == "METRICS":
            parse_events = False
            parse_metrics = True
            continue
        if len(line) == 0 or line.startswith("SHORT") or line == "LONG":
            parse_events = False
            parse_metrics = False
            continue
        if parse_events:
            m = re.match("([\w\d]+)\s+([\w\d_]+)", line)
            if m:
                events[m.group(1)] = m.group(2)
        if parse_metrics:
            llist = re.split("\s+", line)
            calc = llist[-1]
            metric = " ".join(llist[:-1])
            scope = "hwthread"
            if "BOX" in calc:
                scope = "socket"
            if "PWR" in calc:
                scope = "socket"

            m = {"name" : metric, "calc": calc, "type" : scope, "publish" : True}
            metrics.append(m)
    return {"events" : events, "metrics" : metrics}

if len(sys.argv) != 3:
    print("Usage: $0 <likwid-arch> <group-name>")
    sys.exit(1)


arch = sys.argv[1]
group = sys.argv[2]

ltopo = which("likwid-topology")
if not ltopo:
    print("Cannot find LIKWID installation. Please add LIKWID bin folder to your PATH.")
    sys.exit(1)

bindir = os.path.dirname(ltopo)

groupdir = os.path.normpath(os.path.join(bindir, "../share/likwid/perfgroups"))
if not os.path.exists(groupdir):
    print("Cannot find LIKWID performance groups in default install location")
    sys.exit(1)

archdir = os.path.join(groupdir, arch)
if not os.path.exists(archdir):
    print("Cannot find LIKWID performance groups for architecture {}".format(arch))
    sys.exit(1)

groupfile = os.path.join(archdir, "{}.txt".format(group))
if not os.path.exists(groupfile):
    print("Cannot find LIKWID performance group {} for architecture {}".format(group, arch))
    sys.exit(1)

gdata = group_to_json(groupfile)
print(json.dumps(gdata, sort_keys=True, indent=2))
