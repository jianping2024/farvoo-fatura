package fiscalsigning

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StorePolicy is Ops fiscal store policy — ONLY cloud policy payload.
// MaxFiscalTerminals may still appear in JSON but Agent ignores it (local admin owns max).
type StorePolicy struct {
	FiscalProfile      string `json:"fiscal_profile"`
	MaxFiscalTerminals int    `json:"max_fiscal_terminals"`
	TerminalsUsed      int    `json:"terminals_used"`
}

// PullStorePolicy GETs Ops store policy — ONLY store-policy HTTP path.
func (c *Client) PullStorePolicy(ctx context.Context) (*StorePolicy, error) {
	if c == nil || strings.TrimSpace(c.APIBase) == "" || strings.TrimSpace(c.JWT) == "" {
		return nil, fmt.Errorf("fiscalsigning: client not configured")
	}
	url := strings.TrimRight(c.APIBase, "/") + "/api/print-agent/fiscal-store-policy"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.JWT)
	res, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("fiscalsigning: store-policy %s: %s", res.Status, string(raw))
	}
	var out StorePolicy
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
