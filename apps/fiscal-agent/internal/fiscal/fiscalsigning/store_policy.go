package fiscalsigning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StorePolicy is Ops fiscal store policy — ONLY cloud policy payload.
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

// TerminalPairResult is cloud terminal pair response.
type TerminalPairResult struct {
	TerminalID     string `json:"terminal_id"`
	OpsTerminalRef string `json:"ops_terminal_ref"`
	Label          string `json:"label"`
}

// PairFiscalTerminal POSTs pairing code — ONLY terminal pair HTTP path.
func (c *Client) PairFiscalTerminal(ctx context.Context, pairingCode, label string) (*TerminalPairResult, error) {
	if c == nil || strings.TrimSpace(c.APIBase) == "" || strings.TrimSpace(c.JWT) == "" {
		return nil, fmt.Errorf("fiscalsigning: client not configured")
	}
	body, _ := json.Marshal(map[string]string{"pairing_code": pairingCode, "label": label})
	url := strings.TrimRight(c.APIBase, "/") + "/api/print-agent/fiscal-terminals/pair"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.JWT)
	res, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("fiscalsigning: terminal-pair %s: %s", res.Status, string(raw))
	}
	var out TerminalPairResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CloudTerminal is one terminal row from Ops.
type CloudTerminal struct {
	ID             string `json:"id"`
	OpsTerminalRef string `json:"ops_terminal_ref"`
	Label          string `json:"label"`
	Active         bool   `json:"active"`
}

// ListFiscalTerminals GETs terminal list — ONLY terminal list HTTP path.
func (c *Client) ListFiscalTerminals(ctx context.Context) ([]CloudTerminal, error) {
	if c == nil || strings.TrimSpace(c.APIBase) == "" || strings.TrimSpace(c.JWT) == "" {
		return nil, fmt.Errorf("fiscalsigning: client not configured")
	}
	url := strings.TrimRight(c.APIBase, "/") + "/api/print-agent/fiscal-terminals"
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
		return nil, fmt.Errorf("fiscalsigning: terminals %s: %s", res.Status, string(raw))
	}
	var out struct {
		Terminals []CloudTerminal `json:"terminals"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Terminals, nil
}
