# unifi-geoip-blocklist-updater

A small Go service that keeps UniFi **firewall address groups** in sync with GeoIP country data. It downloads MaxMind's GeoLite2 Country database, collects the IP ranges for the countries you list, and writes them to your UniFi controller as groups you can use in firewall rules.

What it does on each run:

1. Reads the country codes from your countries file (re-read every cycle, so edits apply without a restart).
2. Downloads the GeoLite2 Country CSV from MaxMind and extracts the IPv4 and IPv6 ranges for those countries.
3. Removes any addresses listed in an optional exclude file (a whitelist).
4. Merges adjacent ranges into larger ones to cut the group count (on by default).
5. Splits the result into groups of at most `CHUNK_SIZE` entries and creates or updates them on the controller, named `GeoBlock0`, `GeoBlock1`, ... for IPv4 and `GeoBlockV6_0`, ... for IPv6.
6. Sleeps for `INTERVAL`, then repeats.

You still need to create the firewall rules that use these groups in UniFi.

- Source: https://github.com/charlestephen/unifi-geoip-blocklist-updater
- Issues: https://github.com/charlestephen/unifi-geoip-blocklist-updater/issues

## Requirements

- A **MaxMind license key** for GeoLite2 (free account at https://www.maxmind.com).
- A **UniFi API key** with access to the Network application, and the controller's URL.

## Tags

The image is currently published per architecture:

| Tag | Architecture |
|---|---|
| `latest.amd64` | x86-64 |
| `latest.arm64` | ARM64 |

Pick the tag that matches your host. There is no combined `latest` tag.

## Quick start

Create a working directory with a countries file and an `.env` file. Every value below is a placeholder to replace.

`config/countries.txt` lists the ISO 3166-1 alpha-2 country codes to put in the groups, one per line. Lines starting with `#` are comments:

```
# Countries to include in the firewall groups
CN
RU
```

`config/excludes.txt` is optional. It lists IPs or CIDRs to remove from the groups, one per line:

```
# Addresses to leave out even if they fall in a listed country
203.0.113.10
198.51.100.0/24
```

`.env`:

```ini
MAXMIND_LICENSE_KEY=your-maxmind-license-key
UNIFI_URL=https://192.168.1.1
UNIFI_API_KEY=your-unifi-api-key
UNIFI_SITE=default
EXCLUDE_FILE=/config/excludes.txt
INTERVAL=24h
```

Try a dry run first. It downloads and parses the data and prints a summary, without touching your controller:

```bash
docker run --rm \
  -e MAXMIND_LICENSE_KEY=your-maxmind-license-key \
  -v "$PWD/config:/config:ro" \
  -v geoip-data:/data \
  charlestephen/unifi-geoip-blocklist-updater:latest.amd64 -dry-run
```

Then run it for real:

```bash
docker run -d --name unifi-geoip-blocklist-updater --restart unless-stopped \
  --env-file .env \
  -v "$PWD/config:/config:ro" \
  -v geoip-data:/data \
  charlestephen/unifi-geoip-blocklist-updater:latest.amd64
```

Use `latest.arm64` on an ARM host. Watch the first cycle:

```bash
docker logs -f unifi-geoip-blocklist-updater
```

To update:

```bash
docker pull charlestephen/unifi-geoip-blocklist-updater:latest.amd64
docker rm -f unifi-geoip-blocklist-updater
# then run the docker run command above again
```

### Docker Compose

```yaml
services:
  geoip-updater:
    image: charlestephen/unifi-geoip-blocklist-updater:latest.amd64
    container_name: unifi-geoip-blocklist-updater
    restart: unless-stopped
    env_file:
      - .env
    volumes:
      - ./config:/config:ro
      - geoip-data:/data

volumes:
  geoip-data:
```

```bash
docker compose up -d
docker compose logs -f
```

## Volumes

| Path | Purpose |
|---|---|
| `/config` | Your `countries.txt` and optional `excludes.txt`. Read-only is fine. |
| `/data` | Cache of the MaxMind download, so it isn't fetched again on every start. Use a named volume. |

## Configuration

Each setting is an environment variable. A command-line flag, where one exists, takes precedence over the variable.

| Variable | Flag | Default | Description |
|---|---|---|---|
| `MAXMIND_LICENSE_KEY` | `-maxmind-key` | none | MaxMind license key. Required. |
| `UNIFI_URL` | `-url` | none | Controller URL, for example `https://192.168.1.1`. Required unless dry run. |
| `UNIFI_API_KEY` | `-api-key` | none | UniFi API key. Required unless dry run. |
| `UNIFI_SITE` | `-site` | `default` | UniFi site name. |
| `COUNTRIES_FILE` | `-countries-file` | `/config/countries.txt` | Country codes to include. |
| `EXCLUDE_FILE` | `-exclude-file` | none | IPs and CIDRs to leave out. |
| `DATA_DIR` | `-data-dir` | `/data` | Where the MaxMind download is cached. |
| `GROUP_PREFIX` | `-prefix` | `GeoBlock` | Prefix for group names. |
| `CHUNK_SIZE` | `-chunk-size` | `10000` | Maximum entries per group. |
| `INTERVAL` | `-interval` | `24h` | Time between update cycles, for example `12h`. |
| `AGGREGATE` | `-aggregate` | `true` | Merge adjacent ranges into larger ones. |
| `DRY_RUN` | `-dry-run` | `false` | Download and parse, but don't change the controller. The container exits after one run. |
| `INSECURE` | `-insecure` | `false` | Skip TLS certificate verification, for a controller with a self-signed certificate. |

## Notes

- The groups are created in the UniFi Network application under the site you set. Use them as the source or destination in your firewall rules.
- If the controller has a self-signed certificate, setting `INSECURE=true` disables certificate checks for that connection. Prefer a trusted certificate where you can.
- Keep your `.env` file private, since it holds your API keys.

## License

See https://github.com/charlestephen/unifi-geoip-blocklist-updater/blob/main/LICENSE
