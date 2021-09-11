FROM gcr.io/distroless/static:nonroot

COPY kai /

ENTRYPOINT ["/kai"]

CMD ["--config", "/.kai.yaml"]

