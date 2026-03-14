package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	cfg := ParseConfig()

	log.Println("GeoIP Blocklist Updater starting")
	log.Printf("  Countries file: %s", cfg.CountriesFile)
	log.Printf("  Data dir: %s", cfg.DataDir)
	log.Printf("  Prefix: %s, Chunk size: %d", cfg.GroupPrefix, cfg.ChunkSize)
	log.Printf("  Interval: %s, Dry-run: %v", cfg.Interval, cfg.DryRun)
	if !cfg.DryRun {
		log.Printf("  UniFi: %s (site: %s)", cfg.ControllerURL, cfg.Site)
	}

	// Run immediately on startup, then loop
	for {
		if err := runOnce(cfg); err != nil {
			log.Printf("ERROR: %v", err)
		}

		if cfg.DryRun {
			log.Println("dry-run mode — exiting after single run")
			return
		}

		log.Printf("sleeping %s until next run...", cfg.Interval)
		time.Sleep(cfg.Interval)
	}
}

func runOnce(cfg Config) error {
	// 1. Load countries (re-read each cycle for live config changes)
	countries, err := LoadCountries(cfg.CountriesFile)
	if err != nil {
		return fmt.Errorf("loading countries: %w", err)
	}
	log.Printf("targeting %d countries: %v", len(countries), countries)

	// 2. Download and parse MaxMind data
	ipv4, ipv6, err := FetchGeoIPCIDRs(cfg.MaxMindKey, cfg.DataDir, countries)
	if err != nil {
		return fmt.Errorf("fetching GeoIP CIDRs: %w", err)
	}
	log.Printf("collected %d IPv4 CIDRs, %d IPv6 CIDRs", len(ipv4), len(ipv6))

	// 3. Apply excludes
	if cfg.ExcludeFile != "" {
		excludes, err := LoadExcludes(cfg.ExcludeFile)
		if err != nil {
			return fmt.Errorf("loading excludes: %w", err)
		}
		log.Printf("loaded %d exclude entries (%d IPs, %d CIDRs)",
			excludes.Count(), len(excludes.ips), len(excludes.cidrs))
		ipv4 = excludes.Filter(ipv4)
		ipv6 = excludes.Filter(ipv6)
		log.Printf("after excludes: %d IPv4, %d IPv6", len(ipv4), len(ipv6))
	}

	// 4. Sort and chunk
	sortIPs(ipv4)
	sortIPs(ipv6)
	v4Chunks := chunkSlice(ipv4, cfg.ChunkSize)
	v6Chunks := chunkSlice(ipv6, cfg.ChunkSize)
	log.Printf("chunked into %d IPv4 group(s), %d IPv6 group(s)", len(v4Chunks), len(v6Chunks))

	// 5. Dry-run: print summary and return
	if cfg.DryRun {
		for i, chunk := range v4Chunks {
			fmt.Printf("%s%d: %d entries (first: %s, last: %s)\n",
				cfg.GroupPrefix, i, len(chunk), chunk[0], chunk[len(chunk)-1])
		}
		for i, chunk := range v6Chunks {
			fmt.Printf("%sV6_%d: %d entries (first: %s, last: %s)\n",
				cfg.GroupPrefix, i, len(chunk), chunk[0], chunk[len(chunk)-1])
		}
		log.Println("dry-run complete, no changes pushed")
		return nil
	}

	// 6. Sync to UniFi
	client := NewUniFiClient(cfg.ControllerURL, cfg.APIKey, cfg.Site, cfg.Insecure)

	if len(v4Chunks) > 0 {
		if err := client.SyncFirewallGroups(cfg.GroupPrefix, "address-group", v4Chunks); err != nil {
			return fmt.Errorf("syncing IPv4 groups: %w", err)
		}
	}

	v6Prefix := cfg.GroupPrefix + "V6_"
	if len(v6Chunks) > 0 {
		if err := client.SyncFirewallGroups(v6Prefix, "ipv6-address-group", v6Chunks); err != nil {
			return fmt.Errorf("syncing IPv6 groups: %w", err)
		}
	}

	log.Println("sync complete — all firewall groups updated")
	return nil
}
