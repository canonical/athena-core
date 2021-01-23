#!/bin/bash

[ -f $SNAP_COMMON/athena.default ] && . $SNAP_COMMON/athena.default

processor --config $SNAP_COMMON/athena-processor.yaml $PROCESSOR_OPTS
