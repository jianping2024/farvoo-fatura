package billsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

// Puller fetches pending bill syncs and acks — ONLY cloud HTTP client for bill sync.
type Puller struct {
	APIBase string
	JWT     string
	DB      *store.DB
	Client  *http.Client
}

func (p *Puller) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return http.DefaultClient
}

// PullAndIngest is the ONLY compensation/doorbell entry: GET pending → IngestCloudJob → ack.
func (p *Puller) PullAndIngest(ctx context.Context) (processed int, err error) {
	if p == nil || p.DB == nil {
		return 0, fmt.Errorf("billsync: puller not configured")
	}
	jobs, err := p.fetchPending(ctx)
	if err != nil {
		return 0, err
	}
	for _, job := range jobs {
		if err := p.ingestAndAck(ctx, job); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (p *Puller) fetchPending(ctx context.Context) ([]CloudJob, error) {
	url := strings.TrimRight(p.APIBase, "/") + "/api/print-agent/pending-bill-syncs"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.JWT)
	res, err := p.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("pending-bill-syncs %s: %s", res.Status, string(raw))
	}
	var out struct {
		Jobs []CloudJob `json:"jobs"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Jobs, nil
}

func (p *Puller) ingestAndAck(ctx context.Context, job CloudJob) error {
	_, err := IngestCloudJob(p.DB, job)
	if err != nil {
		code, msg := "persist_failed", err.Error()
		if ie := AsIngestError(err); ie != nil {
			code, msg = ie.Code, ie.Message
		}
		return p.ack(ctx, job.ID, "failed", code, msg)
	}
	return p.ack(ctx, job.ID, "succeeded", "", "")
}

func (p *Puller) ack(ctx context.Context, jobID, status, errCode, errMsg string) error {
	body := map[string]any{"status": status}
	if errCode != "" {
		body["error_code"] = errCode
	}
	if errMsg != "" {
		body["error_message"] = errMsg
	}
	raw, _ := json.Marshal(body)
	url := strings.TrimRight(p.APIBase, "/") + "/api/print-agent/bill-syncs/" + jobID + "/ack"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.JWT)
	res, err := p.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("bill-sync ack %s: %s", res.Status, string(b))
	}
	return nil
}
