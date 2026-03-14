package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Config holds the application configuration.
type Config struct {
	ControllerURL string
	APIKey        string
	Site          string
	MaxMindKey    string
	CountriesFile string
	ExcludeFile   string
	DataDir       string
	GroupPrefix   string
	ChunkSize     int
	Interval      time.Duration
	DryRun        bool
	Insecure      bool
}

// ParseConfig reads configuration from flags and environment variables.
// Flags take precedence; env vars are used as fallbacks when a flag is empty.
func ParseConfig() Config {
	var cfg Config
	var intervalStr string

	flag.StringVar(&cfg.ControllerURL, "url", "", "UniFi controller URL (e.g. https://192.168.1.1)")
	flag.StringVar(&cfg.APIKey, "api-key", "", "UniFi API key")
	flag.StringVar(&cfg.Site, "site", "default", "UniFi site name")
	flag.StringVar(&cfg.MaxMindKey, "maxmind-key", "", "MaxMind license key")
	flag.StringVar(&cfg.CountriesFile, "countries-file", "/config/countries.txt", "file with ISO country codes to block")
	flag.StringVar(&cfg.ExcludeFile, "exclude-file", "", "file containing IPs/CIDRs to exclude (whitelist)")
	flag.StringVar(&cfg.DataDir, "data-dir", "/data", "directory to cache MaxMind downloads")
	flag.StringVar(&cfg.GroupPrefix, "prefix", "GeoBlock", "firewall group name prefix")
	flag.IntVar(&cfg.ChunkSize, "chunk-size", 10000, "max entries per firewall group")
	flag.StringVar(&intervalStr, "interval", "24h", "time between update cycles (e.g. 24h, 12h)")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "download and parse but don't push to controller")
	flag.BoolVar(&cfg.Insecure, "insecure", false, "skip TLS certificate verification")
	flag.Parse()

	// Env var fallbacks
	envFallback(&cfg.ControllerURL, "UNIFI_URL")
	envFallback(&cfg.APIKey, "UNIFI_API_KEY")
	envFallback(&cfg.Site, "UNIFI_SITE")
	envFallback(&cfg.MaxMindKey, "MAXMIND_LICENSE_KEY")
	envFallback(&cfg.CountriesFile, "COUNTRIES_FILE")
	envFallback(&cfg.ExcludeFile, "EXCLUDE_FILE")
	envFallback(&cfg.DataDir, "DATA_DIR")
	envFallback(&cfg.GroupPrefix, "GROUP_PREFIX")
	envFallbackDuration(&cfg.Interval, intervalStr, "INTERVAL")
	envFallbackBool(&cfg.DryRun, "DRY_RUN")
	envFallbackBool(&cfg.Insecure, "INSECURE")

	if v := os.Getenv("CHUNK_SIZE"); v != "" && cfg.ChunkSize == 10000 {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.ChunkSize = n
		}
	}

	// Validate required fields
	if cfg.MaxMindKey == "" {
		log.Fatal("MaxMind license key required (-maxmind-key or MAXMIND_LICENSE_KEY env var)")
	}
	if !cfg.DryRun {
		if cfg.ControllerURL == "" {
			log.Fatal("controller URL required (-url or UNIFI_URL env var)")
		}
		if cfg.APIKey == "" {
			log.Fatal("API key required (-api-key or UNIFI_API_KEY env var)")
		}
	}

	cfg.ControllerURL = strings.TrimRight(cfg.ControllerURL, "/")

	return cfg
}

func envFallback(field *string, envKey string) {
	if *field == "" {
		*field = os.Getenv(envKey)
	}
}

func envFallbackDuration(field *time.Duration, flagVal, envKey string) {
	raw := flagVal
	if raw == "24h" {
		if v := os.Getenv(envKey); v != "" {
			raw = v
		}
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Fatalf("invalid interval %q: %v", raw, err)
	}
	*field = d
}

func envFallbackBool(field *bool, envKey string) {
	if !*field {
		v := strings.ToLower(os.Getenv(envKey))
		*field = v == "true" || v == "1" || v == "yes"
	}
}

// LoadCountries reads a file of ISO 3166-1 alpha-2 country codes.
// One code per line; lines starting with # are comments.
func LoadCountries(path string) ([]string, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]struct{})
	var codes []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		code := strings.ToUpper(strings.TrimSpace(line))

		// Validate: exactly 2 ASCII letters
		if len(code) != 2 || !unicode.IsLetter(rune(code[0])) || !unicode.IsLetter(rune(code[1])) {
			return nil, fmt.Errorf("invalid country code %q (expected 2-letter ISO 3166-1 alpha-2)", line)
		}

		if _, exists := seen[code]; !exists {
			seen[code] = struct{}{}
			codes = append(codes, code)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(codes) == 0 {
		return nil, fmt.Errorf("no country codes found in %s", path)
	}

	return codes, nil
}
