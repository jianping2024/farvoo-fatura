package cashdrawer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CloudJob is one pending-cash-drawer-opens row.
type CloudJob struct {
	ID                  string `json:"id"`
	RestaurantID        string `json:"restaurant_id"`
	SessionID           string `json:"session_id"`
	CollectedPaymentID  string `json:"collected_payment_id"`
	Status              string `json:"status"`
	CreatedAt           string `json:"created_at"`
	ExpiresAt           string `json:"expires_at"`
}

// Puller fetches pending cash-drawer opens and acks — ONLY cloud client for drawer hang-queue.
type Puller struct {
	APIBase string
	JWT     string
	Client  *http.Client
	// Kick is the ONLY kick executor (api.KickCashDrawerOnLocalDefault via Runtime.OpenCashDrawer).
	Kick func() error
}

func (p *Puller) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return http.DefaultClient
}

// PullAndKick is the ONLY compensation/doorbell entry: GET pending → kick → ack.
func (p *Puller) PullAndKick(ctx context.Context) (processed int, err error) {
	if p == nil || p.Kick == nil {
		return 0, fmt.Errorf("cashdrawer: puller not configured")
	}
	jobs, err := p.fetchPending(ctx)
	if err != nil {
		return 0, err
	}
	for _, job := range jobs {
		kickErr := p.Kick()
		if kickErr != nil {
			_ = p.ack(ctx, job.ID, "failed", "drawer_open_failed", kickErr.Error())
			continue
		}
		if ackErr := p.ack(ctx, job.ID, "succeeded", "", ""); ackErr != nil {
			return processed, ackErr
		}
		processed++
	}
	return processed, nil
}

func (p *Puller) fetchPending(ctx context.Context) ([]CloudJob, error) {
	url := strings.TrimRight(p.APIBase, "/") + "/api/print-agent/pending-cash-drawer-opens"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.JWT))
	res, err := p.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pending-cash-drawer-opens %s: %s", res.Status, string(raw))
	}
	var body struct {
		Jobs []CloudJob `json:"jobs"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	return body.Jobs, nil
}

func (p *Puller) ack(ctx context.Context, id, status, errCode, errMsg string) error {
	url := strings.TrimRight(p.APIBase, "/") + "/api/print-agent/cash-drawer-opens/" + id + "/ack"
	payload := map[string]any{"status": status}
	if status == "failed" {
		if errCode != "" {
			payload["error_code"] = errCode
		}
		if errMsg != "" {
			payload["error_message"] = errMsg
		}
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.JWT))
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(res.Body)
		return fmt.Errorf("cash-drawer ack %s: %s", res.Status, string(raw))
	}
	return nil
}
