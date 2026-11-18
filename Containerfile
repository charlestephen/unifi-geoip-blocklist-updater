# Stage 1: Build
FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/geoip-updater .

# Stage 2: Runtime
FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/geoip-updater .

# Default config and data directories
RUN mkdir -p /config /data
VOLUME ["/config", "/data"]

ENTRYPOINT ["/app/geoip-updater"]
