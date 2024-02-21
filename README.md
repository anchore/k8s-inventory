# Anchore Kubernetes Inventory
[![Go Report Card](https://goreportcard.com/badge/github.com/anchore/k8s-inventory)](https://goreportcard.com/report/github.com/anchore/k8s-inventory)
[![GitHub release](https://img.shields.io/github/release/anchore/k8s-inventory.svg)](https://github.com/anchore/k8s-inventory/releases/latest)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/anchore/k8s-inventory/blob/main/LICENSE)

`anchore-k8s-inventory` polls the Kubernetes API on an interval to retrieve which images are currently in use.

It can be run inside a cluster (under a Service Account) or outside (via any provided kubeconfig).

## Getting Started
[Install the binary](#installation) or Download the [Docker image](https://hub.docker.com/r/anchore/k8s-inventory/tags)

## Installation
`anchore-k8s-inventory` can be run as a CLI, Docker Container, or Helm Chart

By default, `anchore-k8s-inventory` will look for a kubeconfig in the home directory to use to authenticate (when run as a CLI).

### CLI
```shell script
$ anchore-k8s-inventory --verbose-inventory-reports
{
  "cluster_name": "docker-desktop",
  "containers": [
    {
      "id": "docker://911d2cf6351cbafc349f131aeef1b1fb295a889504d38c89a065da1a91d828b9",
      "image_digest": "sha256:76049887f07a0476dc93efc2d3569b9529bf982b22d29f356092ce206e98765c",
      "image_tag": "docker.io/kubernetesui/metrics-scraper:v1.0.8",
      "name": "dashboard-metrics-scraper",
      "pod_uid": "c5b40099-20a5-4b46-8062-cf84f9d6ac23"
    },
    {
      "id": "docker://a9cd75ad99dd4363bbd882b40e753b58c62bfd7b03cabeb764c1dac97568ad26",
      "image_digest": "sha256:2e500d29e9d5f4a086b908eb8dfe7ecac57d2ab09d65b24f588b1d449841ef93",
      "image_tag": "docker.io/kubernetesui/dashboard:v2.7.0",
      "name": "kubernetes-dashboard",
      "pod_uid": "72ba7e4e-6e35-48c0-bff7-558a525074d5"
    },
	.....
  ],
  "namespaces": [
    {
      "labels": {
        "kubernetes.io/metadata.name": "kube-public"
      },
      "name": "kube-public",
      "uid": "dd561bf1-11ff-4381-8a1f-f156c206fe13"
    },
    {
      "labels": {
        "kubernetes.io/metadata.name": "kube-system"
      },
      "name": "kube-system",
      "uid": "012ebe67-dd49-4fd9-b604-258385df3957"
    },
	.....
  ],
  "nodes": [
    {
      "annotations": {
        "kubeadm.alpha.kubernetes.io/cri-socket": "unix:///var/run/cri-dockerd.sock",
        "node.alpha.kubernetes.io/ttl": "0",
        "volumes.kubernetes.io/controller-managed-attach-detach": "true"
      },
      "arch": "arm64",
      "container_runtime_version": "docker://20.10.23",
      "kernel_version": "5.15.49-linuxkit",
      "kube_proxy_version": "v1.26.1",
      "kubelet_version": "v1.26.1",
      "labels": {
        "beta.kubernetes.io/arch": "arm64",
        "beta.kubernetes.io/os": "linux",
        "kubernetes.io/arch": "arm64",
        "kubernetes.io/hostname": "minikube",
        "kubernetes.io/os": "linux",
        "minikube.k8s.io/commit": "ddac20b4b34a9c8c857fc602203b6ba2679794d3",
        "minikube.k8s.io/name": "minikube",
        "minikube.k8s.io/primary": "true",
        "minikube.k8s.io/updated_at": "2023_04_11T11_20_54_0700",
        "minikube.k8s.io/version": "v1.29.0",
        "node-role.kubernetes.io/control-plane": "",
        "node.kubernetes.io/exclude-from-external-load-balancers": ""
      },
      "name": "minikube",
      "operating_system": "linux",
      "uid": "b8334e25-68a5-4cbc-bf7a-fc188f2c6023"
    }
  ],
  "pods": [
    {
      "annotations": {
        "seccomp.security.alpha.kubernetes.io/pod": "runtime/default"
      },
      "labels": {
        "k8s-app": "dashboard-metrics-scraper",
        "pod-template-hash": "5c6664855"
      },
      "name": "dashboard-metrics-scraper-5c6664855-s8lpc",
      "namespace_uid": "c1d98ff5-6689-4016-aef3-8802790c3b10",
      "node_uid": "b8334e25-68a5-4cbc-bf7a-fc188f2c6023",
      "uid": "c5b40099-20a5-4b46-8062-cf84f9d6ac23"
    },
    {
      "labels": {
        "gcp-auth-skip-secret": "true",
        "k8s-app": "kubernetes-dashboard",
        "pod-template-hash": "55c4cbbc7c"
      },
      "name": "kubernetes-dashboard-55c4cbbc7c-6p28m",
      "namespace_uid": "c1d98ff5-6689-4016-aef3-8802790c3b10",
      "node_uid": "b8334e25-68a5-4cbc-bf7a-fc188f2c6023",
      "uid": "72ba7e4e-6e35-48c0-bff7-558a525074d5"
    },
	.....
  ],
  "serverVersionMetadata": {
    "major": "1",
    "minor": "26",
    "gitVersion": "v1.26.1",
    "gitCommit": "8f94681cd294aa8cfd3407b8191f6c70214973a4",
    "gitTreeState": "clean",
    "buildDate": "2023-01-18T15:51:25Z",
    "goVersion": "go1.19.5",
    "compiler": "gc",
    "platform": "linux/arm64"
  },
  "timestamp": "2023-05-03T12:34:13Z"
}
```
### Container

In order to run `anchore-k8s-inventory` as a container, it needs a kubeconfig
```sh
~ docker run -it --rm -v ~/.kube/config:/.kube/config anchore/k8s-inventory:latest --verbose-inventory-reports
```

### Helm Chart

Anchore-k8s-inventory is the foundation of Anchore Enterprise's Runtime Inventory feature. Running anchore-k8s-inventory via Helm is a great way to retrieve your Kubernetes Image inventory without providing Cluster Credentials to Anchore.

Anchore-k8s-inventory runs as a read-only service account in the cluster it's deployed to.

In order to report the inventory to Anchore, anchore-k8s-inventory does require authentication material for your Anchore Enterprise deployment.
anchore-k8s-inventory's helm chart automatically creates a kubernetes secret for the Anchore Password based on the values file you use, Ex.:

```yaml
anchore-k8s-inventory:
  anchore:
    password: foobar
```

It will set the following environment variable based on this: `ANCHORE_K8S_INVENTORY_ANCHORE_PASSWORD=foobar`.

If you don't want to store your Anchore password in the values file, you can create your own secret to do this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: anchore-k8s-inventory-anchore-password
type: Opaque
stringData:
  ANCHORE_K8S_INVENTORY_ANCHORE_PASSWORD: foobar
```

and then provide it to the helm chart via the values file:

```yaml
anchore-k8s-inventory:
  existingSecret: anchore-k8s-inventory-anchore-password
```

anchore-k8s-inventory's helm chart is part of the [charts.anchore.io](https://charts.anchore.io) repo. You can install it via:

```sh
helm repo add anchore https://charts.anchore.io
helm install <release-name> -f <values.yaml> anchore/anchore-k8s-inventory
```

A basic values file can always be found [here](https://github.com/anchore/anchore-charts/tree/main/stable/k8s-inventory/values.yaml)

## Configuration
```yaml
# same as -q ; suppress all output (except for the inventory results)
quiet: false

log:
  # use structured logging
  structured: false

  # the log level; note: detailed logging suppress the ETUI
  level: "debug"

  # location to write the log file (default is not to have a log file)
  file: "./anchore-k8s-inventory.log"

# enable/disable checking for application updates on startup
check-for-app-update: true

kubeconfig:
  path:
  cluster: docker-desktop
  cluster-cert:
  server:  # ex. https://kubernetes.docker.internal:6443
  user:
    type:  # valid: [private_key, token]
    client-cert:
    private-key:
    token:

# enable/disable printing inventory reports to stdout
verbose-inventory-reports: false
```

### Namespace selection

Configure which namespaces anchore-k8s-inventory should search.

* `include` section
  * A list of explicit strings that will detail the list of namespaces to capture image data from.
  * If left as an empty list `[]` all namespaces will be searched
  * Example:

```yaml
namespace-selectors:
  include:
  - default
  - kube-system
  - prod-app
```

* `exclude` section
  * A list of explicit strings and/or regex patterns for namespaces to be excluded.
  * A regex is determined if the string does not match standard DNS name requirements.
  * Example:

```yaml
namespace-selectors:
  exclude:
  - default
  - ^kube-*
  - ^prod-*
```

```yaml
# Which namespaces to search or exclude.
namespace-selectors:
  # Namespaces to include as explicit strings, not regex
  # NOTE: Will search ALL namespaces if left as an empty array
  include: []

  # List of namespaces to exclude, can use explicit strings and/or regexes.
  # For example
  #
  # list:
  # - default
  # - ^kube-*
  #
  # Will exclude the default, kube-system, and kube-public namespaces
  exclude: []
```

### Kubernetes API Parameters

This section will allow users to tune the way anchore-k8s-inventory interacts with the kubernetes API server.

```yaml
# Kubernetes API configuration parameters (should not need tuning)
kubernetes:
  # Sets the request timeout for kubernetes API requests
  request-timeout-seconds: 60

  # Sets the number of objects to iteratively return when listing resources
  request-batch-size: 100

  # Worker pool size for collecting pods from namespaces. Adjust this if the api-server gets overwhelmed
  worker-pool-size: 100
```

### anchore-k8s-inventory mode of operation

```yaml
# Can be one of adhoc, periodic (defaults to adhoc)
mode: adhoc

# Only respected if mode is periodic
polling-interval-seconds: 300
```

### Missing Tag Policy

There are cases where images in Kubernetes do not have an associated tag - for
example when an image is deployed using the digest.

```sh
kubectl run python --image=python@sha256:f0a210a37565286ecaaac0529a6749917e8ea58d3dfc72c84acfbfbe1a64a20a
```

Anchore Enterprise will use the image digest to process an image but it still requires a tag to be
associated with the image. The `missing-tag-policy` lets you configure the best way to handle the
missing tag edge case in your environment.

**digest** will use the image digest as a dummy tag.
```json
{
  "tag": "alpine:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
  "repoDigest": "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba"
}
```

**insert** will use a dummy tag configured by `missing-tag-policy.tag`
```json
{
  "tag": "alpine:UNKNOWN",
  "repoDigest": "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba"
}
```

**drop** will simply ignore the images that don't have tags.


```yaml
# Handle cases where a tag is missing. For example - images designated by digest
missing-tag-policy:
  # One of the following options [digest, insert, drop]. Default is 'digest'
  #
  # [digest] will use the image's digest as a dummy tag.
  #
  # [insert] will insert a default tag in as a dummy tag. The dummy tag is
  #          customizable under missing-tag-policy.tag
  #
  # [drop] will drop images that do not have tags associated with them. Not
  #        recommended.
  policy: digest

  # Dummy tag to use. Only applicable if policy is 'insert'. Defaults to UNKNOWN
  tag: UNKNOWN
```

### Ignore images that are not yet in a Running state

```yaml
# Ignore images out of pods that are not in a Running state
ignore-not-running: true
```

### Anchore API configuration

Use this section to configure the Anchore Enterprise API endpoint

```yaml
anchore:
  url: <your anchore api url>
  user: <anchore-k8s-inventory_inventory_user>
  password: $ANCHORE_K8S_INVENTORY_ANCHORE_PASSWORD
  http:
    insecure: true
    timeout-seconds: 60
```

## Behavior change (v0.5.0) (formerly KAI) 

In versions of anchore-k8s-inventory < v0.5.0 the default behavior was to output the inventory report
to stdout every time it was generated. anchore-k8s-inventory v0.5.0 changes this so it will not print
to stdout unless `verbose-inventory-reports: true` is set in the config file or
anchore-k8s-inventory is called with the `--verbose-inventory-reports` flag.

## Configuration Changes (v0.2.2 -> v0.3.0) (formerly KAI) 

There are a few configurations that were changed from v0.2.2 to v0.3.0

#### `kubernetes-request-timeout-seconds`

The request timeout for the kubernetes API was changed from

```yaml
kubernetes-request-timeout-seconds: 60
```

to

```yaml
kubernetes:
  request-timeout-seconds: 60
```

anchore-k8s-inventory will still honor the old configuration. It will prefer the old configuration
parameter until it is removed from the config entirely. It is safe to remove the
old configuration in favor of the new config.

#### `namespaces`

The namespace configuration was changed from

```yaml
namespaces:
- all
```

to

```yaml
namespace-selectors:
  include: []
  exclude: []
```

`namespace-selectors` was added to eventually replace `namespaces` to allow for both
include and exclude configs. The old `namespaces` array will be honored if
`namespace-selectors.include` is empty. It is safe to remove `namespaces` entirely
in favor of `namespace-selectors`

## Developing
### Build
**Note:** This will drop the binary in the `./snapshot/` directory

**On Mac**
```sh
make mac-binary
```

**On Linux**
```sh
make linux-binary
```

### Testing

The Makefile has testing built into it. For unit tests simply run

```sh
make unit
```

### Docker
To build a docker image, you'll need to provide a kubeconfig.

Note: Docker build requires files to be within the docker build context

```sh
docker build -t localhost/anchore-k8s-inventory:latest --build-arg KUBECONFIG=./kubeconfig .
```

### Shell Completion
anchore-k8s-inventory comes with shell completion for specifying namespaces, it can be enabled as follows. Run with the `--help` command to get the instructions for the shell of your choice

```sh
anchore-k8s-inventory completion <zsh|bash|fish>
```

### Using Skaffold
You can use skaffold for dev. The 'bootstrap-skaffold' make target will clone the chart into the current directory to wire
it up for skaffold to use. To trigger redeployments you'll need to run `make linux-binary` and skaffold will rebuild the image
and update the helm release.

```sh
make bootstrap-skaffold
make linux-binary
skaffold dev
```

## Releasing
To create a release of anchore-k8s-inventory, a tag needs to be created that points to a commit in `main`
that we want to release. This tag shall be a semver prefixed with a `v`, e.g. `v0.2.7`.
This will trigger a GitHub Action that will create the release.

After the release has been successfully created, make sure to specify the updated version
in both Enterprise and the anchore-k8s-inventory Helm Chart in
[anchore-charts](https://github.com/anchore/anchore-charts).
