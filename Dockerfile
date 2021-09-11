FROM gcr.io/distroless/static:nonroot

COPY kai /

USER nonroot:nobody

ENTRYPOINT ["/kai"]
CMD ["--config", "/.kai.yaml"]
