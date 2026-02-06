package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultBaseURL = "https://fornex.com/api"

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(apiKey string, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	// Ensure no trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Api-Key %s", c.APIKey))
	req.Header.Set("Content-Type", "application/json")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, string(body))
	}

	return body, nil
}

// Domain types
type Domain struct {
	Name     string   `json:"name"`
	Created  string   `json:"created"`
	Updated  string   `json:"updated"`
	EntrySet []Entry  `json:"entry_set"`
	Tags     []string `json:"tags"`
}

type DomainRequest struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// Entry types
type Entry struct {
	ID       int    `json:"id,omitempty"`
	Host     string `json:"host"`
	Type     string `json:"type"`
	TTL      *int   `json:"ttl"`
	Value    string `json:"value"`
	Priority *int   `json:"prio,omitempty"`
}

// Domain methods
func (c *Client) ListDomains() ([]Domain, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/dns/domain/", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	var domains []Domain
	err = json.Unmarshal(body, &domains)
	return domains, err
}

func (c *Client) CreateDomain(name, ip string) (*Domain, error) {
	dr := DomainRequest{Name: name, IP: ip}
	data, err := json.Marshal(dr)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/dns/domain/", c.BaseURL), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	var domain Domain
	err = json.Unmarshal(body, &domain)
	return &domain, err
}

func (c *Client) DeleteDomain(name string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/dns/domain/%s/", c.BaseURL, name), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req)
	return err
}

func (c *Client) GetDomain(name string) (*Domain, error) {
	domains, err := c.ListDomains()
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		if d.Name == name {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("domain %s not found", name)
}

// Entry methods
func (c *Client) ListEntries(domainName string) ([]Entry, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/dns/domain/%s/entry_set/", c.BaseURL, domainName), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	err = json.Unmarshal(body, &entries)
	return entries, err
}

func (c *Client) CreateEntry(domainName string, entry Entry) (*Entry, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/dns/domain/%s/entry_set/", c.BaseURL, domainName), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	var newEntry Entry
	err = json.Unmarshal(body, &newEntry)
	return &newEntry, err
}

func (c *Client) UpdateEntry(domainName string, entryID int, entry Entry) (*Entry, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/dns/domain/%s/entry_set/%d/", c.BaseURL, domainName, entryID), bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}

	var updatedEntry Entry
	err = json.Unmarshal(body, &updatedEntry)
	return &updatedEntry, err
}

func (c *Client) DeleteEntry(domainName string, entryID int) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/dns/domain/%s/entry_set/%d/", c.BaseURL, domainName, entryID), nil)
	if err != nil {
		return err
	}

	_, err = c.doRequest(req)
	return err
}

func (c *Client) GetEntry(domainName string, entryID int) (*Entry, error) {
	entries, err := c.ListEntries(domainName)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.ID == entryID {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("entry %d not found in domain %s", entryID, domainName)
}
