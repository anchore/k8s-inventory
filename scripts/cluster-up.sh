#!/usr/bin/env bash

set -eux

K8S_VERSION=1.19.0
KIND_VERSION=v0.9.0
HELM_VERSION=v3.2.0
CLUSTER_NAME=$1
CLUSTER_CONFIG=./test/integration/fixtures/kind-config.yaml

echo "Installing dependencies to run local k8s cluster."
ARCH=$(uname | tr '[:upper:]' '[:lower:]')
if [[ ! -x "./kind" ]]; then \
  echo "Installing kind" && \
  curl -qsSLo "./kind" "https://github.com/kubernetes-sigs/kind/releases/download/$KIND_VERSION/kind-$ARCH-amd64" && \
  chmod +x "./kind"; \
else \
  echo "Kind already installed."; \
fi
if [[ ! -x "./helm" ]]; then \
  echo "Installing helm" && \
  curl -sSL "https://get.helm.sh/helm-$HELM_VERSION-$ARCH-amd64.tar.gz" | tar xzf - -C "." --strip-components=1 "$ARCH-amd64/helm" && \
  chmod +x "./helm"; \
else \
  echo "helm already installed."; \
fi
if [[ ! -x "./kubectl" ]]; then \
  echo "Installing kubectl" && \
  curl -sSLo "./kubectl" "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/$ARCH/amd64/kubectl" && \
  chmod +x "./kubectl"; \
else \
  echo "kubectl already installed."; \
fi

if ! ./kind get clusters | grep "$CLUSTER_NAME"; then \
  echo "Starting kind cluster." && \
  ./kind create cluster --name "$CLUSTER_NAME" --config "$CLUSTER_CONFIG" --image "kindest/node:v$K8S_VERSION" --wait 60s; \
  ./kubectl config use-context "kind-$CLUSTER_NAME"
else \
  echo "Kind cluster already running."; \
fi