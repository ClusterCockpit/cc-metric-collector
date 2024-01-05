#!/bin/bash -l


# Some settings for scripting with less headache

# when a command fails, bash exits instead of continuing
set -o errexit

# make the script fail, when accessing an unset variable
# use "${VARNAME-}" instead of "$VARNAME" when you want to access
# a variable that may or may not have been set
set -o nounset

# ensure that a pipeline command is treated as failed, even if one command in the pipeline fails
set -o pipefail

# enable debug mode, by running your script as TRACE=1 ./script.sh
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi


# Default values for variables
: ${UID=$(id -u)}
: ${VERBOSITY=0}
: ${DELETE=0}
: ${LIST=0}
: ${JOBID="random"}
: ${BASE=./slurmJobDetector-sys-fs-cgroup}

# Print usage if needed
usage()
{
    echo "
Usage: $(basename $0) <opts>
       [ -h | --help ]
       [ -v | --verbosity ]
       [ -u | --uid <UID> (default: ${UID}) ]
       [ -j | --jobid <JOBID> (default: ${JOBID}) ]
       [ -b | --basedir <JOBID> (default: ${BASE}) ]
       [ -d | --delete ]
       [ -l | --list ]
"
    exit $1;
}

cd "$(dirname "$0")"

main() {
    PARSED_ARGUMENTS=$(getopt -a -n $(basename $0) -o hj:u:vb:dl --long help,verbosity,uid:,jobid:,basedir:,delete,list -- "$@")
    VALID_ARGUMENTS=$?
    # Parsing failed
    if [[ "$VALID_ARGUMENTS" != "0" ]]; then
        usage 2
    fi
    # No argument (comment out if command should work without any arguments)
#    if [[ "${PARSED_ARGUMENTS}" == " --" ]]; then
#        usage 0
#    fi
    # Evaluate arguments
    eval set -- "$PARSED_ARGUMENTS"
    while :
    do
      case "$1" in
        -h | --help)  usage 0; shift ;;
        -v | --verbosity)  VERBOSITY=1; shift ;;
        -d | --delete)  DELETE=1; shift ;;
        -l | --list)  LIST=1; shift ;;
        -u | --uid)   UID=$2 ; shift 2 ;;
        -j | --jobid)   JOBID=$2 ; shift 2 ;;
        -b | --basedir)   BASE=$2 ; shift 2 ;;
        --) shift; break ;;
        *) echo "Unexpected option: $1 - this should not happen."
           usage 2;;
      esac
    done

    if [[ ${LIST} -eq 1 ]]; then
        for F in $(ls -d ${BASE}/cpuset/slurm/uid_*/job_*); do
            JOBID=$(echo "$F" | rev | cut -d '/' -f 1 | rev | cut -d '_' -f 2)
            MYUID=$(echo "$F" | rev | cut -d '/' -f 2 | rev | cut -d '_' -f 2)
            echo "UID ${MYUID} JOBID ${JOBID}"
        done
        exit 0
    fi


    if [[ ${JOBID} == "random" ]]; then
        if [[ ${DELETE} -eq 1 ]]; then
            echo "Cannot use random JOBID for deletion"
            exit 1
        else
            JOBID=$RANDOM
        fi
    fi

    FOLDERS="cpuset cpuacct memory devices"

    if [[ ${DELETE} -eq 1 ]]; then
        for F in ${FOLDERS}; do
            rm -r --force "${BASE}/${F}/slurm/uid_${UID}/job_${JOBID}"
        done
    else
        for F in ${FOLDERS}; do
            if [[ $VERBOSITY -eq 1 ]]; then
                echo "${BASE}/${F}/slurm/uid_${UID}/job_${JOBID}"
            fi
            mkdir -p "${BASE}/${F}/slurm/uid_${UID}/job_${JOBID}"
        done

        echo "0-71" > "${BASE}/cpuset/slurm/uid_${UID}/job_${JOBID}/cpuset.effective_cpus"
        echo "0-3" > "${BASE}/cpuset/slurm/uid_${UID}/job_${JOBID}/cpuset.effective_mems"
        
        echo "249036800000" > "${BASE}/memory/slurm/uid_${UID}/job_${JOBID}/memory.limit_in_bytes"
        echo "249036800000" > "${BASE}/memory/slurm/uid_${UID}/job_${JOBID}/memory.soft_limit_in_bytes"
        echo "13987840" >  "${BASE}/memory/slurm/uid_${UID}/job_${JOBID}/memory.usage_in_bytes"
        echo "14966784" >  "${BASE}/memory/slurm/uid_${UID}/job_${JOBID}/memory.max_usage_in_bytes"
        echo "60" >  "${BASE}/memory/slurm/uid_${UID}/job_${JOBID}/memory.swappiness"

        echo "474140369" >  "${BASE}/cpuacct/slurm/uid_${UID}/job_${JOBID}/cpuacct.usage"
        echo "169078878" >  "${BASE}/cpuacct/slurm/uid_${UID}/job_${JOBID}/cpuacct.usage_user"
        echo "315684619" >  "${BASE}/cpuacct/slurm/uid_${UID}/job_${JOBID}/cpuacct.usage_sys"

        echo "a *:* rwm" >  "${BASE}/devices/slurm/uid_${UID}/job_${JOBID}/devices.list"
#memory.numa_stat
#total=0 N0=0 N1=0 N2=0 N3=0
#file=0 N0=0 N1=0 N2=0 N3=0
#anon=0 N0=0 N1=0 N2=0 N3=0
#unevictable=0 N0=0 N1=0 N2=0 N3=0
#hierarchical_total=958 N0=28 N1=579 N2=180 N3=171
#hierarchical_file=194 N0=0 N1=194 N2=0 N3=0
#hierarchical_anon=764 N0=28 N1=385 N2=180 N3=171
#hierarchical_unevictable=0 N0=0 N1=0 N2=0 N3=0

    fi
}

main "$@"

