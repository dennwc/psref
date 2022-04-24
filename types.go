package psref

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PID is a numeric PSREF product ID.
type PID uint64

// ModelCode is an alphanumeric product code.
type ModelCode string

var (
	_ json.Marshaler   = Date{}
	_ json.Unmarshaler = (*Date)(nil)
)

func normalizeURL(s string) string {
	return strings.ReplaceAll(s, "\\", "/")
}

// Date is wrapper around time.Time which uses custom JSON encoding.
type Date time.Time

func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	s := time.Time(d).Format("2006-01-02")
	return json.Marshal(s)
}

// ProductType is a top-level product type which includes multiple product lineups.
type ProductType struct {
	Name    string        `json:"ClassificationName"`
	BgColor string        `json:"BackgroundColor"`
	Lineup  []ProductLine `json:"ProductLine"`
}

type productType struct {
	Name   string        `json:"ProductType"`
	Lineup []ProductLine `json:"ProductLine"`
}

func (p *ProductType) normalize() {
	for i := range p.Lineup {
		p.Lineup[i].normalize()
	}
}

// ProductLine includes multiple product series.
type ProductLine struct {
	Name   string   `json:"ProductLineName"`
	Image  string   `json:"ImageUrl"`
	Series []Series `json:"Series"`
}

func (p *ProductLine) normalize() {
	p.Image = normalizeURL(p.Image)
	for i := range p.Series {
		p.Series[i].normalize()
	}
}

type Series struct {
	Name     string         `json:"SeriesName"`
	Products []ProductShort `json:"Products"`
}

func (p *Series) normalize() {
	for i := range p.Products {
		p.Products[i].normalize()
	}
}

// ProductShort is a short product description.
type ProductShort struct {
	ID              PID    `json:"ProductId"`
	Key             string `json:"ProductKey"`
	Name            string `json:"ProductName"`
	WithdrawnStatus int64  `json:"P_WdStatus"`
	Updated         Date   `json:"LastUpdated"`
	ModelModified   Date   `json:"ModelModifyDateTime"`
	ConfigModified  Date   `json:"ConfigModifyDateTime"`
}

func (p *ProductShort) normalize() {}

// ModelInfo is a basic model info used in the model list.
type ModelInfo struct {
	Code    ModelCode `json:"ModelCode"`
	Summary string    `json:"Summary"`
	Updated Date      `json:"Updated"`
}

// Documentation is a reference to documentation resource.
type Documentation struct {
	ProductID PID    `json:"ProductId"`
	Title     string `json:"DocTitle"`
	URL       string `json:"DocLink"`
}

// Product is a full product information. It includes multiple models, which in turn list exact specifications.
type Product struct {
	ID              PID             `json:"ProductId"`
	Key             string          `json:"ProductKey"`
	Name            string          `json:"Name"`
	RefURL          string          `json:"ProductURL"`
	WithdrawnStatus int64           `json:"P_WdStatus"`
	SpecURL         string          `json:"Spec"`
	US_Pdf          string          `json:"US_Pdf"`
	EMEA_Pdf        string          `json:"EMEA_Pdf"`
	WW_Pdf          string          `json:"WW_Pdf"`
	Image           string          `json:"ImageForShare"`
	Images          []string        `json:"Images"`
	Models          []ModelInfo     `json:"Models"`
	Docs            []Documentation `json:"Documentations"`
}

func (p *Product) normalize() {
	p.RefURL = normalizeURL(p.RefURL)
	p.SpecURL = normalizeURL(p.SpecURL)
	p.US_Pdf = normalizeURL(p.US_Pdf)
	p.EMEA_Pdf = normalizeURL(p.EMEA_Pdf)
	p.WW_Pdf = normalizeURL(p.WW_Pdf)
	p.Image = normalizeURL(p.Image)
	for i := range p.Images {
		p.Images[i] = normalizeURL(p.Images[i])
	}
}

// UpdatedProduct is an information about product update used in the PSREF Updates info.
type UpdatedProduct struct {
	ID     PID    `json:"productId"`
	Title  string `json:"title"`
	Reason string `json:"reason,omitempty"`
}

var (
	reVersion   = regexp.MustCompile(`Version (\d+)`)
	reVersionTS = regexp.MustCompile(` (\w{3}\.\d{1,2}, \d{4})`)
)

type Updates struct {
	Version      uint64    `json:"x_Version"`   // TODO: upstream
	VersionTS    time.Time `json:"x_VersionTS"` // TODO: upstream
	VersionTitle string    `json:"LatestUpdateVersion"`

	New       []UpdatedProduct `json:"New"`
	Updated   []UpdatedProduct `json:"Updated"`
	Withdrawn []UpdatedProduct `json:"Withdrawn"`
}

func (upd *Updates) parse() {
	if upd == nil {
		return
	}
	upd.VersionTitle = strings.TrimPrefix(upd.VersionTitle, "<b>")
	upd.VersionTitle = strings.TrimSuffix(upd.VersionTitle, "</b>")
	upd.VersionTitle = strings.TrimSpace(upd.VersionTitle)
	if sub := reVersion.FindStringSubmatch(upd.VersionTitle); len(sub) > 1 {
		if vers, err := strconv.ParseUint(sub[1], 10, 64); err == nil {
			upd.Version = vers
		}
	}
	if sub := reVersionTS.FindStringSubmatch(upd.VersionTitle); len(sub) > 1 {
		if ts, err := time.Parse("Jan.2, 2006", sub[1]); err == nil {
			upd.VersionTS = ts
		}
	}
	for i := range upd.Updated {
		u := &upd.Updated[i]
		if !strings.HasSuffix(u.Title, ")") {
			continue
		}
		title := u.Title[:len(u.Title)-1]
		if i := strings.LastIndexByte(title, '('); i > 0 {
			reason := title[i+1:]
			switch reason {
			case "new model added",
				"spec updated":
				u.Title = strings.TrimSpace(u.Title[:i])
				u.Reason = reason
			}
		}
	}
}

// Book is a reference to a resource for users to read.
type Book struct {
	Title  string `json:"BookTitle"`
	URL    string `json:"BookLink"`
	Geo    string `json:"Geo"`
	Remark string `json:"Remark"`
}

// KeyValue pair used in the model specifications.
type KeyValue struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

// Model is a full model information, including exact specifications. Not all Product fields will be set.
type Model struct {
	Product
	WithdrawnStatus int        `json:"M_WdStatus"`
	RefURL          string     `json:"ModelURL"`
	Detail          []KeyValue `json:"Detail"`
	Code            ModelCode  `json:"ModelCode"`
}

// DetailByName searches a specification value by the key name.
func (m *Model) DetailByName(name string) string {
	for _, v := range m.Detail {
		if v.Name == name {
			return v.Value
		}
	}
	return ""
}
