package main

import (
	"encoding/binary"
	"math/big"
	"net"
	"sort"
)

// cidrEntry represents a parsed CIDR or single IP for aggregation.
type cidrEntry struct {
	network *net.IPNet
	ip      net.IP // normalized to 4- or 16-byte form
	ones    int    // prefix length
	bits    int    // total bits (32 for IPv4, 128 for IPv6)
}

// AggregateCIDRs takes a sorted list of IP/CIDR strings and merges
// adjacent and overlapping blocks into the smallest set of supernets.
//
// For example:
//   - 1.0.0.0/24 + 1.0.1.0/24 → 1.0.0.0/23
//   - 10.0.0.1 + 10.0.0.2/31 → collapsed if adjacent
//
// This dramatically reduces the number of firewall group entries.
func AggregateCIDRs(entries []string) []string {
	if len(entries) == 0 {
		return entries
	}

	// Parse all entries into cidrEntry structs
	parsed := make([]cidrEntry, 0, len(entries))
	for _, e := range entries {
		ce, ok := parseCIDREntry(e)
		if !ok {
			continue
		}
		parsed = append(parsed, ce)
	}

	if len(parsed) == 0 {
		return nil
	}

	// Sort by network address, then by prefix length (shorter prefixes first)
	sort.Slice(parsed, func(i, j int) bool {
		cmp := ipToInt(parsed[i].ip).Cmp(ipToInt(parsed[j].ip))
		if cmp != 0 {
			return cmp < 0
		}
		return parsed[i].ones < parsed[j].ones
	})

	// Step 1: Remove overlapping entries (entries fully contained in a prior supernet)
	merged := removeOverlaps(parsed)

	// Step 2: Iteratively merge adjacent sibling networks until no more merges
	for {
		next := mergeSiblings(merged)
		if len(next) == len(merged) {
			break // no merges happened
		}
		merged = next
	}

	// Convert back to strings
	result := make([]string, 0, len(merged))
	for _, ce := range merged {
		ones, bits := ce.network.Mask.Size()
		if ones == bits {
			// Single host — UniFi wants plain IP, not /32 or /128
			result = append(result, ce.ip.String())
		} else {
			result = append(result, ce.network.String())
		}
	}
	return result
}

// parseCIDREntry parses an IP or CIDR string into a cidrEntry.
func parseCIDREntry(s string) (cidrEntry, bool) {
	// Try as CIDR first
	if _, ipNet, err := net.ParseCIDR(s); err == nil {
		ones, bits := ipNet.Mask.Size()
		ip := ipNet.IP
		if bits == 32 {
			ip = ip.To4()
		}
		return cidrEntry{
			network: ipNet,
			ip:      ip,
			ones:    ones,
			bits:    bits,
		}, true
	}

	// Try as plain IP
	ip := net.ParseIP(s)
	if ip == nil {
		return cidrEntry{}, false
	}

	if ip4 := ip.To4(); ip4 != nil {
		mask := net.CIDRMask(32, 32)
		return cidrEntry{
			network: &net.IPNet{IP: ip4, Mask: mask},
			ip:      ip4,
			ones:    32,
			bits:    32,
		}, true
	}

	mask := net.CIDRMask(128, 128)
	return cidrEntry{
		network: &net.IPNet{IP: ip, Mask: mask},
		ip:      ip,
		ones:    128,
		bits:    128,
	}, true
}

// removeOverlaps removes entries that are fully contained within a preceding supernet.
// Input must be sorted by IP then prefix length.
func removeOverlaps(entries []cidrEntry) []cidrEntry {
	if len(entries) == 0 {
		return entries
	}

	result := []cidrEntry{entries[0]}
	for i := 1; i < len(entries); i++ {
		prev := result[len(result)-1]
		curr := entries[i]

		// If current network is fully within previous, skip it
		if prev.network.Contains(curr.ip) && prev.ones <= curr.ones {
			lastIP := lastIPInNet(curr.network)
			if prev.network.Contains(lastIP) {
				continue
			}
		}
		result = append(result, curr)
	}
	return result
}

// mergeSiblings does one pass over sorted, non-overlapping entries and merges
// adjacent siblings (two networks that combine to form one supernet).
// E.g., 10.0.0.0/24 + 10.0.1.0/24 → 10.0.0.0/23
func mergeSiblings(entries []cidrEntry) []cidrEntry {
	if len(entries) < 2 {
		return entries
	}

	result := make([]cidrEntry, 0, len(entries))
	i := 0

	for i < len(entries) {
		if i+1 >= len(entries) {
			result = append(result, entries[i])
			i++
			continue
		}

		a := entries[i]
		b := entries[i+1]

		// Can only merge if same address family and same prefix length
		if a.bits == b.bits && a.ones == b.ones && a.ones > 0 {
			// Check if they're siblings: same parent supernet
			parentOnes := a.ones - 1
			parentMask := net.CIDRMask(parentOnes, a.bits)
			parentA := maskedIP(a.ip, parentMask)
			parentB := maskedIP(b.ip, parentMask)

			if parentA.Equal(parentB) {
				// Merge into parent
				parentNet := &net.IPNet{IP: parentA, Mask: parentMask}
				result = append(result, cidrEntry{
					network: parentNet,
					ip:      parentA,
					ones:    parentOnes,
					bits:    a.bits,
				})
				i += 2
				continue
			}
		}

		result = append(result, a)
		i++
	}

	return result
}

// maskedIP applies a mask to an IP and returns the network address.
func maskedIP(ip net.IP, mask net.IPMask) net.IP {
	result := make(net.IP, len(ip))
	for i := range ip {
		result[i] = ip[i] & mask[i]
	}
	return result
}

// lastIPInNet returns the last (broadcast) IP in a network.
func lastIPInNet(ipNet *net.IPNet) net.IP {
	ip := make(net.IP, len(ipNet.IP))
	for i := range ipNet.IP {
		ip[i] = ipNet.IP[i] | ^ipNet.Mask[i]
	}
	return ip
}

// ipToInt converts an IP to a big.Int for comparison (same approach as iputil.go).
func ipToInt(ip net.IP) *big.Int {
	if ip4 := ip.To4(); ip4 != nil {
		return big.NewInt(int64(binary.BigEndian.Uint32(ip4)))
	}
	return new(big.Int).SetBytes(ip.To16())
}
