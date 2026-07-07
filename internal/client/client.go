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
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const DefaultBaseURL = "https://fornex.com/api"

// ErrEntryNotFound is returned by GetEntry when the domain has no entry with
// the requested ID. Callers use errors.Is to distinguish a record deleted
// out-of-band from a transport/API failure.
var ErrEntryNotFound = errors.New("entry not found")

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *retryablehttp.Client

	// The Fornex DNS list endpoints are slow (~2.5s per domain-list page,
	// ~1s per per-domain entry_set fetch). A single Terraform run can call
	// GetDomain / GetEntry many times for the same names, so the client
	// memoizes results for its lifetime. Writes invalidate the matching
	// entry to keep state consistent within one run.
	//
	// `domainsByName` is a value-only cache: a hit is trustworthy, but a
	// miss is NOT proof-of-absence. The Fornex API's `?q=` filter is fuzzy
	// (e.g. `q=infradim.com` and `q=itdept.team` return overlapping but
	// distinct ~12–25-domain sets), so there's no single walk that
	// enumerates the user's account exhaustively. Every miss must walk
	// `?q=<name>` and merge its results in.
	domainsMu       sync.Mutex
	domainsByName   map[string]Domain
	entriesMu       sync.Mutex
	entriesByDomain map[string][]Entry
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

// paginatedDomains matches the DRF-style envelope returned by
// `GET /dns/domain/`: {"count": N, "next": "<absolute url or null>",
// "previous": ..., "results": [...]}. The provider walks `next` until it is
// empty.
type paginatedDomains struct {
	Next    string   `json:"next"`
	Results []Domain `json:"results"`
}

// listDomainsPage fetches a single page. `path` is either a relative API path
// (e.g. "/dns/domain/?q=foo") or an absolute URL taken verbatim from the
// previous page's `next` field.
func (c *Client) listDomainsPage(ctx context.Context, path string) (*paginatedDomains, error) {
	path = strings.TrimPrefix(path, c.BaseURL)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var page paginatedDomains
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Domain methods
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	var domains []Domain
	next := "/dns/domain/"
	for next != "" {
		page, err := c.listDomainsPage(ctx, next)
		if err != nil {
			return nil, err
		}
		domains = append(domains, page.Results...)
		next = page.Next
	}
	return domains, nil
}

// createDomainStubIP is the placeholder IP sent in the POST /dns/domain/ body.
// Fornex requires `ip` and uses it to seed a default zone template (A records
// for "", "*", "www", "mail" plus MX "" -> "mail"), but the provider wipes
// those auto-created records immediately after Create returns — so the IP
// itself never reaches anyone's resolver. Hardcoding it keeps the resource
// schema free of an attribute whose value has no observable effect.
const createDomainStubIP = "127.0.0.1"

func (c *Client) CreateDomain(ctx context.Context, name string) (*Domain, error) {
	data, err := json.Marshal(DomainRequest{Name: name, IP: createDomainStubIP})
	if err != nil {
		return nil, err
	}

	body, err := c.doRequest(ctx, "POST", "/dns/domain/", data)
	if err != nil {
		return nil, err
	}

	c.invalidateDomain()

	var domain Domain
	err = json.Unmarshal(body, &domain)
	return &domain, err
}

func (c *Client) DeleteDomain(ctx context.Context, name string) error {
	_, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/dns/domain/%s/", name), nil)
	if err == nil {
		c.invalidateDomain()
	}
	return err
}

func (c *Client) GetDomain(ctx context.Context, name string) (*Domain, error) {
	// Fast path: cache hit. Don't hold the lock during the HTTP walk so
	// parallel GetDomain calls for different names can run concurrently.
	c.domainsMu.Lock()
	if d, ok := c.domainsByName[name]; ok {
		c.domainsMu.Unlock()
		return &d, nil
	}
	c.domainsMu.Unlock()

	next := fmt.Sprintf("/dns/domain/?q=%s", url.QueryEscape(name))
	var collected []Domain
	for next != "" {
		page, err := c.listDomainsPage(ctx, next)
		if err != nil {
			return nil, err
		}
		collected = append(collected, page.Results...)
		next = page.Next
	}

	c.domainsMu.Lock()
	if c.domainsByName == nil {
		c.domainsByName = make(map[string]Domain, len(collected))
	}
	for _, d := range collected {
		c.domainsByName[d.Name] = d
	}
	found, ok := c.domainsByName[name]
	c.domainsMu.Unlock()

	if ok {
		return &found, nil
	}
	return nil, fmt.Errorf("domain %s not found", name)
}

// invalidateDomain drops the whole domain cache. Called after Create/Delete so
// the next GetDomain re-fetches.
func (c *Client) invalidateDomain() {
	c.domainsMu.Lock()
	c.domainsByName = nil
	c.domainsMu.Unlock()
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

	c.invalidateEntries(domainName)

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

	c.invalidateEntries(domainName)

	var updatedEntry Entry
	err = json.Unmarshal(body, &updatedEntry)
	return &updatedEntry, err
}

func (c *Client) DeleteEntry(ctx context.Context, domainName string, entryID int) error {
	_, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/dns/domain/%s/entry_set/%d/", domainName, entryID), nil)
	if err == nil {
		c.invalidateEntries(domainName)
	}
	return err
}

func (c *Client) GetEntry(ctx context.Context, domainName string, entryID int) (*Entry, error) {
	c.entriesMu.Lock()
	defer c.entriesMu.Unlock()

	entries, ok := c.entriesByDomain[domainName]
	if !ok {
		fetched, err := c.ListEntries(ctx, domainName)
		if err != nil {
			return nil, err
		}
		if c.entriesByDomain == nil {
			c.entriesByDomain = make(map[string][]Entry)
		}
		c.entriesByDomain[domainName] = fetched
		entries = fetched
	}

	for _, e := range entries {
		if e.ID == entryID {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("entry %d not found in domain %s: %w", entryID, domainName, ErrEntryNotFound)
}

// invalidateEntries drops the entry cache for a single domain. Called after any
// mutation so the next GetEntry re-fetches.
func (c *Client) invalidateEntries(domainName string) {
	c.entriesMu.Lock()
	delete(c.entriesByDomain, domainName)
	c.entriesMu.Unlock()
}
