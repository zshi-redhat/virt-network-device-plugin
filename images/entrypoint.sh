#!/bin/bash

set -e

VIRT_DP_BINARY_FILE="/usr/src/virt-network-device-plugin/bin/virtdp"
SYS_BINARY_DIR="/usr/bin/"

cp -f $VIRT_DP_BINARY_FILE $SYS_BINARY_DIR

$SYS_BINARY_DIR/virtdp --logtostderr -v 10
