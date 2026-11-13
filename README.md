# unifi-geoip-blocklist-updater

A small Go service that keeps UniFi **firewall address groups** in sync with
GeoIP country data. It downloads MaxMind's GeoLite2 Country database, collects
the IP ranges for the countries you list, and writes them to your UniFi
controller as groups you can use in firewall rules.

Each run:

1. Reads the country codes from the countries file (re-read every cycle).
2. Downloads the GeoLite2 Country CSV from MaxMind and extracts the IPv4 and
   IPv6 ranges for those countries.
3. Removes addresses in the optional exclude file.
4. Merges adjacent ranges into larger ones (on by default).
5. Splits the result into groups of at most `CHUNK_SIZE` entries and creates or
   updates them on the controller: `GeoBlock0`, `GeoBlock1`, ... for IPv4 and
   `GeoBlockV6_0`, ... for IPv6.
6. Sleeps for `INTERVAL`, then repeats.

You still create the firewall rules that use the groups in UniFi.

## Requirements

- A MaxMind license key for GeoLite2 (free account at https://www.maxmind.com).
- A UniFi API key and the controller URL.

## Run with Docker

Create `config/countries.txt` (ISO 3166-1 alpha-2 codes, one per line, `#`
for comments), an optional `config/excludes.txt` (IPs or CIDRs, one per line),
and a `.env` file based on [`.env.example`](.env.example).

```bash
docker run -d --name unifi-geoip-blocklist-updater --restart unless-stopped \
  --env-file .env \
  -v "$PWD/config:/config:ro" \
  -v geoip-data:/data \
  ghcr.io/charlestephen/unifi-geoip-blocklist-updater:latest
```

Or with Compose:

```bash
docker compose up -d
```

The image is also published as `docker.io/charlestephen/unifi-geoip-blocklist-updater`
for `linux/amd64` and `linux/arm64`. Run it once with `-dry-run` to download
and parse the data and print a summary without changing the controller.

## Configuration

Each setting is an environment variable. A command-line flag, where one
exists, takes precedence.

| Variable | Flag | Default | Description |
|---|---|---|---|
| `MAXMIND_LICENSE_KEY` | `-maxmind-key` | none | MaxMind license key. Required. |
| `UNIFI_URL` | `-url` | none | Controller URL. Required unless dry run. |
| `UNIFI_API_KEY` | `-api-key` | none | UniFi API key. Required unless dry run. |
| `UNIFI_SITE` | `-site` | `default` | UniFi site name. |
| `COUNTRIES_FILE` | `-countries-file` | `/config/countries.txt` | Country codes to include. |
| `EXCLUDE_FILE` | `-exclude-file` | none | IPs and CIDRs to leave out. |
| `DATA_DIR` | `-data-dir` | `/data` | Cache for the MaxMind download. |
| `GROUP_PREFIX` | `-prefix` | `GeoBlock` | Prefix for group names. |
| `CHUNK_SIZE` | `-chunk-size` | `10000` | Maximum entries per group. |
| `INTERVAL` | `-interval` | `24h` | Time between update cycles. |
| `AGGREGATE` | `-aggregate` | `true` | Merge adjacent ranges. |
| `DRY_RUN` | `-dry-run` | `false` | Parse only, then exit. |
| `INSECURE` | `-insecure` | `false` | Skip TLS verification for the controller. |

## Build from source

```bash
docker build -f Containerfile -t unifi-geoip-blocklist-updater .
go test ./...
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## Sponsor

[Buy Me a Coffee](https://buymeacoffee.com/charlestephen)

## License

See [LICENSE](LICENSE).
