FROM gcr.io/distroless/static:nonroot

COPY kai /usr/bin

USER nonroot:nobody

ENTRYPOINT ["kai"]
