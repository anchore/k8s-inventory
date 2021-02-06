#!/usr/bin/env bash

set -eux

LATEST_COMMIT_HASH=$(git rev-parse HEAD | cut -c 1-8)
RELEASE="acceptance-kai-$LATEST_COMMIT_HASH"

function cleanup () {
  echo "Removing Helm Release: $RELEASE"
  helm uninstall "$RELEASE"
}
trap cleanup EXIT

helm repo add anchore https://charts.anchore.io

helm install "$RELEASE" -f ./test/acceptance/fixtures/helm/values.yaml anchore/kai

sleep 1
max_iterations=20
iterations=0
while [[ $(kubectl get pods -l app.kubernetes.io/name=kai -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]];
do
  echo "waiting for pod to be ready" && sleep 1
  iterations=$((iterations+1))
  if [[ "$iterations" -ge "$max_iterations" ]]; then
    echo "Timeout Waiting for pod"
    exit 1
  fi
done
