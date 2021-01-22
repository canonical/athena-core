#!/bin/bash

[ -f $SNAP_COMMON/athena.default ] && . $SNAP_COMMON/athena.default

monitor --config $SNAP_COMMON/athena-monitor.yaml $MONITOR_OPTS
