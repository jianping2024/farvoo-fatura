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

// Client talks to Farvoo fiscal-signing APIs — ONLY Agent cloud client for product-key provision.
type Client struct {
	APIBase string
	JWT     string
	Client  *http.Client
}

func (c *Client) http() *http.Client {
	if c != nil && c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

// RegisterDevicePublicKey POSTs B' — ONLY register HTTP path.
func (c *Client) RegisterDevicePublicKey(ctx context.Context, devicePublicKeyPEM string) (installationID string, status string, err error) {
	if c == nil || strings.TrimSpace(c.APIBase) == "" || strings.TrimSpace(c.JWT) == "" {
		return "", "", fmt.Errorf("fiscalsigning: client not configured")
	}
	body, _ := json.Marshal(map[string]string{"device_public_key": devicePublicKeyPEM})
	url := strings.TrimRight(c.APIBase, "/") + "/api/print-agent/fiscal-signing/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.JWT)
	res, err := c.http().Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", "", fmt.Errorf("fiscalsigning: register %s: %s", res.Status, string(raw))
	}
	var out struct {
		InstallationID string `json:"installation_id"`
		Status         string `json:"status"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", "", err
	}
	return out.InstallationID, out.Status, nil
}

// ProvisionBundle is the cloud-wrapped product key payload.
type ProvisionBundle struct {
	InstallationID     string `json:"installation_id"`
	SigningKeyVersion  int    `json:"signing_key_version"`
	ProductPublicKeyPEM string `json:"product_public_key_pem"`
	WrappedPrivateKey  string `json:"wrapped_private_key"`
}

// PullProvision GETs active C — ONLY provision HTTP path.
func (c *Client) PullProvision(ctx context.Context) (*ProvisionBundle, error) {
	if c == nil || strings.TrimSpace(c.APIBase) == "" || strings.TrimSpace(c.JWT) == "" {
		return nil, fmt.Errorf("fiscalsigning: client not configured")
	}
	url := strings.TrimRight(c.APIBase, "/") + "/api/print-agent/fiscal-signing/provision"
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
	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotActive
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("fiscalsigning: provision %s: %s", res.Status, string(raw))
	}
	var out ProvisionBundle
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.WrappedPrivateKey == "" || out.ProductPublicKeyPEM == "" || out.InstallationID == "" {
		return nil, fmt.Errorf("fiscalsigning: incomplete provision payload")
	}
	if out.SigningKeyVersion <= 0 {
		out.SigningKeyVersion = 1
	}
	return &out, nil
}

// ErrNotActive means Ops has not activated this device yet.
var ErrNotActive = fmt.Errorf("fiscalsigning: not_active")
