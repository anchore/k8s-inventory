# KAI (Kubernetes Automated Inventory)
[![CircleCI](https://circleci.com/gh/anchore/kai.svg?style=svg&circle-token=6f6ffa17b0630e6af622e162d594e2312c136d94)](https://circleci.com/gh/anchore/kai)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/anchore/kai/blob/main/LICENSE)

KAI polls the Kubernetes API on an interval to retrieve which Docker images are currently in use.

It can be run inside a cluster (under a Service Account) or outside (via any provided Kubeconfig)

## Getting Started
[Install the binary](#installation) or Download the [Docker image](https://hub.docker.com/repository/docker/dakaneye/kai)

With the binary, you may retrieve the running images in each namespace with the following command:

## Installation
Kai can be run as a CLI, Docker Container, or Helm Chart

By default, Kai will look for a Kubeconfig in the home directory.

**CLI**:
```shell script
$ kai
{
 "timestamp": "2020-09-21T21:36:46Z",
 "results": [
  {
   "namespace": "docker",
   "images": [
    "docker/kube-compose-controller:v0.4.25-alpha1",
    "docker/kube-compose-api-server:v0.4.25-alpha1"
   ]
  },
  {
   "namespace": "kube-system",
   "images": [
    "k8s.gcr.io/coredns:1.6.2",
    "k8s.gcr.io/etcd:3.3.15-0",
    "k8s.gcr.io/kube-apiserver:v1.16.5",
    "k8s.gcr.io/kube-controller-manager:v1.16.5",
    "k8s.gcr.io/kube-proxy:v1.16.5",
    "k8s.gcr.io/kube-scheduler:v1.16.5",
    "docker/desktop-storage-provisioner:v1.1",
    "docker/desktop-vpnkit-controller:v1.0"
   ]
  }
 ]
}
```

**Docker Image:**
```shell script
cp ~/.kube/config ./kubeconfig
docker build -t localhost/kai:latest --build-arg KUBECONFIG=./kubeconfig
docker run -it --rm localhost/kai:latest --kubeconfig /kubeconfig
...
```

**Helm Chart:**

Kai will use the in-cluster config in order to connect to the K8s API, and is governed by the [defined Service Account](./helm/kai/templates/serviceaccount.yaml)
The [values.yaml](./helm/kai/values.yaml) can be [customized](#configuration) as needed.
```
helm install local-kai helm/kai
``` 

## Configuration
```yaml
# same as -o ; the output format (options: table, json)
output: "json"

# same as -q ; suppress all output (except for the inventory results)
quiet: false

log:
  # use structured logging
  structured: false

  # the log level; note: detailed logging suppress the ETUI
  level: "warn"

  # location to write the log file (default is not to have a log file)
  file: ""

# enable/disable checking for application updates on startup
check-for-app-update: true

# Which namespaces to search (can just be a single element "all" or it can be multiple)
namespaces:
  - default
  - docker
  - kube-system

# Can be one of adhoc, periodic (defaults to adhoc)
mode: periodic

# Only respected if mode is periodic
polling-interval-seconds: 300

anchore: {}
  # url: 
  # user: admin
  # password: foobar
  # http:
  #   insecure: false
  #   timeoutSeconds: 10

```

## Developing
### Build
Note: Can't point this to ./kai because there's already a subdirectory named kai

`go build -o <localpath>/kai .`

#### Docker
To build a docker image, you'll need to provide a kubeconfig. 

Note: Docker build requires files to be within the docker build context
```
docker build -t localhost/kai:latest --build-arg KUBECONFIG=./kubeconfig .
```

### Shell Completion
Kai comes with shell completion for specifying namespaces, it can be enabled as follows. Run with the `--help` command to get the instructions for the shell of your choice
```
kai completion <zsh|bash|fish>
```