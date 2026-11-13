# Changelog

All notable changes to this project are documented here. Format loosely
follows [Keep a Changelog](https://keepachangelog.com/).

## Unreleased

### Added

- README, `.env.example`, `.dockerignore` and `renovate.json`.
- Container workflow: tests, a multi-arch (`linux/amd64`, `linux/arm64`) image
  pushed to GHCR, and a mirror to Docker Hub.
- Docker Hub overview (`docs/dockerhub.md`), synced by the container workflow.
- Slack notification workflow for build results.

### Changed

- Renamed `Dockerfile` to `Containerfile` and `docker-compose.yml` to
  `compose.yml`. The compose file now uses the published image.
- Base images updated to `golang:1.27-alpine` and `alpine:3.24`.
- Docker Hub tags become a single multi-arch `latest` (plus `sha-*` and
  version tags), replacing the separate `latest.amd64` and `latest.arm64`.
- Formatted `maxmind.go` with `gofmt`.

### Removed

- The MaxMind GeoLite2 database files under `data/` are no longer tracked in
  git. The directory stays git-ignored; the service downloads the database at
  run time with your license key.
