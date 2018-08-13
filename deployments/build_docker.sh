#!/usr/bin/env bash
set -e

## Build docker image
yes | cp ../bin/virtdp .
# cd deployments
docker build -t virt-device-plugin .
