# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/geoip-updater .

# Stage 2: Runtime
FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/geoip-updater .

# Default config and data directories
RUN mkdir -p /config /data
VOLUME ["/config", "/data"]

ENTRYPOINT ["/app/geoip-updater"]
