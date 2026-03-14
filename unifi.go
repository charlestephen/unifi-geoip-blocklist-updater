package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// FirewallGroup represents a UniFi firewall address group.
type FirewallGroup struct {
	ID           string   `json:"_id,omitempty"`
	Name         string   `json:"name"`
	GroupType    string   `json:"group_type"`
	GroupMembers []string `json:"group_members"`
	SiteID       string   `json:"site_id,omitempty"`
}

// UniFiResponse wraps the standard UniFi API response.
type UniFiResponse struct {
	Meta struct {
		RC  string `json:"rc"`
		Msg string `json:"msg,omitempty"`
	} `json:"meta"`
	Data json.RawMessage `json:"data"`
}

// UniFiClient handles communication with the UniFi controller.
type UniFiClient struct {
	baseURL    string
	apiKey     string
	site       string
	httpClient *http.Client
}

// NewUniFiClient creates a new UniFi API client.
func NewUniFiClient(baseURL, apiKey, site string, insecure bool) *UniFiClient {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}
	return &UniFiClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		site:    site,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

func (c *UniFiClient) do(method, path string, body interface{}) (*UniFiResponse, error) {
	url := fmt.Sprintf("%s/proxy/network/api/s/%s/%s", c.baseURL, c.site, path)

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, url, string(respBody))
	}

	var result UniFiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if result.Meta.RC != "ok" {
		return nil, fmt.Errorf("API error: %s", result.Meta.Msg)
	}

	return &result, nil
}

// ListFirewallGroups returns all firewall groups from the controller.
func (c *UniFiClient) ListFirewallGroups() ([]FirewallGroup, error) {
	resp, err := c.do("GET", "rest/firewallgroup", nil)
	if err != nil {
		return nil, err
	}

	var groups []FirewallGroup
	if err := json.Unmarshal(resp.Data, &groups); err != nil {
		return nil, fmt.Errorf("decoding firewall groups: %w", err)
	}
	return groups, nil
}

// CreateFirewallGroup creates a new firewall address group.
func (c *UniFiClient) CreateFirewallGroup(group FirewallGroup) (*FirewallGroup, error) {
	resp, err := c.do("POST", "rest/firewallgroup", group)
	if err != nil {
		return nil, err
	}

	var created []FirewallGroup
	if err := json.Unmarshal(resp.Data, &created); err != nil {
		return nil, fmt.Errorf("decoding created group: %w", err)
	}
	if len(created) == 0 {
		return nil, fmt.Errorf("no group returned after creation")
	}
	return &created[0], nil
}

// UpdateFirewallGroup updates an existing firewall address group.
func (c *UniFiClient) UpdateFirewallGroup(group FirewallGroup) error {
	path := fmt.Sprintf("rest/firewallgroup/%s", group.ID)
	_, err := c.do("PUT", path, group)
	return err
}

// DeleteFirewallGroup removes a firewall address group by ID.
func (c *UniFiClient) DeleteFirewallGroup(id string) error {
	path := fmt.Sprintf("rest/firewallgroup/%s", id)
	_, err := c.do("DELETE", path, nil)
	return err
}

// SyncFirewallGroups creates or updates firewall groups to match the desired chunks.
// Groups are named "{prefix}0", "{prefix}1", etc.
// groupType should be "address-group" for IPv4 or "ipv6-address-group" for IPv6.
// Existing groups matching the prefix are updated in-place; extra ones are deleted.
func (c *UniFiClient) SyncFirewallGroups(prefix string, groupType string, chunks [][]string) error {
	existing, err := c.ListFirewallGroups()
	if err != nil {
		return fmt.Errorf("listing existing groups: %w", err)
	}

	// Index existing groups by name
	groupsByName := make(map[string]FirewallGroup)
	for _, g := range existing {
		if strings.HasPrefix(g.Name, prefix) {
			groupsByName[g.Name] = g
		}
	}

	// Create or update groups for each chunk
	for i, chunk := range chunks {
		name := fmt.Sprintf("%s%d", prefix, i)
		if existing, ok := groupsByName[name]; ok {
			log.Printf("updating group %s (%d entries)", name, len(chunk))
			existing.GroupMembers = chunk
			if err := c.UpdateFirewallGroup(existing); err != nil {
				return fmt.Errorf("updating group %s: %w", name, err)
			}
			delete(groupsByName, name)
		} else {
			log.Printf("creating group %s (%d entries)", name, len(chunk))
			_, err := c.CreateFirewallGroup(FirewallGroup{
				Name:         name,
				GroupType:    groupType,
				GroupMembers: chunk,
			})
			if err != nil {
				return fmt.Errorf("creating group %s: %w", name, err)
			}
		}
	}

	// Delete leftover groups that are no longer needed
	for name, g := range groupsByName {
		log.Printf("deleting stale group %s", name)
		if err := c.DeleteFirewallGroup(g.ID); err != nil {
			return fmt.Errorf("deleting group %s: %w", name, err)
		}
	}

	return nil
}
