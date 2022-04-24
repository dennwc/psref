// Package psref implements an (unofficial) Lenovo PSREF API client.
package psref

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

var (
	// ErrNotFound is returned when lookup leads to no results.
	ErrNotFound = errors.New("not found")
)

const (
	apiVersion             = "2"
	apiDefaultRetries      = 3
	apiDefaultRateInterval = time.Second / 3
	apiDefaultRateBurst    = 10
	// apiDefaultURL is a default base URL.
	//
	// It was set to http://psrefapi.lenovo.com:8081 previously, but this name leads to a different host now.
	apiDefaultURL = "http://104.232.254.26:8081"
)

// ClientOption controls different aspects of Client behavior.
type ClientOption interface {
	apply(c *Client)
}

type clientOptionFunc func(c *Client)

func (fnc clientOptionFunc) apply(c *Client) { fnc(c) }

// WithHTTPClient sets an HTTP client for all requests.
func WithHTTPClient(cli *http.Client) ClientOption {
	if cli == nil {
		cli = http.DefaultClient
	}
	return clientOptionFunc(func(c *Client) {
		c.cli = cli
	})
}

// WithBaseURL changes the base URL for all API requests.
func WithBaseURL(url string) ClientOption {
	if url == "" {
		url = apiDefaultURL
	}
	return clientOptionFunc(func(c *Client) {
		c.baseURL = strings.TrimRight(url, "/")
	})
}

// WithDebug sets a debug log output.
func WithDebug(w io.Writer) ClientOption {
	return clientOptionFunc(func(c *Client) {
		c.debug = w
	})
}

// WithRetry sets the number of retries per request.
// Setting 0 or 1 means send request only once, setting -1 means retry until completion.
func WithRetry(retries int) ClientOption {
	return clientOptionFunc(func(c *Client) {
		c.retries = retries
	})
}

// WithRate sets a rate limit for all requests. Passing nil will disable rate limiting.
func WithRate(rate *rate.Limiter) ClientOption {
	return clientOptionFunc(func(c *Client) {
		c.rate = rate
	})
}

// NewClient creates a client with specified options.
//
// By default, the client will retry requests a few times and will use a conservative rate limit.
// See WithRetry and WithRate to adjust these settings.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		cli:     http.DefaultClient,
		baseURL: apiDefaultURL,
		retries: apiDefaultRetries,
		rate:    rate.NewLimiter(rate.Every(apiDefaultRateInterval), apiDefaultRateBurst),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.apply(c)
	}
	return c
}

// Client for PSREF API.
type Client struct {
	cli     *http.Client
	baseURL string
	rate    *rate.Limiter
	retries int
	debug   io.Writer
}

// get sends an HTTP GET request with given parameters. It will decode JSON response to out.
//
// This method will retry failed requests automatically, if client allows it. See WithRetry.
func (c *Client) get(ctx context.Context, path string, vars url.Values, out interface{}) error {
	if c.retries == 0 || c.retries == 1 {
		return c.getOnce(ctx, path, vars, out)
	}
	var last error
	for try := 0; c.retries < 0 || try < c.retries; try++ {
		err := c.getOnce(ctx, path, vars, out)
		if err == nil || err == ErrNotFound {
			return err
		}
		last = err
	}
	return last
}

// getOnce sends an HTTP GET request with given parameters. It will decode JSON response to out.
//
// This method will not retry requests. Use get instead.
func (c *Client) getOnce(ctx context.Context, path string, vars url.Values, out interface{}) error {
	if c.rate != nil {
		if err := c.rate.Wait(ctx); err != nil {
			return err
		}
	}
	if vars == nil {
		vars = make(url.Values)
	}
	vars.Set("api_v", apiVersion)
	u := strings.Join([]string{c.baseURL, path, "?", vars.Encode()}, "")
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	resp, err := c.cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: status %v", path, resp.Status)
	}
	var r io.Reader = resp.Body
	if c.debug != nil {
		var buf bytes.Buffer
		r = io.TeeReader(r, &buf)
		defer func() {
			out := &buf
			var ident bytes.Buffer
			if err := json.Indent(&ident, buf.Bytes(), "", "\t"); err == nil {
				out = &ident
			}
			fmt.Fprintf(c.debug, "GET %s\n%s\n", u, out.String())
		}()
	}
	return json.NewDecoder(r).Decode(out)
}

// Products lists all available and active products. See WithdrawnProducts for discontinued ones.
func (c *Client) Products(ctx context.Context) ([]ProductType, error) {
	var resp []ProductType
	err := c.get(ctx, "/", nil, &resp)
	for i := range resp {
		resp[i].normalize()
	}
	return resp, err
}

// WithdrawnProducts is similar to Products, but returns discontinued products instead of active ones.
func (c *Client) WithdrawnProducts(ctx context.Context) ([]ProductType, error) {
	var resp []productType
	err := c.get(ctx, "/psref/mobile/withdrawproducts", nil, &resp)
	out := make([]ProductType, 0, len(resp))
	for _, v := range resp {
		p := ProductType{
			Name: v.Name, Lineup: v.Lineup,
		}
		p.normalize()
		out = append(out, p)
	}
	return out, err
}

// Updates returns an information about the current version of PSREF data and a list of added/updated/deleted entries.
func (c *Client) Updates(ctx context.Context) (*Updates, error) {
	var resp *Updates
	err := c.get(ctx, "/psref/mobile/new", nil, &resp)
	resp.parse()
	return resp, err
}

type getModelOpts struct {
	Clsf string
	Sc   string // search cond?
	Qt   string
	Kw   string
	Page int
}

func unescapeImage(s string) string {
	if !strings.HasPrefix(s, "http%") {
		return s
	}
	v, err := url.PathUnescape(s)
	if err != nil {
		return s
	}
	return v
}

func (c *Client) getModel(ctx context.Context, pid PID, opts getModelOpts) (*Product, error) {
	vars := make(url.Values)
	if opts.Clsf != "" {
		vars.Set("clsf", opts.Clsf)
	}
	if opts.Sc != "" {
		vars.Set("sc", opts.Sc)
	}
	if opts.Qt != "" {
		vars.Set("qt", opts.Qt)
	}
	if opts.Page != 0 {
		vars.Set("pagenumber", strconv.Itoa(opts.Page))
	}
	if opts.Kw != "" {
		vars.Set("kw", opts.Kw)
	}
	var resp *Product
	err := c.get(ctx, "/psref/mobile/product/"+strconv.FormatUint(uint64(pid), 10), vars, &resp)
	if resp != nil {
		resp.Image = unescapeImage(resp.Image)
		resp.normalize()
	}
	return resp, err
}

// ProductByID returns an information about the product, given its numeric PSREF ID.
// Description of the product includes a list of all available product models.
func (c *Client) ProductByID(ctx context.Context, id PID) (*Product, error) {
	return c.getModel(ctx, id, getModelOpts{})
}

func (c *Client) productByModelCode(ctx context.Context, code ModelCode) (PID, int, error) {
	res, err := c.Search(ctx, string(code))
	if err != nil {
		return 0, 0, err
	} else if len(res) == 0 {
		return 0, 0, ErrNotFound
	}
	id := res[0].ID
	cnt := res[0].Models
	for _, p := range res[1:] {
		if id != p.ID {
			return 0, 0, errors.New("more than one product matched")
		}
		cnt += p.Models
	}
	return id, cnt, nil
}

// ProductByModelCode returns an information about the product, given its alphanumeric code of one of the models.
//
// This method uses the search API, which might be considerably slower. Use ProductByID instead.
func (c *Client) ProductByModelCode(ctx context.Context, code ModelCode) (*Product, error) {
	pid, _, err := c.productByModelCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return c.ProductByID(ctx, pid)
}

// Books returns a list of resources for users to read.
func (c *Client) Books(ctx context.Context) ([]Book, error) {
	var resp []Book
	err := c.get(ctx, "/psref/mobile/book", nil, &resp)
	return resp, err
}

// ModelByID returns information about the given product model.
func (c *Client) ModelByID(ctx context.Context, id PID, code ModelCode) (*Model, error) {
	var resp *Model
	u := strings.Join([]string{"/psref/mobile/Model", strconv.FormatUint(uint64(id), 10), string(code)}, "/")
	err := c.get(ctx, u, nil, &resp)
	if resp != nil {
		resp.Code = code
		resp.normalize()
	}
	return resp, err
}

// ModelByCode returns information about the given product model.
//
// This method uses the search API, which might be considerably slower. Use ModelByID instead.
func (c *Client) ModelByCode(ctx context.Context, code ModelCode) (*Model, error) {
	pid, models, err := c.productByModelCode(ctx, code)
	if err != nil {
		return nil, err
	} else if models > 1 {
		return nil, errors.New("more than one model matched")
	}
	return c.ModelByID(ctx, pid, code)
}

// SearchResult as returned by the Search API.
type SearchResult struct {
	ID     PID    `json:"ProductId"`
	Name   string `json:"ProductName"`
	Models int    `json:"ModelCount"`
}

// Search PSREF data using keywords.
func (c *Client) Search(ctx context.Context, qu string) ([]SearchResult, error) {
	var resp struct {
		Results []SearchResult `json:"result"`
	}
	vars := make(url.Values)
	vars.Set("kw", qu)
	err := c.get(ctx, "/psref/mobile/searchv3", vars, &resp)
	return resp.Results, err
}
