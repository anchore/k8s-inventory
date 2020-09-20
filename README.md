# KAI (Kubernetes Automated Inventory)
Prototype for listing in-use images from k8s API

## Build
Note: Can't point this to ./kai because there's already a subdirectory named kai

`go build -o <localpath>/kai .`

## Run
`<localpath>/kai`

## Configuration
```yaml
# same as -o ; the output format (options: table, json). 
# Only respected if Kai is printing results to STDOUT
output: "json"

# same as -q ; suppress all output (except for results)
quiet: false

log:
  # use structured logging
  structured: false

  # the log level; note: detailed logging suppress the ETUI
  level: "debug"

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

# Only respected if mode is periodic (defaults to 300)
polling-interval-seconds: 60

```