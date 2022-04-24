package psref

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testLogData(t testing.TB, v interface{}) {
	if debug {
		data, _ := json.MarshalIndent(v, "", "\t")
		t.Logf("json:\n%s", string(data))
	}
}

func TestProductTypes(t *testing.T) {
	c := NewClient(nil)
	types, err := c.Products(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, types)
	testLogData(t, types)
}

func TestWithdrawnProductTypes(t *testing.T) {
	c := NewClient(nil)
	types, err := c.WithdrawnProducts(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, types)
	testLogData(t, types)
}

func TestUpdates(t *testing.T) {
	c := NewClient(nil)
	resp, err := c.Updates(context.Background())
	require.NoError(t, err)
	testLogData(t, resp)
	t.Logf("%q", resp.VersionTitle)
	t.Logf("Version = %v", resp.Version)
	if !resp.VersionTS.IsZero() {
		t.Logf("TS = %v", time.Since(resp.VersionTS).Round(time.Hour*24))
	}
	require.GreaterOrEqual(t, resp.Version, uint64(593))
	require.Greater(t, len(resp.New)+len(resp.Updated)+len(resp.Withdrawn), 0)
	require.NotZero(t, resp.VersionTS)
}

func TestProductByID(t *testing.T) {
	c := NewClient(nil)
	p, err := c.ProductByID(context.Background(), 1234)
	require.NoError(t, err)
	require.Equal(t, "Lenovo_Legion_5P_15IMH05H", p.Key)
	require.Greater(t, len(p.Models), 100)
	testLogData(t, p)
}

func TestModelByID(t *testing.T) {
	c := NewClient(nil)
	p, err := c.ModelByID(context.Background(), 1234, "82AW006JRK")
	require.NoError(t, err)
	require.Equal(t, "Lenovo_Legion_5P_15IMH05H", p.Key)
	require.Greater(t, len(p.Detail), 30)
	testLogData(t, p)
}

func TestModelByCode(t *testing.T) {
	c := NewClient(nil)
	p, err := c.ModelByCode(context.Background(), "82AK0002US")
	require.NoError(t, err)
	require.Equal(t, "Lenovo_Flex_5G_14Q8CX05", p.Key)
	require.Greater(t, len(p.Detail), 30)
	testLogData(t, p)
}

func TestProductByModelCode(t *testing.T) {
	c := NewClient(nil)
	p, err := c.ProductByModelCode(context.Background(), "82AK0002US")
	require.NoError(t, err)
	require.Equal(t, PID(1163), p.ID)
	require.Equal(t, "Lenovo_Flex_5G_14Q8CX05", p.Key)
	testLogData(t, p)
}
