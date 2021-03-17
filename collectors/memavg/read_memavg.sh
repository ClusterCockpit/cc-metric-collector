#!/bin/bash


TOTAL=$(grep "MemTotal" /proc/meminfo | awk '{print $2}')
AVAIL=$(grep "MemAvailable" /proc/meminfo | awk '{print $2}')
FREE=$(grep "MemFree" /proc/meminfo | awk '{print $2}')
HOST=$(hostname -s)


echo "mem_total,host=$HOST $TOTAL"
echo "mem_avail,host=$HOST $AVAIL"
echo "mem_free,host=$HOST $FREE"
