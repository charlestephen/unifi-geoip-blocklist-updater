package main

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// ExcludeSet holds IPs and CIDRs that should be excluded from blocklists.
type ExcludeSet struct {
	ips   map[string]struct{} // exact IP matches (normalized)
	cidrs []*net.IPNet        // CIDR ranges to check containment
}

// LoadExcludes reads an exclude file (one IP or CIDR per line, # comments).
func LoadExcludes(path string) (*ExcludeSet, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	es := &ExcludeSet{ips: make(map[string]struct{})}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entry := parseExcludeLine(strings.TrimSpace(scanner.Text()))
		if entry == "" {
			continue
		}
		if _, cidr, err := net.ParseCIDR(entry); err == nil {
			es.cidrs = append(es.cidrs, cidr)
		} else {
			es.ips[entry] = struct{}{}
		}
	}
	return es, scanner.Err()
}

// Count returns total number of exclude entries.
func (es *ExcludeSet) Count() int {
	return len(es.ips) + len(es.cidrs)
}

// ShouldExclude returns true if the given IP or CIDR should be excluded.
func (es *ExcludeSet) ShouldExclude(entry string) bool {
	// Exact match
	if _, ok := es.ips[entry]; ok {
		return true
	}

	// Parse the entry to test against CIDR ranges
	if ip := net.ParseIP(entry); ip != nil {
		// Entry is a single IP — check if it falls in any excluded CIDR
		for _, cidr := range es.cidrs {
			if cidr.Contains(ip) {
				return true
			}
		}
	} else if _, entryCIDR, err := net.ParseCIDR(entry); err == nil {
		// Entry is a CIDR — exclude if an excluded CIDR fully contains it
		for _, cidr := range es.cidrs {
			if cidr.Contains(entryCIDR.IP) && cidrContains(cidr, entryCIDR) {
				return true
			}
		}
	}
	return false
}

// cidrContains returns true if outer fully contains inner.
func cidrContains(outer, inner *net.IPNet) bool {
	outerOnes, outerBits := outer.Mask.Size()
	innerOnes, innerBits := inner.Mask.Size()
	return outerBits == innerBits && outerOnes <= innerOnes && outer.Contains(inner.IP)
}

// Filter returns a new slice with excluded entries removed.
func (es *ExcludeSet) Filter(entries []string) []string {
	result := make([]string, 0, len(entries))
	for _, e := range entries {
		if !es.ShouldExclude(e) {
			result = append(result, e)
		}
	}
	return result
}

// parseExcludeLine extracts an IP or CIDR from a line, skipping comments.
func parseExcludeLine(line string) string {
	if line == "" || line[0] == '#' || line[0] == ';' {
		return ""
	}

	// Strip inline comments
	if idx := strings.IndexByte(line, '#'); idx > 0 {
		line = strings.TrimSpace(line[:idx])
	}

	// Validate as IP or CIDR
	if ip := net.ParseIP(line); ip != nil {
		return ip.String()
	}
	if _, cidr, err := net.ParseCIDR(line); err == nil {
		return cidr.String()
	}
	return ""
}
