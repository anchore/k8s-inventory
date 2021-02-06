#!/usr/bin/env bash

set -eux

CLUSTER_NAME=$1

echo "Tearing down kind cluster." && \
./kind delete cluster --name "$CLUSTER_NAME"