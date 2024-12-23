#!/bin/bash -l

SRCDIR="$(pwd)"
DESTDIR="$1"

if [ -z "$DESTDIR" ]; then
    echo "Destination folder not provided"
    exit 1
fi


COLLECTORS=$(find "${SRCDIR}/collectors" -name "*Metric.md")
SINKS=$(find "${SRCDIR}/sinks"  -name "*Sink.md")
RECEIVERS=$(find "${SRCDIR}/receivers"  -name "*Receiver.md")



# Collectors
mkdir -p "${DESTDIR}/collectors"
for F in $COLLECTORS; do
    echo "$F"
    FNAME=$(basename "$F")
    TITLE=$(grep -E "^##" "$F" | head -n 1 | sed -e 's+## ++g')
    echo "'${TITLE//\`/}'"
    if [ "${TITLE}" == "" ]; then continue; fi
    rm --force "${DESTDIR}/collectors/${FNAME}"
    cat << EOF >> "${DESTDIR}/collectors/${FNAME}"
---
title: ${TITLE//\`/}
description: >
  Toplevel ${FNAME/.md/}
categories: [cc-metric-collector]
tags: [cc-metric-collector, Collector, ${FNAME/Metric.md/}]
weight: 2
---

EOF
    cat "$F" >> "${DESTDIR}/collectors/${FNAME}"
done

if [ -e "${SRCDIR}/collectors/README.md" ]; then
    cat << EOF > "${DESTDIR}/collectors/_index.md"
---
title: cc-metric-collector's collectors
description: Documentation of cc-metric-collector's collectors
categories: [cc-metric-collector]
tags: [cc-metric-collector, Collector, General]
weight: 40
---

EOF
    cat "${SRCDIR}/collectors/README.md" >> "${DESTDIR}/collectors/_index.md"
fi

# Sinks
mkdir -p "${DESTDIR}/sinks"
for F in $SINKS; do
    echo "$F"
    FNAME=$(basename "$F")
    TITLE=$(grep -E "^##" "$F" | head -n 1 | sed -e 's+## ++g')
    echo "'${TITLE//\`/}'"
    if [ "${TITLE}" == "" ]; then continue; fi
    rm --force "${DESTDIR}/sinks/${FNAME}"
    cat << EOF >> "${DESTDIR}/sinks/${FNAME}"
---
title: ${TITLE//\`/}
description: >
  Toplevel ${FNAME/.md/}
categories: [cc-metric-collector]
tags: [cc-metric-collector, Sink, ${FNAME/Sink.md/}]
weight: 2
---

EOF
    cat "$F" >> "${DESTDIR}/sinks/${FNAME}"
done

if [ -e "${SRCDIR}/collectors/README.md" ]; then
    cat << EOF > "${DESTDIR}/sinks/_index.md"
---
title: cc-metric-collector's sinks
description: Documentation of cc-metric-collector's sinks
categories: [cc-metric-collector]
tags: [cc-metric-collector, Sink, General]
weight: 40
---

EOF
    cat "${SRCDIR}/sinks/README.md" >> "${DESTDIR}/sinks/_index.md"
fi


# Receivers
mkdir -p "${DESTDIR}/receivers"
for F in $RECEIVERS; do
    echo "$F"
    FNAME=$(basename "$F")
    TITLE=$(grep -E "^##" "$F" | head -n 1 | sed -e 's+## ++g')
    echo "'${TITLE//\`/}'"
    if [ "${TITLE}" == "" ]; then continue; fi
    rm --force "${DESTDIR}/receivers/${FNAME}"
    cat << EOF >> "${DESTDIR}/receivers/${FNAME}"
---
title: ${TITLE//\`/}
description: >
  Toplevel ${FNAME/.md/}
categories: [cc-metric-collector]
tags: [cc-metric-collector, Receiver, ${FNAME/Receiver.md/}]
weight: 2
---

EOF
    cat "$F" >> "${DESTDIR}/receivers/${FNAME}"
done

if [ -e "${SRCDIR}/receivers/README.md" ]; then
    cat << EOF > "${DESTDIR}/receivers/_index.md"
---
title: cc-metric-collector's receivers
description: Documentation of cc-metric-collector's receivers
categories: [cc-metric-collector]
tags: [cc-metric-collector, Receiver, General]
weight: 40
---

EOF
    cat "${SRCDIR}/receivers/README.md" >> "${DESTDIR}/receivers/_index.md"
fi

mkdir -p "${DESTDIR}/internal/metricRouter"
if [ -e "${SRCDIR}/internal/metricRouter/README.md" ]; then
    cat << EOF > "${DESTDIR}/internal/metricRouter/_index.md"
---
title: cc-metric-collector's router
description: Documentation of cc-metric-collector's router
categories: [cc-metric-collector]
tags: [cc-metric-collector, Router, General]
weight: 40
---

EOF
    cat "${SRCDIR}/internal/metricRouter/README.md" >> "${DESTDIR}/internal/metricRouter/_index.md"
fi

if [ -e "${SRCDIR}/README.md" ]; then
    cat << EOF > "${DESTDIR}/_index.md"
---
title: cc-metric-collector
description: Documentation of cc-metric-collector
categories: [cc-metric-collector]
tags: [cc-metric-collector, General]
weight: 40
---

EOF
    cat "${SRCDIR}/README.md" >> "${DESTDIR}/_index.md"
    sed -i -e 's+README.md+_index.md+g' "${DESTDIR}/_index.md"
fi


mkdir -p "${DESTDIR}/pkg/messageProcessor"
if [ -e "${SRCDIR}/pkg/messageProcessor/README.md" ]; then
    cat << EOF > "${DESTDIR}/pkg/messageProcessor/_index.md"
---
title: cc-metric-collector's message processor
description: Documentation of cc-metric-collector's message processor
categories: [cc-metric-collector]
tags: [cc-metric-collector, Message Processor]
weight: 40
---

EOF
    cat "${SRCDIR}/pkg/messageProcessor/README.md" >> "${DESTDIR}/pkg/messageProcessor/_index.md"
fi

