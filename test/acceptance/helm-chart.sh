#!/bin/zsh

set -eux
set -o pipefail

LATEST_COMMIT_HASH=$(git rev-parse HEAD | cut -c 1-8)
RELEASE="acceptance-kai-$LATEST_COMMIT_HASH"
function install_helm () {
  curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
  chmod 700 get_helm.sh
  ./get_helm.sh
}

function install_kubectl () {
  curl -L -o /usr/local/bin/kubectl "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
  chmod 700 /usr/local/bin/kubectl
}

function cleanup () {
  helm uninstall "$RELEASE"
}
trap cleanup EXIT

helm version || install_helm
kubectl version || install_kubectl

helm repo add anchore https://charts.anchore.io

helm install "$RELEASE" -f ./test/acceptance/fixtures/helm/values.yaml anchore/kai

max_iterations=20
iterations=0
while [[ $(kubectl get pods -l app.kubernetes.io/name=kai -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]];
do
  echo "waiting for pod to be ready" && sleep 1
  ((iterations++))
  if [[ "$iterations" -ge "$max_iterations" ]]; then
    echo "Timeout Waiting for pod"
    exit 1
  fi
done

echo "KAI Helm Chart Successfully Installed, removing"