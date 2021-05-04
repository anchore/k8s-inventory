# KAI (Kubernetes Automated Inventory)
[![CircleCI](https://circleci.com/gh/anchore/kai.svg?style=svg&circle-token=6f6ffa17b0630e6af622e162d594e2312c136d94)](https://circleci.com/gh/anchore/kai)
[![Go Report Card](https://goreportcard.com/badge/github.com/anchore/kai)](https://goreportcard.com/report/github.com/anchore/kai)
[![GitHub release](https://img.shields.io/github/release/anchore/kai.svg)](https://github.com/anchore/kai/releases/latest)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/anchore/kai/blob/main/LICENSE)

KAI polls the Kubernetes API on an interval to retrieve which Docker images are currently in use.

It can be run inside a cluster (under a Service Account) or outside (via any provided Kubeconfig).

## Getting Started
[Install the binary](#installation) or Download the [Docker image](https://hub.docker.com/repository/docker/anchore/kai)

## Installation
Kai can be run as a CLI, Docker Container, or Helm Chart

By default, Kai will look for a Kubeconfig in the home directory to use to authenticate (when run as a CLI). 

### CLI
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
...
```
### Container

In order to run kai as a container, it needs a kubeconfig
```
~ docker run -it --rm -v ~/.kube/config:/.kube/config anchore/kai:v0.1.0
{
 "timestamp": "2021-01-26T22:22:03Z",
 "results": [
  {
   "namespace": "kube-node-lease",
   "images": []
  },
  {
   "namespace": "kube-public",
   "images": []
  },
  {
   "namespace": "default",
   "images": [
    {
     "tag": "anchore/kai:v0.1.0",
     "repoDigest": "sha256:668cd005062d5a5b04dcf822556c02da50cbc08db079d2a0fe4ea45a396e0ac1"
    },
...
```

### Helm Chart

KAI is the foundation of Anchore Enterprise's Runtime Inventory feature. Running KAI via Helm is a great way to retrieve your Kubernetes Image inventory without providing Cluster Credentials to Anchore.

KAI runs as a read-only service account in the cluster it's deployed to. 

In order to report the inventory to Anchore, KAI does require authentication material for your Anchore Enterprise deployment.
KAI's helm chart automatically creates a kubernetes secret for the Anchore Password based on the values file you use, Ex.:
```
kai:
    anchore:
        password: foobar
```
It will set the following environment variable based on this: `KAI_ANCHORE_PASSWORD=foobar`.

If you don't want to store your Anchore password in the values file, you can create your own secret to do this:
```
apiVersion: v1
kind: Secret
metadata:
  name: kai-anchore-password
type: Opaque
stringData:
  KAI_ANCHORE_PASSWORD: foobar
```
and then provide it to the helm chart via the values file:
```
kai:
    existingSecret: kai-anchore-password
```
KAI's helm chart is part of the [charts.anchore.io](https://charts.anchore.io) repo. You can install it via:
```
helm repo add anchore https://charts.anchore.io
helm install <release-name> -f <values.yaml> anchore/kai
``` 
A basic values file can always be found [here](https://github.com/anchore/anchore-charts/tree/master/stable/kai/values.yaml)

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

## Known Limitations

### Unknown/Empty Digest
kai uses the Kubernetes GO SDK to fetch pods and parse out the image information in each namespace. This works well but has a known limitation around discovering an image's digest.
In Kubernetes, when a pod is deployed, kubernetes will automatically try to pull the image from the remote registry. If this download happens, we are able to retrieve a digest, because the image ID field has a prefix 'docker-pullable'. 
In some cases, like local development, the image already exists on the kubernetes node (this is common in development situations where kubernetes is running on a workstation in docker-desktop or kind). Since kubernetes is NOT pulling the image from a remote location, it sets the image ID field to the image ID locally, which is NOT the same as the image digest. Therefore, kai cannot retrieve this information and will report the value as an empty string.  
