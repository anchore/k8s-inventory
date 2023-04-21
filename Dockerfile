FROM gcr.io/distroless/static:nonroot

COPY anchore-k8s-inventory /usr/bin

USER nonroot:nobody

ARG BUILD_DATE
ARG BUILD_VERSION
ARG VCS_REF
ARG VCS_URL

LABEL org.opencontainers.image.created=$BUILD_DATE
LABEL org.opencontainers.image.title="anchore-k8s-inventory"
LABEL org.opencontainers.image.description="AKI (Anchore Kubernetes Inventory) can poll Kubernetes Cluster API(s) to tell Anchore which Images are currently in-use"
LABEL org.opencontainers.image.source=$VCS_URL
LABEL org.opencontainers.image.revision=$VCS_REF
LABEL org.opencontainers.image.vendor="Anchore, Inc."
LABEL org.opencontainers.image.version=$BUILD_VERSION
LABEL org.opencontainers.image.licenses="Apache-2.0"

ENTRYPOINT ["anchore-k8s-inventory"]
