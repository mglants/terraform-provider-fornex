package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const DefaultBaseURL = "https://fornex.com/api"

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *retryablehttp.Client
}

// DefaultTimeout is the per-request HTTP timeout used when the caller does not
// supply one. The Fornex `GET /dns/domain/` endpoint has been observed taking
// well over a minute for accounts with many domains, so the default leaves
// headroom for that.
const DefaultTimeout = 3 * time.Minute

func NewClient(apiKey string, baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	// Ensure no trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	rc := retryablehttp.NewClient()
	rc.HTTPClient.Timeout = timeout
	rc.RetryMax = 4
	rc.RetryWaitMin = 1 * time.Second
	rc.RetryWaitMax = 30 * time.Second
	rc.CheckRetry = checkRetryNoTimeoutRetry
	// Silence retryablehttp's stderr logger; the Terraform plugin host already
	// surfaces request errors via diagnostics.
	rc.Logger = nil

	return &Client{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: rc,
	}
}

// checkRetryNoTimeoutRetry behaves like retryablehttp.DefaultRetryPolicy except
// it does not retry when the per-request HTTPClient.Timeout fires. Retrying a
// genuinely-slow endpoint just multiplies wall time by RetryMax and hammers the
// upstream; the caller is better off seeing the timeout and raising it.
func checkRetryNoTimeoutRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if err != nil && ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
		return false, err
	}
	return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Api-Key %s", c.APIKey))
	req.Header.Set("Content-Type", "application/json")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("status: %d, body: %s", res.StatusCode, string(respBody))
	}

	return respBody, nil
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
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	body, err := c.doRequest(ctx, "GET", "/dns/domain/", nil)
	if err != nil {
		return nil, err
	}

	var domains []Domain
	err = json.Unmarshal(body, &domains)
	return domains, err
}

func (c *Client) CreateDomain(ctx context.Context, name, ip string) (*Domain, error) {
	data, err := json.Marshal(DomainRequest{Name: name, IP: ip})
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, "POST", "/dns/domain/", data)
	if err != nil {
		return nil, err
	}

	var domain Domain
	err = json.Unmarshal(body, &domain)
	return &domain, err
}

func (c *Client) DeleteDomain(ctx context.Context, name string) error {
	_, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/dns/domain/%s/", name), nil)
	return err
}

func (c *Client) GetDomain(ctx context.Context, name string) (*Domain, error) {
	body, err := c.doRequest(ctx, "GET", fmt.Sprintf("/dns/domain/?q=%s", url.QueryEscape(name)), nil)
	if err != nil {
		return nil, err
	}

	var domains []Domain
	if err := json.Unmarshal(body, &domains); err != nil {
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
func (c *Client) ListEntries(ctx context.Context, domainName string) ([]Entry, error) {
	body, err := c.doRequest(ctx, "GET", fmt.Sprintf("/dns/domain/%s/entry_set/", domainName), nil)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	err = json.Unmarshal(body, &entries)
	return entries, err
}

func (c *Client) CreateEntry(ctx context.Context, domainName string, entry Entry) (*Entry, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, "POST", fmt.Sprintf("/dns/domain/%s/entry_set/", domainName), data)
	if err != nil {
		return nil, err
	}

	var newEntry Entry
	err = json.Unmarshal(body, &newEntry)
	return &newEntry, err
}

func (c *Client) UpdateEntry(ctx context.Context, domainName string, entryID int, entry Entry) (*Entry, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, "PUT", fmt.Sprintf("/dns/domain/%s/entry_set/%d/", domainName, entryID), data)
	if err != nil {
		return nil, err
	}

	var updatedEntry Entry
	err = json.Unmarshal(body, &updatedEntry)
	return &updatedEntry, err
}

func (c *Client) DeleteEntry(ctx context.Context, domainName string, entryID int) error {
	_, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/dns/domain/%s/entry_set/%d/", domainName, entryID), nil)
	return err
}

func (c *Client) GetEntry(ctx context.Context, domainName string, entryID int) (*Entry, error) {
	entries, err := c.ListEntries(ctx, domainName)
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
