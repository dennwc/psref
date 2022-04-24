package psref

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var (
	ErrNotFound = errors.New("not found")
)

const (
	debug      = false
	apiVersion = "2"
)

func NewClient(cli *http.Client) *Client {
	// was http://psrefapi.lenovo.com:8081
	return NewClientWithURL(cli, "http://104.232.254.26:8081")
}

func NewClientWithURL(cli *http.Client, baseURL string) *Client {
	if cli == nil {
		cli = http.DefaultClient
	}
	return &Client{cli: cli, baseURL: baseURL}
}

type Client struct {
	cli     *http.Client
	baseURL string
}

func (c *Client) get(ctx context.Context, path string, vars url.Values, out interface{}) error {
	if vars == nil {
		vars = make(url.Values)
	}
	vars.Set("api_v", apiVersion)
	u := strings.Join([]string{c.baseURL, path, "?", vars.Encode()}, "")
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
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
	if debug {
		var buf bytes.Buffer
		r = io.TeeReader(r, &buf)
		defer func() {
			out := &buf
			var ident bytes.Buffer
			if err := json.Indent(&ident, buf.Bytes(), "", "\t"); err == nil {
				out = &ident
			}
			log.Printf("GET %s\n%s", u, out.String())
		}()
	}
	return json.NewDecoder(r).Decode(out)
}

func (c *Client) Products(ctx context.Context) ([]ProductType, error) {
	var resp []ProductType
	err := c.get(ctx, "/", nil, &resp)
	for i := range resp {
		resp[i].normalize()
	}
	return resp, err
}

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

func (c *Client) ProductByModelCode(ctx context.Context, code ModelCode) (*Product, error) {
	pid, _, err := c.productByModelCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return c.ProductByID(ctx, pid)
}

func (c *Client) Books(ctx context.Context) ([]Book, error) {
	var resp []Book
	err := c.get(ctx, "/psref/mobile/book", nil, &resp)
	return resp, err
}

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

func (c *Client) ModelByCode(ctx context.Context, code ModelCode) (*Model, error) {
	pid, models, err := c.productByModelCode(ctx, code)
	if err != nil {
		return nil, err
	} else if models > 1 {
		return nil, errors.New("more than one model matched")
	}
	return c.ModelByID(ctx, pid, code)
}

func (c *Client) Search(ctx context.Context, qu string) ([]SearchResult, error) {
	var resp struct {
		Results []SearchResult `json:"result"`
	}
	vars := make(url.Values)
	vars.Set("kw", qu)
	err := c.get(ctx, "/psref/mobile/searchv3", vars, &resp)
	return resp.Results, err
}
