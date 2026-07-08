// Package confluence is a thin client for the Confluence Cloud REST v2 API
// (/wiki/api/v2). It uses Basic auth and follows the v2 cursor-based pagination
// envelope (results + _links.next).
package confluence

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/dtonair/confluence-cli/config"
)

// APIBasePath is the Confluence Cloud REST v2 base path, appended to the site.
const APIBasePath = "/wiki/api/v2"

// Default pagination caps.
const (
	DefaultLimit    = 20
	DefaultPageLen  = 50
	DefaultMaxPages = 10
)

// excerptLimit bounds how much of an error response body is surfaced.
const excerptLimit = 500

// HTTPError is a normalized non-2xx response from Confluence.
type HTTPError struct {
	Method     string `json:"method"`
	URL        string `json:"url"`
	Status     int    `json:"status"`
	StatusText string `json:"statusText"`
	Excerpt    string `json:"excerpt"`
}

func (e *HTTPError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized:
		return "Confluence authentication failed. Check CONFLUENCE_EMAIL and CONFLUENCE_API_TOKEN."
	case http.StatusForbidden:
		return "Confluence authorization failed. Check API token scopes and space permissions."
	case http.StatusNotFound:
		return "Confluence resource not found. Check the site, space, and IDs."
	case http.StatusConflict:
		return "Confluence version conflict. The page changed since it was read; retry."
	case http.StatusTooManyRequests:
		return "Confluence rate limit reached. Retry later."
	default:
		return fmt.Sprintf("Confluence request failed with %d %s: %s", e.Status, e.StatusText, e.Excerpt)
	}
}

// EncodePathSegment percent-encodes a single path segment (slashes included).
func EncodePathSegment(value string) string {
	return url.PathEscape(value)
}

// Client talks to the Confluence Cloud API using Basic auth.
type Client struct {
	cfg  config.Config
	http *http.Client
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient injects a custom *http.Client (used in tests).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// NewClient builds a Client from config.
func NewClient(cfg config.Config, opts ...Option) *Client {
	c := &Client{cfg: cfg, http: http.DefaultClient}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// RequestOptions configures a single request.
type RequestOptions struct {
	Method string
	Body   any
}

func (c *Client) authHeader() string {
	raw := c.cfg.Email + ":" + c.cfg.APIToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// baseURL is the site root plus the v2 API base path.
func (c *Client) baseURL() string {
	return "https://" + c.cfg.Site + APIBasePath
}

// buildURL resolves a path or URL against the configured site. It handles three
// shapes: an absolute https URL (used verbatim), a site-relative path already
// containing the API base (e.g. the "/wiki/api/v2/pages?cursor=..." returned in
// _links.next), and an API-relative path (e.g. "/pages").
func (c *Client) buildURL(pathOrURL string) string {
	if strings.HasPrefix(pathOrURL, "https://") {
		return pathOrURL
	}
	if strings.HasPrefix(pathOrURL, "/wiki/") {
		return "https://" + c.cfg.Site + pathOrURL
	}
	if strings.HasPrefix(pathOrURL, "/") {
		return c.baseURL() + pathOrURL
	}
	return c.baseURL() + "/" + pathOrURL
}

func excerpt(b []byte) string {
	s := string(b)
	if len(s) > excerptLimit {
		return s[:excerptLimit] + "..."
	}
	return s
}

// Request performs an API call and decodes the JSON response into out (which
// may be nil to discard the body). It returns an *HTTPError for non-2xx
// responses.
func (c *Client) Request(ctx context.Context, pathOrURL string, opts RequestOptions, out any) error {
	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}
	fullURL := c.buildURL(pathOrURL)

	var bodyReader io.Reader
	if opts.Body != nil {
		bodyBytes, err := json.Marshal(opts.Body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", c.authHeader())
	if opts.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", req.URL.String(), err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{
			Method:     req.Method,
			URL:        req.URL.String(),
			Status:     resp.StatusCode,
			StatusText: http.StatusText(resp.StatusCode),
			Excerpt:    excerpt(payload),
		}
	}

	if out == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// page is the standard Confluence v2 paginated envelope.
type page struct {
	Results []json.RawMessage `json:"results"`
	Links   struct {
		Next string `json:"next"`
	} `json:"_links"`
}

// Paginate follows `_links.next` links, accumulating raw results until limit is
// reached or maxPages is exhausted. Each element is a raw JSON object the caller
// decodes or passes through. The `next` link is a site-relative path (e.g.
// "/wiki/api/v2/pages?cursor=..."), which buildURL resolves against the site.
func (c *Client) Paginate(ctx context.Context, pathOrURL string, limit, maxPages int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if maxPages <= 0 {
		maxPages = DefaultMaxPages
	}

	var values []json.RawMessage
	next := pathOrURL
	pages := 0

	for next != "" && len(values) < limit && pages < maxPages {
		var p page
		if err := c.Request(ctx, next, RequestOptions{}, &p); err != nil {
			return nil, err
		}
		values = append(values, p.Results...)
		next = p.Links.Next
		pages++
	}

	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}
