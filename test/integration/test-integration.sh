#!/usr/bin/env bash

set -eux

LATEST_COMMIT_HASH=$(git rev-parse HEAD | cut -c 1-8)
RELEASE="integration-kai-$LATEST_COMMIT_HASH"
## Note: if you change this value, get_images_test.go must be updated
NAMESPACE="kai-integration-test"
CLUSTER_NAME=$1

function cleanup () {
  echo "Removing Helm Release '$RELEASE' and Namespace '$NAMESPACE'"
  ./helm uninstall "$RELEASE" -n "$NAMESPACE"

  echo "Tearing down Kubernetes namespace: $NAMESPACE"
  ./kubectl delete namespace "$NAMESPACE"
}
trap cleanup EXIT

## Install a basic nginx container
./helm install "$RELEASE" --create-namespace --namespace "$NAMESPACE" ./test/integration/fixtures/hello-world

./kubectl wait --for=condition=available "deployment/$RELEASE-hello-world" --timeout=-1s --namespace "$NAMESPACE"

go test -v -tags=integration ./test/integration
