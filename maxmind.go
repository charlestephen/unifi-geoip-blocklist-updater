package main

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	maxmindDownloadURL = "https://download.maxmind.com/app/geoip_download"
	editionID          = "GeoLite2-Country-CSV"
	zipFilename        = "GeoLite2-Country-CSV.zip"
)

// FetchGeoIPCIDRs downloads the MaxMind GeoLite2-Country CSV database (if needed),
// extracts it, and returns IPv4 and IPv6 CIDRs for the specified countries.
func FetchGeoIPCIDRs(licenseKey, dataDir string, countries []string) (ipv4, ipv6 []string, err error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating data dir: %w", err)
	}

	zipPath := filepath.Join(dataDir, zipFilename)

	// Download (with If-Modified-Since caching)
	updated, err := downloadMaxMindCSV(licenseKey, zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("downloading MaxMind DB: %w", err)
	}
	if updated {
		log.Println("MaxMind database downloaded/updated")
	} else {
		log.Println("MaxMind database is up to date (304 Not Modified)")
	}

	// Extract CSV files from ZIP
	csvDir := filepath.Join(dataDir, "csv")
	if err := extractCSVFiles(zipPath, csvDir); err != nil {
		return nil, nil, fmt.Errorf("extracting CSV: %w", err)
	}

	// Build geoname_id → country_iso_code map
	locations, err := loadLocations(csvDir)
	if err != nil {
		return nil, nil, fmt.Errorf("loading locations: %w", err)
	}
	log.Printf("loaded %d location entries", len(locations))

	// Build country filter set
	wantCountries := make(map[string]bool, len(countries))
	for _, c := range countries {
		wantCountries[c] = true
	}

	// Parse blocks
	ipv4, err = loadBlocks(csvDir, "GeoLite2-Country-Blocks-IPv4.csv", locations, wantCountries)
	if err != nil {
		return nil, nil, fmt.Errorf("loading IPv4 blocks: %w", err)
	}

	ipv6, err = loadBlocks(csvDir, "GeoLite2-Country-Blocks-IPv6.csv", locations, wantCountries)
	if err != nil {
		return nil, nil, fmt.Errorf("loading IPv6 blocks: %w", err)
	}

	return ipv4, ipv6, nil
}

// downloadMaxMindCSV downloads the GeoLite2-Country-CSV ZIP file.
// Uses If-Modified-Since to avoid redundant downloads.
// Returns true if the file was downloaded/updated, false if 304 Not Modified.
func downloadMaxMindCSV(licenseKey, destPath string) (bool, error) {
	url := fmt.Sprintf("%s?edition_id=%s&license_key=%s&suffix=zip",
		maxmindDownloadURL, editionID, licenseKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	// Use If-Modified-Since if we have an existing file
	if info, err := os.Stat(destPath); err == nil {
		req.Header.Set("If-Modified-Since", info.ModTime().UTC().Format(http.TimeFormat))
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d from MaxMind", resp.StatusCode)
	}

	// Write to temp file then rename for atomicity
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return false, fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return false, fmt.Errorf("writing download: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return false, fmt.Errorf("renaming temp file: %w", err)
	}

	return true, nil
}

// extractCSVFiles extracts the relevant CSV files from the MaxMind ZIP archive.
// The ZIP contains files inside a date-stamped subdirectory, so we flatten them.
func extractCSVFiles(zipPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening ZIP: %w", err)
	}
	defer r.Close()

	// Files we need (by basename)
	wanted := map[string]bool{
		"GeoLite2-Country-Blocks-IPv4.csv":  true,
		"GeoLite2-Country-Blocks-IPv6.csv":  true,
		"GeoLite2-Country-Locations-en.csv": true,
	}

	extracted := 0
	for _, f := range r.File {
		base := filepath.Base(f.Name)
		if !wanted[base] {
			continue
		}

		destPath := filepath.Join(destDir, base)
		if err := extractZipFile(f, destPath); err != nil {
			return fmt.Errorf("extracting %s: %w", base, err)
		}
		extracted++
	}

	if extracted != len(wanted) {
		return fmt.Errorf("expected %d CSV files, found %d in ZIP", len(wanted), extracted)
	}

	return nil
}

func extractZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// loadLocations parses GeoLite2-Country-Locations-en.csv and returns
// a map of geoname_id → country_iso_code.
// CSV columns: geoname_id, locale_code, continent_code, continent_name,
//
//	country_iso_code, country_name, is_in_european_union
func loadLocations(csvDir string) (map[int]string, error) {
	path := filepath.Join(csvDir, "GeoLite2-Country-Locations-en.csv")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)

	// Skip header
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	locations := make(map[int]string)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV: %w", err)
		}

		if len(record) < 5 {
			continue
		}

		geonameID, err := strconv.Atoi(record[0])
		if err != nil {
			continue // skip invalid rows
		}

		countryCode := strings.TrimSpace(record[4])
		if countryCode != "" {
			locations[geonameID] = countryCode
		}
	}

	return locations, nil
}

// loadBlocks parses a GeoLite2-Country-Blocks CSV file and returns CIDRs
// belonging to the specified countries.
// CSV columns: network, geoname_id, registered_country_geoname_id,
//
//	represented_country_geoname_id, is_anonymous_proxy,
//	is_satellite_provider, is_anycast
func loadBlocks(csvDir, filename string, locations map[int]string, wantCountries map[string]bool) ([]string, error) {
	path := filepath.Join(csvDir, filename)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)

	// Skip header
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	var cidrs []string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV: %w", err)
		}

		if len(record) < 3 {
			continue
		}

		network := strings.TrimSpace(record[0])

		// Try geoname_id (col 1), fall back to registered_country_geoname_id (col 2)
		countryCode := resolveCountry(record[1], record[2], locations)
		if countryCode == "" || !wantCountries[countryCode] {
			continue
		}

		// Validate CIDR
		_, ipNet, err := net.ParseCIDR(network)
		if err != nil {
			continue
		}

		// UniFi rejects /32 and /128 — use plain IP for single-host entries
		ones, bits := ipNet.Mask.Size()
		if ones == bits {
			cidrs = append(cidrs, ipNet.IP.String())
		} else {
			cidrs = append(cidrs, network)
		}
	}

	return cidrs, nil
}

// resolveCountry looks up the country code for a block row.
// Tries the primary geoname_id first, falls back to registered_country_geoname_id.
func resolveCountry(primaryID, registeredID string, locations map[int]string) string {
	if id, err := strconv.Atoi(strings.TrimSpace(primaryID)); err == nil {
		if code, ok := locations[id]; ok {
			return code
		}
	}
	if id, err := strconv.Atoi(strings.TrimSpace(registeredID)); err == nil {
		if code, ok := locations[id]; ok {
			return code
		}
	}
	return ""
}
