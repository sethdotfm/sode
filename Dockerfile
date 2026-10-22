FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./... \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sode ./cmd/sode \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/swp08sim ./cmd/swp08sim \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot AS sim
COPY --from=build /out/swp08sim /swp08sim
EXPOSE 8910
ENTRYPOINT ["/swp08sim"]

FROM gcr.io/distroless/static-debian12:nonroot AS sode
COPY --from=build /out/sode /sode
COPY --from=build --chown=65532:65532 /out/data /data
ENV SODE_HISTORY=/data/history.jsonl SODE_CONFIG=/data/config.yaml
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD ["/sode", "-healthcheck"]
ENTRYPOINT ["/sode"]
