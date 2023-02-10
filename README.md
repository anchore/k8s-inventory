# KAI (Kubernetes Automated Inventory)
[![CircleCI](https://circleci.com/gh/anchore/kai.svg?style=svg&circle-token=6f6ffa17b0630e6af622e162d594e2312c136d94)](https://circleci.com/gh/anchore/kai)
[![Go Report Card](https://goreportcard.com/badge/github.com/anchore/kai)](https://goreportcard.com/report/github.com/anchore/kai)
[![GitHub release](https://img.shields.io/github/release/anchore/kai.svg)](https://github.com/anchore/kai/releases/latest)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/anchore/kai/blob/main/LICENSE)

KAI polls the Kubernetes API on an interval to retrieve which images are currently in use.

It can be run inside a cluster (under a Service Account) or outside (via any provided Kubeconfig).

## Getting Started
[Install the binary](#installation) or Download the [Docker image](https://hub.docker.com/repository/docker/anchore/kai)

## Installation
KAI can be run as a CLI, Docker Container, or Helm Chart

By default, KAI will look for a Kubeconfig in the home directory to use to authenticate (when run as a CLI).

### CLI
```shell script
$ kai
{
  "timestamp": "2021-11-17T18:47:36Z",
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
          "tag": "alpine:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
          "repoDigest": "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba"
        },
        {
          "tag": "memcached:05a8f320f47594e13a995ce6010bf1a1ffefbc0801af3db71a4b307d80507e1f",
          "repoDigest": "sha256:05a8f320f47594e13a995ce6010bf1a1ffefbc0801af3db71a4b307d80507e1f"
        },
        {
          "tag": "python:f0a210a37565286ecaaac0529a6749917e8ea58d3dfc72c84acfbfbe1a64a20a",
          "repoDigest": "sha256:f0a210a37565286ecaaac0529a6749917e8ea58d3dfc72c84acfbfbe1a64a20a"
        }
      ]
    },
    {
      "namespace": "kube-system",
      "images": [
        {
          "tag": "602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni-init:v1.7.5-eksbuild.1",
          "repoDigest": "sha256:d96d712513464de6ce94e422634a25546565418f20d1b28d3bce399d578f3296"
        },
        {
          "tag": "602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:v1.7.5-eksbuild.1",
          "repoDigest": "sha256:f310c918ee2b4ebced76d2d64a2ec128dde3b364d1b495f0ae73011f489d474d"
        },
        {
          "tag": "602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/coredns:v1.8.4-eksbuild.1",
          "repoDigest": "sha256:fcb60ebdb0d8ec23abe46c65d0f650d9e2bf2f803fac004ceb1f0bf348db0fd0"
        },
        {
          "tag": "602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-proxy:v1.21.2-eksbuild.2",
          "repoDigest": "sha256:0ea6717ed144c7f04922bf56662d58d5b14b7b62ef78c70e636a02d22052681c"
        }
      ]
    }
  ],
  "serverVersionMetadata": {
    "major": "1",
    "minor": "21+",
    "gitVersion": "v1.21.2-eks-06eac09",
    "gitCommit": "5f6d83fe4cb7febb5f4f4e39b3b2b64ebbbe3e97",
    "gitTreeState": "clean",
    "buildDate": "2021-09-13T14:20:15Z",
    "goVersion": "go1.16.5",
    "compiler": "gc",
    "platform": "linux/amd64"
  },
  "cluster_name": "eks-prod",
  "inventory_type": "kubernetes"
}
```
### Container

In order to run kai as a container, it needs a kubeconfig
```sh
~ docker run -it --rm -v ~/.kube/config:/.kube/config anchore/kai:v0.3.0
{
  "timestamp": "2021-11-17T18:47:36Z",
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
          "tag": "alpine:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba",
          "repoDigest": "sha256:4ed1812024ed78962a34727137627e8854a3b414d19e2c35a1dc727a47e16fba"
        },
        {
          "tag": "memcached:05a8f320f47594e13a995ce6010bf1a1ffefbc0801af3db71a4b307d80507e1f",
          "repoDigest": "sha256:05a8f320f47594e13a995ce6010bf1a1ffefbc0801af3db71a4b307d80507e1f"
        },
        {
          "tag": "python:f0a210a37565286ecaaac0529a6749917e8ea58d3dfc72c84acfbfbe1a64a20a",
          "repoDigest": "sha256:f0a210a37565286ecaaac0529a6749917e8ea58d3dfc72c84acfbfbe1a64a20a"
        }
      ]
    },
    {
      "namespace": "kube-system",
      "images": [
        {
          "tag": "602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni-init:v1.7.5-eksbuild.1",
          "repoDigest": "sha256:d96d712513464de6ce94e422634a25546565418f20d1b28d3bce399d578f3296"
        },
        {
          "tag": "602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:v1.7.5-eksbuild.1",
          "repoDigest": "sha256:f310c918ee2b4ebced76d2d64a2ec128dde3b364d1b495f0ae73011f489d474d"
        },
        {
          "tag": "602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/coredns:v1.8.4-eksbuild.1",
          "repoDigest": "sha256:fcb60ebdb0d8ec23abe46c65d0f650d9e2bf2f803fac004ceb1f0bf348db0fd0"
        },
        {
          "tag": "602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-proxy:v1.21.2-eksbuild.2",
          "repoDigest": "sha256:0ea6717ed144c7f04922bf56662d58d5b14b7b62ef78c70e636a02d22052681c"
        }
      ]
    }
  ],
  "serverVersionMetadata": {
    "major": "1",
    "minor": "21+",
    "gitVersion": "v1.21.2-eks-06eac09",
    "gitCommit": "5f6d83fe4cb7febb5f4f4e39b3b2b64ebbbe3e97",
    "gitTreeState": "clean",
    "buildDate": "2021-09-13T14:20:15Z",
    "goVersion": "go1.16.5",
    "compiler": "gc",
    "platform": "linux/amd64"
  },
  "cluster_name": "eks-prod",
  "inventory_type": "kubernetes"
}
```

### Helm Chart

KAI is the foundation of Anchore Enterprise's Runtime Inventory feature. Running KAI via Helm is a great way to retrieve your Kubernetes Image inventory without providing Cluster Credentials to Anchore.

KAI runs as a read-only service account in the cluster it's deployed to.

In order to report the inventory to Anchore, KAI does require authentication material for your Anchore Enterprise deployment.
KAI's helm chart automatically creates a kubernetes secret for the Anchore Password based on the values file you use, Ex.:

```yaml
kai:
  anchore:
    password: foobar
```

It will set the following environment variable based on this: `KAI_ANCHORE_PASSWORD=foobar`.

If you don't want to store your Anchore password in the values file, you can create your own secret to do this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kai-anchore-password
type: Opaque
stringData:
  KAI_ANCHORE_PASSWORD: foobar
```

and then provide it to the helm chart via the values file:

```yaml
kai:
  existingSecret: kai-anchore-password
```

KAI's helm chart is part of the [charts.anchore.io](https://charts.anchore.io) repo. You can install it via:

```sh
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
  level: "debug"

  # location to write the log file (default is not to have a log file)
  file: "./kai.log"

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
```

### Namespace selection

Configure which namespaces kai should search.

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

This section will allow users to tune the way kai interacts with the kubernetes API server.

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

### KAI mode of operation

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
  user: <kai_inventory_user>
  password: $KAI_ANCHORE_PASSWORD
  http:
    insecure: true
    timeout-seconds: 10
```

## Configuration Changes (v0.2.2 -> v0.3.0)

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

KAI will still honor the old configuration. It will prefer the old configuration
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

To use FIPS boringcrypto:

```sh
make mac-binary-fips
```

**On Linux**

```sh
make linux-binary
```

To use FIPS boringcrypto:

```sh
make linux-binary-fips
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
docker build -t localhost/kai:latest --build-arg KUBECONFIG=./kubeconfig .
```

### Shell Completion
KAI comes with shell completion for specifying namespaces, it can be enabled as follows. Run with the `--help` command to get the instructions for the shell of your choice

```sh
kai completion <zsh|bash|fish>
```

## Releasing
To create a release of kai, a tag needs to be created that points to a commit in `main`
that we want to release. This tag shall be a semver prefixed with a `v`, e.g. `v0.2.7`.
This will trigger a GitHub Action that will create the release.

After the release has been successfully created, make sure to specify the updated version
in both Enterprise and the KAI Helm Chart in
[anchore-charts](https://github.com/anchore/anchore-charts).
