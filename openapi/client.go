package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a thin HTTP client that signs every request with CODO1-HMAC-SHA256.
type Client struct {
	accessKey  string
	secretKey  string
	endpoint   string // scheme://host[:port], no trailing slash
	httpClient *http.Client
	timeout    time.Duration
}

// ClientOption configures NewClient.
type ClientOption func(*Client)

// WithHTTPClient sets a custom *http.Client (timeouts, transport, etc.).
func WithHTTPClient(c *http.Client) ClientOption {
	return func(cl *Client) {
		if c != nil {
			cl.httpClient = c
		}
	}
}

// WithTimeout sets the default client timeout when no custom HTTP client is provided.
func WithTimeout(d time.Duration) ClientOption {
	return func(cl *Client) {
		cl.timeout = d
	}
}

// NewClient creates an OpenAPI client.
// endpoint is the gateway base URL, e.g. "https://gw.example.com".
func NewClient(accessKey, secretKey, endpoint string, opts ...ClientOption) *Client {
	cl := &Client{
		accessKey: accessKey,
		secretKey: secretKey,
		endpoint:  strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		timeout:   10 * time.Second,
	}
	for _, o := range opts {
		o(cl)
	}
	if cl.httpClient == nil {
		cl.httpClient = &http.Client{Timeout: cl.timeout}
	}
	return cl
}

// DoOptions controls a single request.
type DoOptions struct {
	// Query is merged into the URL query string and included in the signature.
	Query url.Values
	// Body is the raw request body; used as-is for signing and sending.
	Body []byte
	// ContentType is set when non-empty; signed only if Body is non-empty.
	ContentType string
	// Headers are extra headers (not signed unless you also put them into the
	// signed set via a custom flow — reserved for caller metadata).
	Headers http.Header
	// Auth overrides timestamp/nonce for tests.
	Auth *AuthHeaderOptions
}

// Do signs and sends an HTTP request.
// pathOrURL may be an absolute path ("/api/p/...") or a full URL.
func (c *Client) Do(ctx context.Context, method, pathOrURL string, opt *DoOptions) (*http.Response, error) {
	if c == nil {
		return nil, fmt.Errorf("openapi: nil client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if opt == nil {
		opt = &DoOptions{}
	}

	path, query, base, err := c.resolveURL(pathOrURL, opt.Query)
	if err != nil {
		return nil, err
	}

	body := opt.Body
	contentType := opt.ContentType

	authOpt := opt.Auth
	if authOpt == nil {
		authOpt = &AuthHeaderOptions{ContentType: contentType}
	} else if contentType != "" && authOpt.ContentType == "" {
		cp := *authOpt
		cp.ContentType = contentType
		authOpt = &cp
	}

	auth, err := BuildAuthHeaders(c.accessKey, c.secretKey, method, path, query, body, authOpt)
	if err != nil {
		return nil, err
	}

	fullURL := base + path
	if len(query) > 0 {
		fullURL = fullURL + "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range auth {
		req.Header.Set(k, v)
	}
	if opt.Headers != nil {
		for k, vs := range opt.Headers {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	return c.httpClient.Do(req)
}

// Get issues a signed GET.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, path, &DoOptions{Query: query})
}

// Delete issues a signed DELETE.
func (c *Client) Delete(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	return c.Do(ctx, http.MethodDelete, path, &DoOptions{Query: query})
}

// PostJSON marshals v as compact JSON and issues a signed POST.
// Encoding matches Python separators=(",", ":") with UTF-8 (no HTML escape).
func (c *Client) PostJSON(ctx context.Context, path string, v any, query url.Values) (*http.Response, error) {
	body, err := marshalJSONCompact(v)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, http.MethodPost, path, &DoOptions{
		Query:       query,
		Body:        body,
		ContentType: "application/json",
	})
}

// PutJSON marshals v as compact JSON and issues a signed PUT.
func (c *Client) PutJSON(ctx context.Context, path string, v any, query url.Values) (*http.Response, error) {
	body, err := marshalJSONCompact(v)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, http.MethodPut, path, &DoOptions{
		Query:       query,
		Body:        body,
		ContentType: "application/json",
	})
}

// Request is a convenience alias: Do with explicit body bytes.
func (c *Client) Request(ctx context.Context, method, path string, query url.Values, body []byte, contentType string) (*http.Response, error) {
	return c.Do(ctx, method, path, &DoOptions{
		Query:       query,
		Body:        body,
		ContentType: contentType,
	})
}

func (c *Client) resolveURL(pathOrURL string, extra url.Values) (path string, query url.Values, base string, err error) {
	query = url.Values{}
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		u, e := url.Parse(pathOrURL)
		if e != nil {
			return "", nil, "", fmt.Errorf("openapi: bad url: %w", e)
		}
		path = u.Path
		if path == "" {
			path = "/"
		}
		query = u.Query()
		base = u.Scheme + "://" + u.Host
	} else {
		path = pathOrURL
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		base = c.endpoint
		if base == "" {
			return "", nil, "", fmt.Errorf("openapi: endpoint is empty and path is not absolute URL")
		}
	}
	if extra != nil {
		for k, vs := range extra {
			for _, v := range vs {
				query.Add(k, v)
			}
		}
	}
	return path, query, base, nil
}

func marshalJSONCompact(v any) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	// encoding/json produces compact output by default (no spaces).
	// SetEscapeHTML(false) to avoid \u003c etc., closer to Python ensure_ascii=False path for ASCII JSON.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode appends a trailing newline — strip it so body matches Python dumps.
	b := bytes.TrimSuffix(buf.Bytes(), []byte{'\n'})
	return b, nil
}
