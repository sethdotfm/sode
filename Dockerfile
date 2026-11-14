FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH
ARG COMMIT=
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Tests run in CI (ci.yml, rc.yml) before any image is pushed, not here.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X github.com/sethdotfm/sode/internal/app.Commit=$COMMIT" -o /out/sode ./cmd/sode \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/swp08sim ./cmd/swp08sim \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot AS sim
LABEL org.opencontainers.image.source="https://github.com/sethdotfm/sode"
COPY --from=build /out/swp08sim /swp08sim
EXPOSE 8910
ENTRYPOINT ["/swp08sim"]

FROM gcr.io/distroless/static-debian12:nonroot AS sode
LABEL org.opencontainers.image.source="https://github.com/sethdotfm/sode"
COPY --from=build /out/sode /sode
COPY --from=build --chown=65532:65532 /out/data /data
ENV SODE_HISTORY=/data/history.jsonl SODE_SENTLOG=/data/sent.jsonl SODE_UPTIME=/data/uptime.jsonl SODE_CONFIG=/data/config.yaml
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD ["/sode", "-healthcheck"]
ENTRYPOINT ["/sode"]
