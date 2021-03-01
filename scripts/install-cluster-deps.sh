#!/usr/bin/env bash

set -eux

HELM_VERSION=v3.2.0

echo "Installing dependencies to run local k8s cluster."
ARCH=$(uname | tr '[:upper:]' '[:lower:]')
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