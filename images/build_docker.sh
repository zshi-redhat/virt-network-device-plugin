#!/usr/bin/env bash
set -e

## Build docker image
docker build -t virt-device-plugin -f ./Dockerfile  ../
