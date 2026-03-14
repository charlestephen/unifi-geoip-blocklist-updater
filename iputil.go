package main

import (
	"math/big"
	"net"
	"sort"
	"strings"
)

// isIPv6 returns true if the entry is an IPv6 address or CIDR.
func isIPv6(entry string) bool {
	return strings.Contains(entry, ":")
}

// sortIPs sorts IP/CIDR entries in numerical order using big.Int comparison.
func sortIPs(entries []string) {
	sort.Slice(entries, func(i, j int) bool {
		return ipSortKey(entries[i]).Cmp(ipSortKey(entries[j])) < 0
	})
}

// ipSortKey converts an IP or CIDR to a big.Int for numerical comparison.
func ipSortKey(entry string) *big.Int {
	var ip net.IP
	if _, cidr, err := net.ParseCIDR(entry); err == nil {
		ip = cidr.IP
	} else {
		ip = net.ParseIP(entry)
	}
	if ip == nil {
		return big.NewInt(0)
	}
	ip = ip.To16()
	return new(big.Int).SetBytes(ip)
}

// chunkSlice splits a slice into chunks of the specified size.
func chunkSlice(items []string, size int) [][]string {
	var chunks [][]string
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}
