# KAI (Kubernetes Automated Inventory)
Prototype for listing in-use images from k8s API

## Build
Note: Can't point this to ./kai because there's already a subdirectory named kai

`go build -o <localpath>/kai .`

### Docker
To build a docker image, you'll need to provide a kubeconfig. 

Note: Docker build requires files to be within the docker build context
```
docker build -t localhost/kai:latest --build-arg KUBECONFIG=./kubeconfig .
```

## Run
`<localpath>/kai`

### Docker
```
docker run -it --rm localhost/kai:latest --kubeconfig /kubeconfig
```
### Helm
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