package main

import (
	"fmt"
	"testing"
)

func TestAggregateCIDRs_SiblingMerge(t *testing.T) {
	// Two /24s that should merge into one /23
	input := []string{"10.0.0.0/24", "10.0.1.0/24"}
	got := AggregateCIDRs(input)
	want := []string{"10.0.0.0/23"}
	assertSliceEqual(t, "sibling merge", want, got)
}

func TestAggregateCIDRs_FourBlocksToOne(t *testing.T) {
	input := []string{
		"192.168.0.0/24",
		"192.168.1.0/24",
		"192.168.2.0/24",
		"192.168.3.0/24",
	}
	got := AggregateCIDRs(input)
	want := []string{"192.168.0.0/22"}
	assertSliceEqual(t, "four /24s → /22", want, got)
}

func TestAggregateCIDRs_OverlapRemoval(t *testing.T) {
	// /16 contains the /24 — should collapse to just the /16
	input := []string{"10.0.0.0/16", "10.0.1.0/24"}
	got := AggregateCIDRs(input)
	want := []string{"10.0.0.0/16"}
	assertSliceEqual(t, "overlap removal", want, got)
}

func TestAggregateCIDRs_PlainIPs(t *testing.T) {
	// Two adjacent single IPs should merge into a /31,
	// but /31 should stay as CIDR (not plain IP)
	input := []string{"10.0.0.0", "10.0.0.1"}
	got := AggregateCIDRs(input)
	want := []string{"10.0.0.0/31"}
	assertSliceEqual(t, "two IPs → /31", want, got)
}

func TestAggregateCIDRs_NoMerge(t *testing.T) {
	// Non-adjacent blocks should not merge
	input := []string{"10.0.0.0/24", "10.0.2.0/24"}
	got := AggregateCIDRs(input)
	want := []string{"10.0.0.0/24", "10.0.2.0/24"}
	assertSliceEqual(t, "non-adjacent", want, got)
}

func TestAggregateCIDRs_Empty(t *testing.T) {
	got := AggregateCIDRs(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestAggregateCIDRs_SingleHost(t *testing.T) {
	// A lone IP should stay as plain IP (no /32)
	input := []string{"8.8.8.8"}
	got := AggregateCIDRs(input)
	want := []string{"8.8.8.8"}
	assertSliceEqual(t, "single host", want, got)
}

func TestAggregateCIDRs_IPv6(t *testing.T) {
	input := []string{"2001:db8::/48", "2001:db8:1::/48"}
	got := AggregateCIDRs(input)
	want := []string{"2001:db8::/47"}
	assertSliceEqual(t, "IPv6 sibling merge", want, got)
}

func TestAggregateCIDRs_MixedSizes(t *testing.T) {
	// /23 already contains both /24s — should not duplicate
	input := []string{
		"10.0.0.0/24",
		"10.0.1.0/24",
		"10.0.0.0/23",
	}
	got := AggregateCIDRs(input)
	want := []string{"10.0.0.0/23"}
	assertSliceEqual(t, "mixed sizes", want, got)
}

func assertSliceEqual(t *testing.T, label string, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("[%s] length mismatch: want %d, got %d\n  want: %v\n  got:  %v",
			label, len(want), len(got), want, got)
		return
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("[%s] index %d: want %q, got %q\n  full want: %v\n  full got:  %v",
				label, i, want[i], got[i], want, got)
			return
		}
	}
	fmt.Printf("[%s] PASS: %v → %v\n", label, want, got)
}
