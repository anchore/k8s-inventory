FROM golang:1.14.6-alpine AS builder
# Install git.
# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git
RUN mkdir -p /kai-build
WORKDIR /kai-build
COPY . .

# Fetch dependencies.
RUN go mod download

# Since we are deplying in a scratch image, disable cgo and build statically (and rebuild all dependencies without cgo)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /kai .
RUN chmod +x /kai

FROM scratch
MAINTAINER Sam Dacanay <sam@anchore.com>
# Copy our static executable.
COPY --from=builder /kai /kai
COPY --from=builder /kai-build/kai.yaml /.kai.yaml
ARG KUBECONFIG
ADD ${KUBECONFIG} /kubeconfig
HEALTHCHECK --interval=1m --timeout=5s \
    CMD ["/kai", "version"]

ENTRYPOINT ["/kai"]